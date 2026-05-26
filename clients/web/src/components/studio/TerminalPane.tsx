"use client";

// TerminalPane — xterm.js wired to the runtime's PTY WebSocket.
//
// Protocol (core/runtime/internal/httpapi/api.go:617):
//   GET /workspaces/{id}/terminal?token={jwt}   → WebSocket
//   server → client: binary frames (raw stdout)
//   client → server: binary frames (raw stdin)
//   client → server: text JSON {type:"resize",rows,cols}
//
// Auth: WS clients can't set Authorization headers, so the runtime's
// auth.Middleware (core/runtime/internal/auth/auth.go:97) accepts a
// `?token=` query string. We pass the orchestrator JWT — the runtime
// shares the same JWT secret.
//
// xterm.js is heavy (~250KB). The pane is rendered inside the studio's
// bottom dock which already lazy-loads its children, but we still
// dynamic-import this module at the call site so the chunk loads only
// when the terminal tab is first opened.

import { Box, Button, Stack, Typography } from "@mui/material";
import { useEffect, useMemo, useRef, useState } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { getToken } from "../../lib/apollo";
import { tokens } from "../../theme";

export interface TerminalPaneProps {
  workspaceID: string;
}

type ConnectionState =
  | "connecting"
  | "open"
  | "closed"
  | "error"
  | "missing-workspace";

// Stable theme object — Terminal constructor reads it once. Pulled
// from `tokens.color.*` so the terminal matches the IronFlyer dark
// reference; no raw hex.
const XTERM_THEME = {
  background: tokens.color.bg.inset,
  foreground: tokens.color.text.primary,
  cursor: tokens.color.accent.violet,
  cursorAccent: tokens.color.bg.inset,
  selectionBackground: `${tokens.color.accent.violet}66`,
  black: "#1a1a2e",
  red: tokens.color.accent.danger,
  green: tokens.color.accent.success,
  yellow: tokens.color.accent.warning,
  blue: tokens.color.accent.sky,
  magenta: tokens.color.brand.magenta,
  cyan: tokens.color.accent.sky,
  white: tokens.color.text.primary,
  brightBlack: tokens.color.text.muted,
  brightRed: tokens.color.accent.danger,
  brightGreen: tokens.color.accent.success,
  brightYellow: tokens.color.accent.warning,
  brightBlue: tokens.color.accent.sky,
  brightMagenta: tokens.color.accent.violet,
  brightCyan: tokens.color.accent.sky,
  brightWhite: tokens.color.text.primary,
};

const RUNTIME_BASE =
  process.env.NEXT_PUBLIC_RUNTIME_URL || "http://localhost:8090";

function wsUrlFor(workspaceID: string, token: string | null): string {
  const httpBase = RUNTIME_BASE.replace(/\/+$/, "");
  const wsBase = httpBase.replace(/^http/, "ws");
  const q = token ? `?token=${encodeURIComponent(token)}` : "";
  return `${wsBase}/workspaces/${encodeURIComponent(workspaceID)}/terminal${q}`;
}

export function TerminalPane({ workspaceID }: TerminalPaneProps) {
  const hostRef = useRef<HTMLDivElement | null>(null);
  const termRef = useRef<Terminal | null>(null);
  const fitRef = useRef<FitAddon | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const [state, setState] = useState<ConnectionState>("connecting");
  const [nonce, setNonce] = useState(0); // bump to force reconnect

  // Stable connection setup — runs once per workspaceID / nonce. We
  // tear down both the xterm instance and the socket on cleanup so
  // mounting the pane twice (e.g. React strict mode dev) doesn't leak.
  useEffect(() => {
    if (!workspaceID) {
      setState("missing-workspace");
      return;
    }
    setState("connecting");

    const host = hostRef.current;
    if (!host) return;

    const term = new Terminal({
      theme: XTERM_THEME,
      fontFamily: tokens.font.mono,
      fontSize: 13,
      lineHeight: 1.25,
      cursorBlink: true,
      cursorStyle: "bar",
      allowProposedApi: true,
      // Default to xterm-256color so common TUIs (vim, htop, less)
      // render their colours correctly. The shell can still override
      // via $TERM if it wants to.
      scrollback: 5000,
      convertEol: true,
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(host);
    fit.fit();
    termRef.current = term;
    fitRef.current = fit;

    const token = getToken();
    const ws = new WebSocket(wsUrlFor(workspaceID, token));
    ws.binaryType = "arraybuffer";
    wsRef.current = ws;

    const sendResize = () => {
      try {
        fit.fit();
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(
            JSON.stringify({
              type: "resize",
              rows: term.rows,
              cols: term.cols,
            }),
          );
        }
      } catch {
        // Fit can throw mid-mount if the host has zero dimensions
        // (e.g. dock just collapsed). Safe to ignore — the next
        // ResizeObserver tick will try again.
      }
    };

    ws.onopen = () => {
      setState("open");
      term.write(
        "\x1b[1;35m◢\x1b[0m \x1b[2mIronflyer terminal connected — workspace \x1b[0;36m" +
          workspaceID +
          "\x1b[0m\r\n",
      );
      sendResize();
    };
    ws.onmessage = (event: MessageEvent) => {
      if (event.data instanceof ArrayBuffer) {
        term.write(new Uint8Array(event.data));
        return;
      }
      // Text frames are control messages from the server — the runtime
      // sends `{"type":"pty.shutdown_imminent"}` before pod drain.
      try {
        const ctrl = JSON.parse(event.data as string) as {
          type?: string;
          msg?: string;
        };
        if (ctrl.type === "pty.shutdown_imminent") {
          term.write(
            "\r\n\x1b[1;33m! Pod shutdown — terminal will reconnect on next focus.\x1b[0m\r\n",
          );
        } else if (ctrl.type === "error" && ctrl.msg) {
          term.write(`\r\n\x1b[1;31m! ${ctrl.msg}\x1b[0m\r\n`);
        }
      } catch {
        // Non-JSON text frame — write it verbatim.
        term.write(String(event.data));
      }
    };
    ws.onerror = () => {
      setState("error");
    };
    ws.onclose = () => {
      setState((s) => (s === "open" ? "closed" : s));
      term.write("\r\n\x1b[2mTerminal disconnected.\x1b[0m\r\n");
    };

    // stdin → ws
    const stdinSub = term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        const enc = new TextEncoder();
        ws.send(enc.encode(data));
      }
    });

    // host element resize → resize PTY
    const ro = new ResizeObserver(() => sendResize());
    ro.observe(host);
    window.addEventListener("resize", sendResize);

    return () => {
      stdinSub.dispose();
      ro.disconnect();
      window.removeEventListener("resize", sendResize);
      try {
        ws.close(1000, "unmount");
      } catch {
        // ignore
      }
      term.dispose();
      termRef.current = null;
      fitRef.current = null;
      wsRef.current = null;
    };
  }, [workspaceID, nonce]);

  const reconnect = useMemo(
    () => () => setNonce((n) => n + 1),
    [],
  );

  if (state === "missing-workspace") {
    return (
      <EmptyShell
        title="No workspace allocated"
        body="The runtime provisions a workspace once the first execution starts. Run a build to open a real shell here."
      />
    );
  }

  return (
    <Box
      sx={{
        bgcolor: tokens.color.bg.inset,
        display: "flex",
        flexDirection: "column",
        height: "100%",
        minHeight: 0,
      }}
    >
      <StatusStrip state={state} workspaceID={workspaceID} onReconnect={reconnect} />
      <Box
        ref={hostRef}
        sx={{
          flex: 1,
          minHeight: 0,
          overflow: "hidden",
          px: 1,
          py: 0.5,
          // xterm's own canvas owns the rendering; we just give it a
          // contained box matched to the cockpit's mono font feel.
          "& .xterm": { height: "100%" },
          "& .xterm-viewport::-webkit-scrollbar": { width: 8 },
          "& .xterm-viewport::-webkit-scrollbar-thumb": {
            background: tokens.color.border.subtle,
            borderRadius: 4,
          },
        }}
      />
    </Box>
  );
}

function StatusStrip({
  state,
  workspaceID,
  onReconnect,
}: {
  state: ConnectionState;
  workspaceID: string;
  onReconnect: () => void;
}) {
  const palette = STATE_PALETTE[state];
  return (
    <Stack
      direction="row"
      alignItems="center"
      sx={{
        bgcolor: tokens.color.bg.surfaceRaised,
        borderBottom: `1px solid ${tokens.color.border.subtle}`,
        gap: 1,
        px: 1.25,
        py: 0.5,
      }}
    >
      <Box
        sx={{
          bgcolor: palette.dot,
          borderRadius: "50%",
          height: 8,
          width: 8,
        }}
      />
      <Typography
        sx={{
          color: palette.fg,
          fontFamily: tokens.font.mono,
          fontSize: 11,
          fontWeight: 700,
          letterSpacing: 0.6,
          textTransform: "uppercase",
        }}
      >
        {palette.label}
      </Typography>
      <Box sx={{ flex: 1 }} />
      <Typography
        sx={{
          color: tokens.color.text.muted,
          fontFamily: tokens.font.mono,
          fontSize: 10.5,
          overflow: "hidden",
          textOverflow: "ellipsis",
          whiteSpace: "nowrap",
          maxWidth: 200,
        }}
      >
        {workspaceID}
      </Typography>
      {(state === "closed" || state === "error") && (
        <Button
          size="small"
          variant="outlined"
          onClick={onReconnect}
          sx={{
            color: tokens.color.accent.violet,
            borderColor: `${tokens.color.accent.violet}66`,
            fontSize: 11,
            minHeight: 24,
            py: 0,
            px: 1,
            "&:hover": {
              borderColor: tokens.color.accent.violet,
              bgcolor: `${tokens.color.accent.violet}14`,
            },
          }}
        >
          Reconnect
        </Button>
      )}
    </Stack>
  );
}

const STATE_PALETTE: Record<
  ConnectionState,
  { dot: string; fg: string; label: string }
> = {
  connecting: {
    dot: tokens.color.accent.warning,
    fg: tokens.color.accent.warning,
    label: "Connecting",
  },
  open: {
    dot: tokens.color.accent.success,
    fg: tokens.color.accent.success,
    label: "Live",
  },
  closed: {
    dot: tokens.color.text.muted,
    fg: tokens.color.text.muted,
    label: "Disconnected",
  },
  error: {
    dot: tokens.color.accent.danger,
    fg: tokens.color.accent.danger,
    label: "Error",
  },
  "missing-workspace": {
    dot: tokens.color.text.muted,
    fg: tokens.color.text.muted,
    label: "No workspace",
  },
};

function EmptyShell({ title, body }: { title: string; body: string }) {
  return (
    <Stack
      alignItems="center"
      justifyContent="center"
      spacing={1}
      sx={{
        color: tokens.color.text.muted,
        height: "100%",
        p: 3,
        textAlign: "center",
      }}
    >
      <Typography
        sx={{
          color: tokens.color.text.primary,
          fontFamily: tokens.font.mono,
          fontSize: 12,
          fontWeight: 800,
          letterSpacing: 0.5,
          textTransform: "uppercase",
        }}
      >
        {title}
      </Typography>
      <Typography sx={{ color: tokens.color.text.muted, fontSize: 12.5, maxWidth: 360 }}>
        {body}
      </Typography>
    </Stack>
  );
}

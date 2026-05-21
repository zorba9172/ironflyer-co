'use client';

import { useEffect, useRef } from 'react';
import { Box, Typography } from '@mui/material';
import { runtime } from '../../../lib/runtime';
import { tokens } from '../../../lib/theme';

import '@xterm/xterm/css/xterm.css';

interface Props {
  workspaceId: string | null;
}

// Terminal mounts xterm.js into a div and bridges it to the runtime WS.
// Binary frames carry I/O; text frames carry control messages (resize).
export function Terminal({ workspaceId }: Props) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const termRef = useRef<any>(null);
  const fitRef = useRef<any>(null);

  useEffect(() => {
    if (!workspaceId || !containerRef.current) return;
    let cancelled = false;

    (async () => {
      // Dynamic imports so xterm.js never enters SSR.
      const { Terminal: XTerm } = await import('@xterm/xterm');
      const { FitAddon } = await import('@xterm/addon-fit');
      if (cancelled) return;

      const term = new XTerm({
        fontFamily: tokens.font.mono,
        fontSize: 13,
        theme: {
          background: tokens.color.bg.inset,
          foreground: tokens.color.text.primary,
          cursor: tokens.color.accent.lime,
        },
        cursorBlink: true,
        scrollback: 2000,
      });
      const fit = new FitAddon();
      term.loadAddon(fit);
      term.open(containerRef.current!);
      fit.fit();
      termRef.current = term;
      fitRef.current = fit;

      const ws = new WebSocket(runtime.terminalURL(workspaceId));
      ws.binaryType = 'arraybuffer';
      wsRef.current = ws;

      ws.onopen = () => {
        const { rows, cols } = term;
        ws.send(JSON.stringify({ type: 'resize', rows, cols }));
        term.writeln(`\x1b[32m▍ ironflyer\x1b[0m  workspace ${workspaceId}`);
      };
      ws.onmessage = (e) => {
        if (typeof e.data === 'string') {
          // control frame from server (errors)
          try {
            const ctrl = JSON.parse(e.data);
            if (ctrl.type === 'error') term.writeln(`\x1b[31m${ctrl.msg}\x1b[0m`);
          } catch {}
        } else {
          term.write(new Uint8Array(e.data));
        }
      };
      ws.onclose = () => term.writeln(`\r\n\x1b[2mconnection closed\x1b[0m`);
      ws.onerror = () => term.writeln(`\r\n\x1b[31mconnection error\x1b[0m`);

      term.onData((d: string) => {
        if (ws.readyState === WebSocket.OPEN) ws.send(new TextEncoder().encode(d));
      });

      const onResize = () => {
        try {
          fit.fit();
          if (ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify({ type: 'resize', rows: term.rows, cols: term.cols }));
          }
        } catch {}
      };
      window.addEventListener('resize', onResize);

      return () => {
        window.removeEventListener('resize', onResize);
      };
    })();

    return () => {
      cancelled = true;
      try { wsRef.current?.close(); } catch {}
      try { termRef.current?.dispose(); } catch {}
    };
  }, [workspaceId]);

  if (!workspaceId) {
    return (
      <Box sx={{ height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Typography variant="body2" color="text.secondary">
          No workspace attached. Open the Files tab and click <b>Create workspace</b>.
        </Typography>
      </Box>
    );
  }

  return (
    <Box ref={containerRef} sx={{
      height: '100%', minHeight: 360,
      bgcolor: tokens.color.bg.inset, borderRadius: 2, p: 1,
      '& .xterm': { height: '100%' },
      '& .xterm-viewport': { backgroundColor: 'transparent !important' },
    }} />
  );
}

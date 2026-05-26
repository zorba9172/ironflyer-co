"use client";

// ExpoQRCard — visual landing card for an Expo Metro session. Renders
// the QR payload that the Expo Go app scans plus the LAN / tunnel
// URLs. The QR itself depends on the optional `qrcode.react` lib; if
// the library is not installed we render a labeled placeholder so the
// surface still describes its purpose without claiming a working scan.
//
// The card is intentionally compact: status dot, QR (or placeholder),
// then URL rows. Every color comes from tokens / theme.

import {
  ContentCopyRounded,
  OpenInNewRounded,
  RefreshRounded,
} from "@mui/icons-material";
import {
  Box,
  Button,
  IconButton,
  Stack,
  Tooltip,
  Typography,
} from "@mui/material";
import { tokens } from "../../../theme";

export interface ExpoQRCardProps {
  payload: string;
  lanUrl: string;
  tunnelUrl?: string;
  running: boolean;
  metroPort?: number;
  startedAt?: string;
  onRefresh?: () => void;
}

function formatRelative(iso?: string): string {
  if (!iso) {
    return "—";
  }
  const t = Date.parse(iso);
  if (Number.isNaN(t)) {
    return "—";
  }
  const diff = Date.now() - t;
  if (diff < 0) {
    return "just now";
  }
  const sec = Math.floor(diff / 1000);
  if (sec < 60) {
    return sec + "s ago";
  }
  const min = Math.floor(sec / 60);
  if (min < 60) {
    return min + "m ago";
  }
  const hr = Math.floor(min / 60);
  if (hr < 24) {
    return hr + "h ago";
  }
  const day = Math.floor(hr / 24);
  return day + "d ago";
}

// `qrcode.react` is intentionally NOT a hard dependency yet. When the
// mobile-runtime ticket lands we'll add the dep and swap this gate to
// a real `<QRCodeSVG />` render. Until then we keep a clearly labeled
// placeholder so the surface tells the truth.
const QR_LIB_AVAILABLE = false;

function QRSurface({ payload }: { payload: string }) {
  if (!QR_LIB_AVAILABLE) {
    return (
      <Box
        sx={{
          alignItems: "center",
          bgcolor: tokens.color.bg.inset,
          border: `1px dashed ${tokens.color.border.subtle}`,
          borderRadius: 1,
          color: tokens.color.text.muted,
          display: "flex",
          fontFamily: tokens.font.mono,
          fontSize: 11,
          height: 200,
          justifyContent: "center",
          lineHeight: 1.4,
          p: 1.5,
          textAlign: "center",
          width: 200,
        }}
        aria-label="QR placeholder"
      >
        Install qrcode.react to render QR
      </Box>
    );
  }
  // Reserved render path for when qrcode.react is installed.
  return (
    <Box
      sx={{
        bgcolor: tokens.color.text.primary,
        borderRadius: 1,
        height: 200,
        width: 200,
      }}
      aria-label="QR code"
      data-qr-payload={payload}
    />
  );
}

function copyToClipboard(value: string) {
  if (typeof navigator !== "undefined" && navigator.clipboard) {
    void navigator.clipboard.writeText(value).catch(() => undefined);
  }
}

function UrlRow({ label, url }: { label: string; url: string }) {
  return (
    <Stack
      direction="row"
      spacing={1}
      sx={{
        alignItems: "center",
        bgcolor: tokens.color.bg.inset,
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: 1,
        px: 1,
        py: 0.5,
      }}
    >
      <Typography
        sx={{
          color: tokens.color.text.muted,
          fontFamily: tokens.font.mono,
          fontSize: 10.5,
          letterSpacing: 0.6,
          textTransform: "uppercase",
          width: 56,
        }}
      >
        {label}
      </Typography>
      <Typography
        sx={{
          color: tokens.color.text.secondary,
          flex: 1,
          fontFamily: tokens.font.mono,
          fontSize: 12,
          overflow: "hidden",
          textOverflow: "ellipsis",
          whiteSpace: "nowrap",
        }}
      >
        {url}
      </Typography>
      <Tooltip title="Copy URL" arrow>
        <IconButton
          size="small"
          aria-label={`Copy ${label} URL`}
          onClick={() => copyToClipboard(url)}
          sx={{ color: tokens.color.text.secondary }}
        >
          <ContentCopyRounded sx={{ fontSize: 14 }} />
        </IconButton>
      </Tooltip>
    </Stack>
  );
}

export function ExpoQRCard({
  payload,
  lanUrl,
  tunnelUrl,
  running,
  metroPort,
  startedAt,
  onRefresh,
}: ExpoQRCardProps) {
  const statusColor = running
    ? tokens.color.brand.mint
    : tokens.color.text.muted;
  const statusLabel = running ? "Metro running" : "Metro not running";
  // Prefer the tunnel URL for the QR payload — phones outside the LAN
  // cannot reach the workspace's container IP, only the public tunnel.
  const effectivePayload =
    tunnelUrl && tunnelUrl.length > 0 ? tunnelUrl : payload;
  const portLabel =
    typeof metroPort === "number" && metroPort > 0 ? metroPort : "—";
  const lastConnection = formatRelative(startedAt);

  return (
    <Box
      sx={{
        bgcolor: tokens.color.bg.surface,
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: 1,
        display: "flex",
        flexDirection: { xs: "column", md: "row" },
        gap: 2,
        p: 2,
      }}
    >
      <Box
        sx={{
          alignItems: "center",
          bgcolor: tokens.color.bg.inset,
          border: `1px solid ${tokens.color.border.subtle}`,
          borderRadius: 1,
          display: "flex",
          flex: "0 0 auto",
          justifyContent: "center",
          p: 1.5,
        }}
      >
        <QRSurface payload={effectivePayload} />
      </Box>
      <Stack spacing={1.25} sx={{ flex: 1, minWidth: 0 }}>
        <Stack
          direction="row"
          spacing={0.75}
          sx={{ alignItems: "center", justifyContent: "space-between" }}
        >
          <Stack direction="row" spacing={0.75} sx={{ alignItems: "center" }}>
            <Box
              sx={{
                bgcolor: statusColor,
                borderRadius: "50%",
                boxShadow: running
                  ? `0 0 0 4px ${tokens.color.brand.mint}33`
                  : "none",
                height: 8,
                width: 8,
              }}
            />
            <Typography
              sx={{
                color: tokens.color.text.secondary,
                fontFamily: tokens.font.mono,
                fontSize: 11,
                letterSpacing: 0.8,
                textTransform: "uppercase",
              }}
            >
              {statusLabel}
            </Typography>
          </Stack>
          {onRefresh ? (
            <Tooltip title="Refresh Metro session" arrow>
              <IconButton
                size="small"
                aria-label="Refresh Metro session"
                onClick={onRefresh}
                sx={{ color: tokens.color.text.secondary }}
              >
                <RefreshRounded sx={{ fontSize: 16 }} />
              </IconButton>
            </Tooltip>
          ) : null}
        </Stack>
        <Stack
          direction="row"
          spacing={2}
          sx={{ alignItems: "center", flexWrap: "wrap" }}
        >
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 11,
              letterSpacing: 0.4,
            }}
          >
            Metro running on port {portLabel}
          </Typography>
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 11,
              letterSpacing: 0.4,
            }}
          >
            Last connection: {lastConnection}
          </Typography>
        </Stack>
        <Typography
          sx={{
            color: tokens.color.text.primary,
            fontSize: 14,
            fontWeight: 600,
          }}
        >
          Scan with Expo Go
        </Typography>
        <Typography
          sx={{
            color: tokens.color.text.muted,
            fontSize: 12.5,
            lineHeight: 1.5,
          }}
        >
          Open Expo Go on your phone and scan the QR. The device must be
          on the same LAN as the workspace, or use the tunnel URL if
          you&apos;re off-network.
        </Typography>
        {tunnelUrl ? <UrlRow label="Tunnel" url={tunnelUrl} /> : null}
        <UrlRow label="LAN" url={lanUrl} />
        {tunnelUrl ? (
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontSize: 11.5,
            }}
          >
            Open on same WiFi?{" "}
            <Box
              component="a"
              href={lanUrl}
              target="_blank"
              rel="noopener noreferrer"
              sx={{
                color: tokens.color.accent.violet,
                textDecoration: "none",
              }}
            >
              Use LAN URL
            </Box>
          </Typography>
        ) : null}
        <Stack direction="row" spacing={1}>
          <Button
            variant="contained"
            color="primary"
            size="small"
            startIcon={<OpenInNewRounded sx={{ fontSize: 16 }} />}
            component="a"
            href={tunnelUrl ?? lanUrl}
            target="_blank"
            rel="noopener noreferrer"
            disabled={!running}
          >
            Open in Expo Go
          </Button>
          <Button
            variant="outlined"
            color="inherit"
            size="small"
            onClick={() => copyToClipboard(effectivePayload)}
          >
            Copy payload
          </Button>
        </Stack>
      </Stack>
    </Box>
  );
}

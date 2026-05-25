"use client";

// ChatPanel — the left pane of the Studio split-view.
//
// Composition: [header (agent identity + status pill)] → MessageList →
// SuggestionsRow → ChatComposer. The panel itself owns no data; the
// page resolves the execution + chat buffer and passes everything in.

import { Box, Chip, Stack, Typography } from "@mui/material";
import type { ReactNode } from "react";
import { tokens } from "../../theme";
import { ChatComposer } from "./ChatComposer";
import { MessageList } from "./MessageList";
import { SuggestionsRow, type StudioStatusBucket } from "./SuggestionsRow";
import type { StudioAttachment, StudioMessage } from "./types";

export interface ChatPanelProps {
  messages: StudioMessage[];
  status: StudioStatusBucket;
  pending: boolean;
  onSend: (text: string, attachments?: StudioAttachment[]) => void | Promise<void>;
  onStop?: () => void;
  onRetry?: () => void;
  userInitials?: string;
  agentLabel?: string;
  // Optional slot rendered just below the chat header (e.g. StudioContextBar).
  contextBar?: ReactNode;
}

function statusLabel(s: StudioStatusBucket): { label: string; tone: "live" | "ok" | "bad" | "idle" } {
  switch (s) {
    case "running":
      return { label: "Building live", tone: "live" };
    case "succeeded":
      return { label: "Ready", tone: "ok" };
    case "failed":
      return { label: "Failed", tone: "bad" };
    default:
      return { label: "Idle", tone: "idle" };
  }
}

export function ChatPanel({
  messages,
  status,
  pending,
  onSend,
  onStop,
  onRetry,
  userInitials,
  agentLabel = "Ironflyer",
  contextBar,
}: ChatPanelProps) {
  const s = statusLabel(status);
  const pillBg =
    s.tone === "live"
      ? `${tokens.color.accent.success}1c`
      : s.tone === "ok"
      ? `${tokens.color.accent.success}1c`
      : s.tone === "bad"
      ? `${tokens.color.accent.danger}1c`
      : tokens.color.bg.surfaceRaised;
  const pillFg =
    s.tone === "live"
      ? tokens.color.accent.success
      : s.tone === "ok"
      ? tokens.color.accent.success
      : s.tone === "bad"
      ? tokens.color.accent.danger
      : tokens.color.text.secondary;

  return (
    <Box
      sx={{
        bgcolor: `${tokens.color.bg.surface}d6`,
        display: "flex",
        flexDirection: "column",
        height: "100%",
        minWidth: 0,
        borderRight: `1px solid ${tokens.color.border.subtle}`,
      }}
    >
      <Stack
        direction="row"
        spacing={1.25}
        sx={{
          alignItems: "center",
          borderBottom: `1px solid ${tokens.color.border.subtle}`,
          bgcolor: `${tokens.color.bg.surfaceRaised}eb`,
          px: 1.75,
          py: 1,
        }}
      >
        <Box
          sx={{
            alignItems: "center",
            bgcolor: tokens.color.bg.base,
            border: `1px solid ${tokens.color.accent.violet}66`,
            borderRadius: 1,
            color: tokens.color.accent.violet,
            display: "flex",
            fontFamily: tokens.font.mono,
            fontSize: 14,
            fontWeight: 900,
            height: 28,
            justifyContent: "center",
            width: 28,
          }}
        >
          ◢
        </Box>
        <Stack spacing={0.1} sx={{ flex: 1, minWidth: 0 }}>
          <Typography
            sx={{
              color: tokens.color.text.primary,
              fontSize: 13,
              fontWeight: 800,
              letterSpacing: 0.2,
            }}
          >
            {agentLabel}
          </Typography>
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 10.5,
              letterSpacing: 0.4,
              textTransform: "uppercase",
            }}
          >
            Finisher loop · gates enforced
          </Typography>
        </Stack>
        <Chip
          size="small"
          label={s.label.toUpperCase()}
          sx={{
            bgcolor: pillBg,
            color: pillFg,
            border: `1px solid ${pillFg}55`,
            fontFamily: tokens.font.mono,
            fontSize: 10,
            fontWeight: 800,
            height: 22,
            letterSpacing: 0.8,
            borderRadius: 0.75,
            "& .MuiChip-label": { px: 1 },
          }}
        />
      </Stack>
      {contextBar}
      <MessageList
        messages={messages}
        userInitials={userInitials}
        onRetry={onRetry}
      />
      <SuggestionsRow status={status} onPick={onSend} disabled={pending} />
      <ChatComposer
        onSend={onSend}
        onStop={onStop}
        pending={pending}
        disabled={status === "failed" && !onRetry}
      />
    </Box>
  );
}

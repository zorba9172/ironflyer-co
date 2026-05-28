"use client";

// ChatPanel — DeepSeek-inspired studio chat.
//
// The panel keeps the old data contract, but the UI is now a natural
// assistant surface: quiet header, centered empty state, readable
// conversation stream, and a large composer that owns the main action.

import {
  AutoAwesomeRounded,
  BoltRounded,
  CodeRounded,
  KeyboardArrowDownRounded,
  RocketLaunchRounded,
} from "@mui/icons-material";
import { Box, Button, Chip, Stack, Typography } from "@mui/material";
import type { SvgIconComponent } from "@mui/icons-material";
import type { ReactNode } from "react";
import { tokens } from "../../theme";
import { ChatComposer } from "./ChatComposer";
import { MessageList } from "./MessageList";
import { suggestionsFor, type StudioStatusBucket } from "./SuggestionsRow";
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

const PROMPT_ICONS: SvgIconComponent[] = [
  AutoAwesomeRounded,
  CodeRounded,
  RocketLaunchRounded,
];

export function ChatPanel({
  messages,
  status,
  pending,
  onSend,
  onStop,
  onRetry,
  userInitials,
  agentLabel = "Ironflyer",
}: ChatPanelProps) {
  const s = statusLabel(status);
  const pillFg =
    s.tone === "live"
      ? tokens.color.accent.success
      : s.tone === "ok"
      ? tokens.color.accent.success
      : s.tone === "bad"
      ? tokens.color.accent.danger
      : tokens.color.text.secondary;
  const empty = messages.length === 0;
  const suggestions = suggestionsFor(status).slice(0, 3);

  return (
    <Box
      sx={{
        bgcolor: "#070817",
        display: "flex",
        flexDirection: "column",
        height: "100%",
        minWidth: 0,
        overflow: "hidden",
      }}
    >
      <Stack
        direction="row"
        spacing={1}
        sx={{
          alignItems: "center",
          borderBottom: `1px solid ${tokens.color.border.subtle}`,
          bgcolor: "rgba(11,12,28,0.92)",
          minHeight: 56,
          px: 1.5,
        }}
      >
        <Stack spacing={0.1} sx={{ flex: 1, minWidth: 0 }}>
          <Typography
            sx={{
              color: tokens.color.text.primary,
              fontSize: 14,
              fontWeight: 850,
            }}
          >
            {agentLabel}
          </Typography>
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontSize: 12,
            }}
          >
            Build, debug, explain, ship
          </Typography>
        </Stack>
        <Button
          size="small"
          endIcon={<KeyboardArrowDownRounded sx={{ fontSize: 16 }} />}
          sx={{
            border: `1px solid ${tokens.color.border.subtle}`,
            borderRadius: 999,
            color: tokens.color.text.secondary,
            fontSize: 11.5,
            fontWeight: 750,
            minHeight: 30,
            px: 1,
            textTransform: "none",
            "&:hover": {
              bgcolor: tokens.color.bg.surfaceHover,
              borderColor: tokens.color.border.strong,
              color: tokens.color.text.primary,
            },
          }}
        >
          Reasoner
        </Button>
        <Chip
          size="small"
          label={s.label}
          sx={{
            bgcolor: "transparent",
            color: pillFg,
            border: `1px solid ${pillFg}40`,
            fontSize: 11,
            fontWeight: 800,
            height: 22,
            borderRadius: 999,
            "& .MuiChip-label": { px: 1 },
          }}
        />
      </Stack>
      {empty ? (
        <EmptyChatCanvas
          suggestions={suggestions}
          onPick={onSend}
          disabled={pending}
        />
      ) : (
        <MessageList
          messages={messages}
          userInitials={userInitials}
          onRetry={onRetry}
        />
      )}
      {pending && (
        <Stack
          direction="row"
          spacing={1}
          sx={{
            alignItems: "center",
            color: tokens.color.text.secondary,
            px: 2.1,
            py: 0.85,
          }}
        >
          <Box
            sx={{
              display: "flex",
              gap: 0.35,
              "& span": {
                animation: "ironflyerTyping 1.1s ease-in-out infinite",
                bgcolor: tokens.color.accent.violet,
                borderRadius: "50%",
                height: 4,
                opacity: 0.45,
                width: 4,
              },
              "& span:nth-of-type(2)": { animationDelay: "140ms" },
              "& span:nth-of-type(3)": { animationDelay: "280ms" },
              "@keyframes ironflyerTyping": {
                "0%, 80%, 100%": { opacity: 0.28, transform: "translateY(0)" },
                "40%": { opacity: 1, transform: "translateY(-2px)" },
              },
            }}
            aria-hidden
          >
            <span />
            <span />
            <span />
          </Box>
          <Typography sx={{ fontSize: 12.5, color: tokens.color.text.muted }}>
            Ironflyer is thinking through the next change...
          </Typography>
        </Stack>
      )}
      <ChatComposer
        onSend={onSend}
        onStop={onStop}
        pending={pending}
        disabled={status === "failed" && !onRetry}
        placeholder="Ask for a change, a fix, a feature, or an explanation..."
        modelLabel="Ironflyer Reasoner"
      />
    </Box>
  );
}

function EmptyChatCanvas({
  suggestions,
  onPick,
  disabled,
}: {
  suggestions: string[];
  onPick: (text: string) => void | Promise<void>;
  disabled?: boolean;
}) {
  return (
    <Box
      sx={{
        alignItems: "center",
        display: "flex",
        flex: 1,
        justifyContent: "center",
        minHeight: 0,
        px: 2.2,
        py: 3,
      }}
    >
      <Stack spacing={2.2} sx={{ maxWidth: 360, textAlign: "center", width: "100%" }}>
        <Box
          sx={{
            alignItems: "center",
            background: `linear-gradient(135deg, ${tokens.color.accent.coral}, ${tokens.color.brand.magenta} 48%, ${tokens.color.accent.purple})`,
            borderRadius: 2,
            boxShadow: `0 18px 48px ${tokens.color.accent.violet}26`,
            color: tokens.color.text.primary,
            display: "flex",
            height: 44,
            justifyContent: "center",
            mx: "auto",
            width: 44,
          }}
        >
          <BoltRounded sx={{ fontSize: 22 }} />
        </Box>
        <Stack spacing={0.8}>
          <Typography sx={{ color: tokens.color.text.primary, fontSize: 22, fontWeight: 900 }}>
            What should we improve?
          </Typography>
          <Typography sx={{ color: tokens.color.text.muted, fontSize: 13.5, lineHeight: 1.55 }}>
            Ask in plain English. Ironflyer will update the build, explain the work, and keep the studio in sync.
          </Typography>
        </Stack>
        <Stack spacing={0.8}>
          {suggestions.map((text, index) => {
            const Icon = PROMPT_ICONS[index % PROMPT_ICONS.length];
            return (
              <Button
                key={text}
                disabled={disabled}
                onClick={() => void onPick(text)}
                startIcon={<Icon sx={{ fontSize: 17 }} />}
                sx={{
                  border: `1px solid ${tokens.color.border.subtle}`,
                  borderRadius: 1.5,
                  color: tokens.color.text.secondary,
                  fontSize: 12.5,
                  fontWeight: 750,
                  justifyContent: "flex-start",
                  minHeight: 40,
                  px: 1.2,
                  textAlign: "left",
                  textTransform: "none",
                  "&:hover": {
                    bgcolor: "rgba(255,255,255,0.04)",
                    borderColor: `${tokens.color.accent.violet}66`,
                    color: tokens.color.text.primary,
                  },
                }}
              >
                {text}
              </Button>
            );
          })}
        </Stack>
      </Stack>
    </Box>
  );
}

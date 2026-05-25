"use client";

// MessageBubble — single chat row. Visual variants per role:
//
//   user            → right-aligned dark surface tile with initials avatar
//   assistant       → left-aligned, "Ironflyer" header, optional thinking
//                     disclosure (collapsed by default), plaintext body
//   system          → full-width italic mono line ("Gate Coder passed …")
//   error           → red-bordered card with retry slot (callers wire onRetry)
//   costtick        → tiny inline chip; collapses gracefully into the flow
//   agent_progress  → A58. Left-aligned thin row with spinner — the
//                     agent just opened a new stage.
//   agent_action    → A58. Same row layout; flips between spinner
//                     (in-flight) and check/x icon (settled) and shows
//                     a click-to-expand summary when complete.
//   agent_result    → A58. Same shape as a settled agent_action; used
//                     only when no matching action existed to merge
//                     into.
//   refinement_ack  → A58. Centred lime chip; confirms the orchestrator
//                     picked up the user's refinement.

import {
  CachedRounded,
  CheckCircleRounded,
  ErrorRounded,
  ExpandLessRounded,
  ExpandMoreRounded,
  ReplayRounded,
} from "@mui/icons-material";
import {
  Avatar,
  Box,
  Button,
  Chip,
  CircularProgress,
  Collapse,
  IconButton,
  Stack,
  Typography,
} from "@mui/material";
import { memo, useState } from "react";
import { relativeTime } from "../../lib/relativeTime";
import { tokens } from "../../theme";
import type { StudioMessage } from "./types";

export interface MessageBubbleProps {
  message: StudioMessage;
  userInitials?: string;
  onRetry?: () => void;
}

function initials(name: string | null | undefined): string {
  if (!name) return "U";
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((p) => p[0].toUpperCase())
    .join("");
}

// AgentRow — the shared visual for agent_progress / agent_action /
// agent_result. Left-aligned, thin, with a leading status icon
// (spinner | check | error) and an optional collapsible summary.
function AgentRow({ message }: { message: StudioMessage }) {
  const [open, setOpen] = useState(false);
  const settled = message.inProgress === false;
  const success = message.success !== false; // default-true once settled
  const icon = !settled ? (
    <CircularProgress
      size={12}
      thickness={5}
      sx={{ color: tokens.color.accent.success }}
    />
  ) : success ? (
    <CheckCircleRounded
      sx={{ color: tokens.color.accent.success, fontSize: 14 }}
    />
  ) : (
    <ErrorRounded sx={{ color: tokens.color.accent.danger, fontSize: 14 }} />
  );
  const bodyColor = !settled
    ? tokens.color.text.secondary
    : success
    ? tokens.color.text.primary
    : tokens.color.accent.danger;
  const hasSummary = settled && !!message.summary;

  return (
    <Stack
      direction="row"
      spacing={1.25}
      sx={{ alignItems: "flex-start", py: 0.35, pl: 4.5 }}
    >
      <Box
        sx={{
          width: 16,
          height: 16,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          mt: "2px",
        }}
      >
        {icon}
      </Box>
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Stack
          direction="row"
          spacing={0.75}
          sx={{
            alignItems: "center",
            cursor: hasSummary ? "pointer" : "default",
          }}
          onClick={hasSummary ? () => setOpen((v) => !v) : undefined}
        >
          {message.stage && (
            <Typography
              sx={{
                color: tokens.color.text.muted,
                fontFamily: tokens.font.mono,
                fontSize: 10,
                fontWeight: 800,
                letterSpacing: 0.8,
                textTransform: "uppercase",
              }}
            >
              {message.stage}
            </Typography>
          )}
          <Typography
            sx={{
              color: bodyColor,
              fontSize: 12.5,
              lineHeight: 1.45,
              flex: 1,
              minWidth: 0,
              fontStyle: settled ? "normal" : "italic",
              wordBreak: "break-word",
            }}
          >
            {message.body}
          </Typography>
          {hasSummary && (
            <IconButton
              size="small"
              aria-label={open ? "Hide summary" : "Show summary"}
              sx={{ color: tokens.color.text.muted, p: 0 }}
            >
              {open ? (
                <ExpandLessRounded sx={{ fontSize: 16 }} />
              ) : (
                <ExpandMoreRounded sx={{ fontSize: 16 }} />
              )}
            </IconButton>
          )}
        </Stack>
        {hasSummary && (
          <Collapse in={open}>
            <Box
              sx={{
                mt: 0.5,
                ml: 0,
                pl: 1,
                borderLeft: `2px solid ${tokens.color.border.subtle}`,
                color: tokens.color.text.secondary,
                fontFamily: tokens.font.mono,
                fontSize: 11.5,
                lineHeight: 1.55,
                whiteSpace: "pre-wrap",
              }}
            >
              {message.summary}
            </Box>
          </Collapse>
        )}
      </Box>
    </Stack>
  );
}

function MessageBubbleImpl({ message, userInitials, onRetry }: MessageBubbleProps) {
  const [thinkingOpen, setThinkingOpen] = useState(false);

  if (message.role === "costtick") {
    return (
      <Box sx={{ display: "flex", justifyContent: "center", py: 0.5 }}>
        <Chip
          size="small"
          label={message.body}
          sx={{
            bgcolor: `${tokens.color.accent.success}14`,
            color: tokens.color.accent.success,
            border: `1px solid ${tokens.color.accent.success}33`,
            fontFamily: tokens.font.mono,
            fontSize: 11,
            fontWeight: 700,
            height: 22,
            borderRadius: 0.75,
            letterSpacing: 0.4,
          }}
        />
      </Box>
    );
  }

  if (message.role === "refinement_ack") {
    return (
      <Box sx={{ display: "flex", justifyContent: "center", py: 0.5 }}>
        <Chip
          size="small"
          icon={
            <CachedRounded
              sx={{ fontSize: 13, color: `${tokens.color.accent.success} !important` }}
            />
          }
          label={message.body}
          sx={{
            bgcolor: `${tokens.color.accent.success}1c`,
            color: tokens.color.accent.success,
            border: `1px solid ${tokens.color.accent.success}55`,
            fontFamily: tokens.font.mono,
            fontSize: 11,
            fontStyle: "italic",
            fontWeight: 700,
            height: 22,
            borderRadius: 0.75,
            letterSpacing: 0.3,
            "& .MuiChip-label": { px: 0.75 },
          }}
        />
      </Box>
    );
  }

  if (
    message.role === "agent_progress" ||
    message.role === "agent_action" ||
    message.role === "agent_result"
  ) {
    return <AgentRow message={message} />;
  }

  if (message.role === "system") {
    return (
      <Box sx={{ py: 0.5 }}>
        <Typography
          sx={{
            color: tokens.color.text.muted,
            fontFamily: tokens.font.mono,
            fontSize: 11.5,
            fontStyle: "italic",
            letterSpacing: 0.2,
            textAlign: "center",
          }}
        >
          {message.body}
        </Typography>
      </Box>
    );
  }

  if (message.role === "user") {
    return (
      <Stack
        direction="row"
        spacing={1.25}
        sx={{ justifyContent: "flex-end", alignItems: "flex-start", py: 0.5 }}
      >
        <Box
          sx={{
            maxWidth: "78%",
            bgcolor: tokens.color.bg.surfaceRaised,
            color: tokens.color.text.primary,
            border: `1px solid ${tokens.color.border.subtle}`,
            borderRadius: 1.25,
            px: 1.5,
            py: 1,
            whiteSpace: "pre-wrap",
            wordBreak: "break-word",
            fontSize: 13.5,
            lineHeight: 1.5,
          }}
        >
          {message.body}
          <Typography
            component="div"
            sx={{
              mt: 0.5,
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 10,
              letterSpacing: 0.4,
              textAlign: "right",
            }}
          >
            {relativeTime(message.createdAt)}
          </Typography>
        </Box>
        <Avatar
          sx={{
            width: 28,
            height: 28,
            background: `linear-gradient(135deg, ${tokens.color.accent.violet}, ${tokens.color.accent.purple})`,
            color: tokens.color.text.primary,
            fontSize: 11,
            fontWeight: 800,
            fontFamily: tokens.font.mono,
          }}
        >
          {initials(userInitials)}
        </Avatar>
      </Stack>
    );
  }

  if (message.role === "error") {
    return (
      <Stack
        direction="row"
        spacing={1.25}
        sx={{ alignItems: "flex-start", py: 0.5 }}
      >
        <Avatar
          sx={{
            width: 28,
            height: 28,
            bgcolor: tokens.color.accent.danger,
            color: tokens.color.text.primary,
            fontSize: 11,
            fontWeight: 800,
          }}
        >
          !
        </Avatar>
        <Box
          sx={{
            flex: 1,
            border: `1px solid ${tokens.color.accent.danger}55`,
            bgcolor: `${tokens.color.accent.danger}10`,
            borderRadius: 1.25,
            px: 1.5,
            py: 1,
          }}
        >
          <Typography
            sx={{
              color: tokens.color.accent.danger,
              fontFamily: tokens.font.mono,
              fontSize: 10.5,
              fontWeight: 800,
              letterSpacing: 0.8,
              mb: 0.5,
              textTransform: "uppercase",
            }}
          >
            Error
          </Typography>
          <Typography
            sx={{
              color: tokens.color.text.primary,
              fontSize: 13.5,
              lineHeight: 1.5,
              whiteSpace: "pre-wrap",
            }}
          >
            {message.body}
          </Typography>
          {onRetry && (
            <Button
              size="small"
              startIcon={<ReplayRounded sx={{ fontSize: 14 }} />}
              onClick={onRetry}
              sx={{
                mt: 1,
                color: tokens.color.accent.danger,
                fontFamily: tokens.font.mono,
                fontSize: 11,
                fontWeight: 700,
                minHeight: 28,
                px: 1,
                "&:hover": { bgcolor: `${tokens.color.accent.danger}1a` },
              }}
            >
              Retry
            </Button>
          )}
        </Box>
      </Stack>
    );
  }

  // assistant
  return (
    <Stack
      direction="row"
      spacing={1.25}
      sx={{ alignItems: "flex-start", py: 0.75 }}
    >
      <Avatar
        sx={{
          width: 28,
          height: 28,
          bgcolor: tokens.color.bg.surface,
          color: tokens.color.accent.violet,
          border: `1px solid ${tokens.color.accent.violet}55`,
          fontFamily: tokens.font.mono,
          fontSize: 13,
          fontWeight: 900,
        }}
      >
        ◢
      </Avatar>
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Stack direction="row" spacing={1} sx={{ alignItems: "center", mb: 0.5 }}>
          <Typography
            sx={{
              color: tokens.color.text.primary,
              fontSize: 12,
              fontWeight: 800,
              letterSpacing: 0.2,
            }}
          >
            Ironflyer
          </Typography>
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 10.5,
              letterSpacing: 0.4,
            }}
          >
            · {relativeTime(message.createdAt)}
          </Typography>
        </Stack>
        {message.thinking && (
          <Box sx={{ mb: 0.75 }}>
            <Stack
              direction="row"
              spacing={0.5}
              sx={{
                alignItems: "center",
                cursor: "pointer",
                color: tokens.color.text.muted,
                "&:hover": { color: tokens.color.text.secondary },
              }}
              onClick={() => setThinkingOpen((v) => !v)}
            >
              <IconButton
                size="small"
                sx={{ color: "inherit", p: 0 }}
                aria-label={thinkingOpen ? "Hide thought" : "Show thought"}
              >
                {thinkingOpen ? (
                  <ExpandLessRounded sx={{ fontSize: 16 }} />
                ) : (
                  <ExpandMoreRounded sx={{ fontSize: 16 }} />
                )}
              </IconButton>
              <Typography
                sx={{
                  color: "inherit",
                  fontFamily: tokens.font.mono,
                  fontSize: 11,
                  fontStyle: "italic",
                  letterSpacing: 0.2,
                }}
              >
                Thought for less than a second
              </Typography>
            </Stack>
            <Collapse in={thinkingOpen}>
              <Box
                sx={{
                  mt: 0.5,
                  ml: 2.25,
                  pl: 1,
                  borderLeft: `2px solid ${tokens.color.border.subtle}`,
                  color: tokens.color.text.secondary,
                  fontFamily: tokens.font.mono,
                  fontSize: 11.5,
                  lineHeight: 1.55,
                  whiteSpace: "pre-wrap",
                }}
              >
                {message.thinking}
              </Box>
            </Collapse>
          </Box>
        )}
        <Typography
          sx={{
            color: tokens.color.text.primary,
            fontSize: 13.75,
            lineHeight: 1.55,
            whiteSpace: "pre-wrap",
            wordBreak: "break-word",
          }}
        >
          {message.body}
        </Typography>
      </Box>
    </Stack>
  );
}

// Memoised export — the chat list re-renders on every new message,
// but each existing bubble is identity-stable (StudioMessage objects
// are immutable once produced by eventToMessage/applyIncomingMessage),
// so a default referential-equality check is correct and cheap.
export const MessageBubble = memo(MessageBubbleImpl);

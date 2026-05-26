"use client";

// MessageList — virtualized scroll container for the studio chat.
//
// Backed by react-virtuoso so a 500-message buffer keeps the DOM at
// roughly two viewports of bubbles instead of all 500. We use Virtuoso's
// `followOutput` to stay pinned to the bottom when the user is already
// there; when they scroll up to read history, the stick state lifts so
// new arrivals don't yank them forward.
//
// Auto-scroll behaviour:
//   - On mount, jump to the last index so reloads land at the latest
//     message.
//   - When new messages arrive AND the user is at the bottom, Virtuoso
//     scrolls smoothly to the new tail. When the user is scrolled away
//     it leaves them alone.

import { Box, Stack, Typography } from "@mui/material";
import { useEffect, useRef } from "react";
import { Virtuoso, type VirtuosoHandle } from "react-virtuoso";
import { tokens } from "../../theme";
import { MessageBubble } from "./MessageBubble";
import type { StudioMessage } from "./types";

export interface MessageListProps {
  messages: StudioMessage[];
  userInitials?: string;
  onRetry?: () => void;
}

export function MessageList({ messages, userInitials, onRetry }: MessageListProps) {
  const ref = useRef<VirtuosoHandle | null>(null);
  const atBottomRef = useRef(true);

  // On first non-empty render, jump straight to the tail so a reload
  // doesn't briefly flash the top of the buffer before scrolling.
  useEffect(() => {
    if (messages.length === 0) return;
    ref.current?.scrollToIndex({ index: messages.length - 1, behavior: "auto" });
    // We intentionally only run this when the message count crosses
    // from 0 → >0 (first hydration).
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [messages.length > 0]);

  if (messages.length === 0) {
    return (
      <Box
        sx={{
          flex: 1,
          minHeight: 0,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          px: 2,
          py: 1.5,
        }}
      >
        <Stack
          spacing={1}
          sx={{
            alignItems: "center",
            color: tokens.color.text.muted,
            textAlign: "center",
          }}
        >
          <Typography
            sx={{
              color: tokens.color.text.secondary,
              fontFamily: tokens.font.mono,
              fontSize: 11,
              letterSpacing: 1,
              textTransform: "uppercase",
            }}
          >
            Live execution feed
          </Typography>
          <Typography sx={{ color: tokens.color.text.muted, fontSize: 13, maxWidth: 320 }}>
            Type a follow-up below — Ironflyer will refine the build, run the gates, and
            stream every verdict here.
          </Typography>
        </Stack>
      </Box>
    );
  }

  return (
    <Box
      sx={{
        flex: 1,
        minHeight: 0,
        // Virtuoso owns its own scroll container; we still style our
        // scrollbar via the inner element selector to match the rest
        // of the cockpit chrome.
        "& [data-testid='virtuoso-scroller']": {
          scrollbarWidth: "thin",
          "&::-webkit-scrollbar": { width: 6 },
          "&::-webkit-scrollbar-thumb": {
            background: tokens.color.border.subtle,
            borderRadius: 3,
          },
        },
      }}
    >
      <Virtuoso<StudioMessage>
        ref={ref}
        data={messages}
        computeItemKey={(_, m) => m.id}
        followOutput={(isAtBottom) => (isAtBottom ? "smooth" : false)}
        atBottomStateChange={(atBottom) => {
          atBottomRef.current = atBottom;
        }}
        increaseViewportBy={{ top: 200, bottom: 200 }}
        style={{ height: "100%" }}
        itemContent={(_, message) => (
          <Box sx={{ px: 2, py: 0.15 }}>
            <MessageBubble
              message={message}
              userInitials={userInitials}
              onRetry={message.role === "error" ? onRetry : undefined}
            />
          </Box>
        )}
      />
    </Box>
  );
}

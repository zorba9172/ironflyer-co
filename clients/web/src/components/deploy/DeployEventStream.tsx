"use client";

// DeployEventStream — Apollo subscription wrapper that paints live
// deploy events into the Events tab on /deploy/[id]. Events are kept
// in-memory (no persistence) so the list resets on tab close — a
// future enhancement can hydrate from a server-side event log.

import { Box, Stack, Typography } from "@mui/material";
import { useEffect, useState } from "react";
import { useDeployFeedSubscription } from "../../lib/gql/__generated__";
import { formatDateTime } from "../../lib/format";
import { tokens } from "../../theme";

interface StreamEvent {
  id: string;
  eventType: string;
  payload: unknown;
  createdAt: string;
}

export interface DeployEventStreamProps {
  deployID: string;
}

export function DeployEventStream({ deployID }: DeployEventStreamProps) {
  const [events, setEvents] = useState<StreamEvent[]>([]);
  const { data, error } = useDeployFeedSubscription({
    variables: { id: deployID },
  });

  useEffect(() => {
    if (!data?.deployFeed) return;
    const evt = data.deployFeed;
    const key = `${evt.deployID}-${evt.eventType}-${evt.createdAt}`;
    setEvents((prev) => {
      if (prev.some((p) => p.id === key)) return prev;
      return [
        ...prev,
        { id: key, eventType: evt.eventType, payload: evt.payload, createdAt: evt.createdAt },
      ];
    });
  }, [data]);

  if (error) {
    return (
      <Typography sx={{ color: tokens.color.accent.danger, fontSize: 13 }}>
        Event stream error: {error.message}
      </Typography>
    );
  }

  if (events.length === 0) {
    return (
      <Box
        sx={{
          border: `1px dashed ${tokens.color.border.subtle}`,
          borderRadius: 1,
          p: 4,
          textAlign: "center",
          color: tokens.color.text.muted,
          fontSize: 13.5,
        }}
      >
        Listening for deploy events… nothing has fired yet.
      </Box>
    );
  }

  return (
    <Stack
      spacing={0}
      sx={{
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: 1,
        bgcolor: tokens.color.bg.inset,
        overflow: "hidden",
      }}
    >
      {events
        .slice()
        .reverse()
        .map((e) => (
          <Stack
            key={e.id}
            direction="row"
            spacing={1.5}
            sx={{
              px: 1.5,
              py: 1,
              borderBottom: `1px solid ${tokens.color.border.subtle}`,
              "&:last-of-type": { borderBottom: 0 },
            }}
          >
            <Typography
              sx={{
                fontFamily: tokens.font.mono,
                fontSize: 11.5,
                color: tokens.color.text.muted,
                minWidth: 168,
              }}
            >
              {formatDateTime(e.createdAt)}
            </Typography>
            <Typography
              sx={{
                fontFamily: tokens.font.mono,
                fontSize: 12,
                color: tokens.color.accent.violet,
                minWidth: 160,
              }}
            >
              {e.eventType}
            </Typography>
            <Typography
              sx={{
                flex: 1,
                fontFamily: tokens.font.mono,
                fontSize: 12,
                color: tokens.color.text.secondary,
                whiteSpace: "pre-wrap",
                wordBreak: "break-word",
              }}
            >
              {summarisePayload(e.payload)}
            </Typography>
          </Stack>
        ))}
    </Stack>
  );
}

function summarisePayload(payload: unknown): string {
  if (payload === null || payload === undefined) return "—";
  if (typeof payload === "string") return payload;
  try {
    return JSON.stringify(payload);
  } catch {
    return String(payload);
  }
}

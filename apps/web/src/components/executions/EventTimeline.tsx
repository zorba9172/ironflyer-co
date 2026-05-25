"use client";

// EventTimeline — vertical list of timestamped events. Two modes:
//
//   1. Curated (TimelineEvent[]) — the static milestone stripe used by
//      legacy callers (overview tab). Renders titles + descriptions with
//      colour dots per kind.
//   2. Live (LiveEvent[]) — the streaming feed surfaced by the cockpit
//      page. Supports filter chips, live search, expand-for-payload,
//      and auto-scroll-to-newest when the user is parked at the bottom.
//
// Both modes share one row template so we don't duplicate styles.

import { ExpandLessRounded, ExpandMoreRounded } from "@mui/icons-material";
import {
  Box,
  IconButton,
  InputAdornment,
  Stack,
  TextField,
  Typography,
} from "@mui/material";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { formatDateTime } from "../../lib/format";
import { relativeTime } from "../../lib/relativeTime";
import { tokens } from "../../theme";
import { FilterChips, type FilterChipOption } from "./FilterChips";

export interface TimelineEvent {
  id: string;
  timestamp: string | null | undefined;
  title: string;
  description?: string;
  kind?: "neutral" | "success" | "warning" | "danger" | "accent";
}

export interface LiveEvent {
  id: string;
  eventType: string;
  createdAt: string;
  payload?: unknown;
}

const KIND_COLOR: Record<NonNullable<TimelineEvent["kind"]>, string> = {
  neutral: tokens.color.text.muted,
  success: tokens.color.accent.success,
  warning: tokens.color.accent.warning,
  danger: tokens.color.accent.danger,
  accent: tokens.color.accent.violet,
};

// ── Curated milestone mode ─────────────────────────────────────────

export interface EventTimelineProps {
  events: TimelineEvent[];
  emptyLabel?: string;
}

export function EventTimeline({
  events,
  emptyLabel = "No events yet.",
}: EventTimelineProps) {
  const filtered = events.filter((e) => e.timestamp);
  if (filtered.length === 0) {
    return (
      <Typography sx={{ color: tokens.color.text.muted, fontSize: 13 }}>
        {emptyLabel}
      </Typography>
    );
  }
  return (
    <Box component="ol" sx={{ m: 0, p: 0, listStyle: "none" }}>
      {filtered.map((e, i) => (
        <Stack
          key={e.id}
          direction="row"
          spacing={1.5}
          component="li"
          sx={{ position: "relative", pl: 0.5, pb: i === filtered.length - 1 ? 0 : 2 }}
        >
          <TimelineDot
            color={KIND_COLOR[e.kind ?? "neutral"]}
            connector={i < filtered.length - 1}
          />
          <Box sx={{ flex: 1, minWidth: 0, pb: 0.5 }}>
            <Typography
              sx={{
                color: tokens.color.text.primary,
                fontWeight: 700,
                fontSize: 13.5,
              }}
            >
              {e.title}
            </Typography>
            {e.description && (
              <Typography sx={{ mt: 0.25, color: tokens.color.text.secondary, fontSize: 13 }}>
                {e.description}
              </Typography>
            )}
            <Typography
              sx={{
                mt: 0.4,
                color: tokens.color.text.muted,
                fontFamily: tokens.font.mono,
                fontSize: 11,
              }}
            >
              {formatDateTime(e.timestamp)} · {relativeTime(e.timestamp)}
            </Typography>
          </Box>
        </Stack>
      ))}
    </Box>
  );
}

// ── Live event-feed mode ───────────────────────────────────────────

export type LiveEventCategory =
  | "all"
  | "gate"
  | "patch"
  | "provider"
  | "ledger"
  | "error";

export interface LiveEventTimelineProps {
  events: LiveEvent[];
  emptyLabel?: ReactNode;
  height?: number | string;
}

const FILTER_OPTIONS: FilterChipOption<LiveEventCategory>[] = [
  { value: "all", label: "All" },
  { value: "gate", label: "Gate" },
  { value: "patch", label: "Patch" },
  { value: "provider", label: "Provider" },
  { value: "ledger", label: "Ledger" },
  { value: "error", label: "Error" },
];

function categorize(eventType: string): LiveEventCategory {
  const t = eventType.toLowerCase();
  if (t.includes("error") || t.includes("fail") || t.includes("kill")) return "error";
  if (t.includes("gate") || t.includes("verdict") || t.includes("approval")) return "gate";
  if (t.includes("patch") || t.includes("diff") || t.includes("file")) return "patch";
  if (t.includes("provider") || t.includes("model") || t.includes("token") || t.includes("stream"))
    return "provider";
  if (t.includes("ledger") || t.includes("wallet") || t.includes("spend") || t.includes("hold"))
    return "ledger";
  return "all";
}

function categoryColor(cat: LiveEventCategory): string {
  switch (cat) {
    case "gate":
      return tokens.color.accent.purple;
    case "patch":
      return tokens.color.accent.sky;
    case "provider":
      return tokens.color.accent.violet;
    case "ledger":
      return tokens.color.accent.success;
    case "error":
      return tokens.color.accent.danger;
    default:
      return tokens.color.text.muted;
  }
}

function summarize(eventType: string, payload: unknown): string {
  if (payload && typeof payload === "object") {
    const obj = payload as Record<string, unknown>;
    for (const key of [
      "message",
      "summary",
      "reason",
      "title",
      "name",
      "verdict",
      "status",
    ]) {
      const v = obj[key];
      if (typeof v === "string" && v.trim().length > 0) return v;
    }
  }
  return eventType.replace(/_/g, " ");
}

export function LiveEventTimeline({
  events,
  emptyLabel = "Waiting for the runtime to claim this execution…",
  height = 520,
}: LiveEventTimelineProps) {
  const [filter, setFilter] = useState<LiveEventCategory>("all");
  const [query, setQuery] = useState("");
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const stickyBottomRef = useRef(true);

  const onScroll = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;
    const dist = el.scrollHeight - el.scrollTop - el.clientHeight;
    stickyBottomRef.current = dist < 24;
  }, []);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return events.filter((e) => {
      const cat = categorize(e.eventType);
      if (filter !== "all" && cat !== filter) return false;
      if (!q) return true;
      const hay = `${e.eventType} ${JSON.stringify(e.payload ?? "")}`.toLowerCase();
      return hay.includes(q);
    });
  }, [events, filter, query]);

  useEffect(() => {
    if (!stickyBottomRef.current) return;
    const el = scrollRef.current;
    if (!el) return;
    el.scrollTop = el.scrollHeight;
  }, [filtered.length]);

  return (
    <Stack spacing={1.25} sx={{ minHeight: 0, flex: 1 }}>
      <Stack
        direction={{ xs: "column", md: "row" }}
        spacing={1.25}
        alignItems={{ md: "center" }}
        sx={{
          position: "sticky",
          top: 0,
          zIndex: 1,
          bgcolor: tokens.color.bg.surface,
          borderBottom: `1px solid ${tokens.color.border.subtle}`,
          py: 1,
        }}
      >
        <FilterChips options={FILTER_OPTIONS} value={filter} onChange={setFilter} />
        <Box sx={{ flex: 1 }} />
        <TextField
          size="small"
          placeholder="Filter events"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          slotProps={{
            input: {
              sx: { fontFamily: tokens.font.mono, fontSize: 12.5 },
              startAdornment: (
                <InputAdornment position="start" sx={{ color: tokens.color.text.muted, fontSize: 12 }}>
                  /
                </InputAdornment>
              ),
            },
          }}
          sx={{ minWidth: { xs: "100%", md: 220 } }}
        />
      </Stack>

      <Box
        ref={scrollRef}
        onScroll={onScroll}
        sx={{
          flex: 1,
          minHeight: 0,
          maxHeight: height,
          overflowY: "auto",
          border: `1px solid ${tokens.color.border.subtle}`,
          borderRadius: 1,
          bgcolor: tokens.color.bg.inset,
        }}
      >
        {filtered.length === 0 ? (
          <Box sx={{ p: 3, textAlign: "center" }}>
            <Typography sx={{ color: tokens.color.text.muted, fontSize: 13 }}>
              {emptyLabel}
            </Typography>
          </Box>
        ) : (
          <Box component="ol" sx={{ m: 0, p: 0, listStyle: "none" }}>
            {filtered.map((e, i) => {
              const cat = categorize(e.eventType);
              const color = categoryColor(cat);
              const isOpen = !!expanded[e.id];
              const payloadStr = formatPayload(e.payload);
              const hasPayload = payloadStr !== null;
              return (
                <Box
                  component="li"
                  key={e.id}
                  sx={{
                    px: 1.5,
                    py: 1,
                    borderBottom:
                      i === filtered.length - 1
                        ? "none"
                        : `1px solid ${tokens.color.border.subtle}`,
                    "&:hover": { bgcolor: tokens.color.bg.surfaceHover },
                  }}
                >
                  <Stack direction="row" spacing={1.25} alignItems="flex-start">
                    <TimelineDot color={color} />
                    <Box sx={{ flex: 1, minWidth: 0 }}>
                      <Stack direction="row" spacing={1} alignItems="baseline">
                        <Typography
                          sx={{
                            fontFamily: tokens.font.mono,
                            fontWeight: 700,
                            fontSize: 12,
                            color,
                            letterSpacing: 0.4,
                            textTransform: "uppercase",
                          }}
                        >
                          {cat === "all" ? "event" : cat}
                        </Typography>
                        <Typography
                          sx={{
                            fontFamily: tokens.font.mono,
                            fontSize: 11,
                            color: tokens.color.text.muted,
                            ml: "auto",
                            whiteSpace: "nowrap",
                          }}
                        >
                          {relativeTime(e.createdAt)}
                        </Typography>
                      </Stack>
                      <Typography
                        sx={{
                          mt: 0.25,
                          color: tokens.color.text.primary,
                          fontSize: 13.5,
                          fontWeight: 600,
                          overflow: "hidden",
                          textOverflow: "ellipsis",
                          display: "-webkit-box",
                          WebkitLineClamp: isOpen ? "unset" : 2,
                          WebkitBoxOrient: "vertical",
                        }}
                      >
                        {summarize(e.eventType, e.payload)}
                      </Typography>
                      <Stack
                        direction="row"
                        spacing={1}
                        alignItems="center"
                        sx={{ mt: 0.5 }}
                      >
                        <Typography
                          sx={{
                            color: tokens.color.text.muted,
                            fontFamily: tokens.font.mono,
                            fontSize: 10.5,
                          }}
                        >
                          {e.eventType}
                        </Typography>
                        <Typography
                          sx={{
                            color: tokens.color.text.muted,
                            fontFamily: tokens.font.mono,
                            fontSize: 10.5,
                          }}
                        >
                          · {formatDateTime(e.createdAt)}
                        </Typography>
                      </Stack>
                      {isOpen && hasPayload && (
                        <Box
                          component="pre"
                          sx={{
                            mt: 1,
                            mb: 0,
                            p: 1.25,
                            border: `1px solid ${tokens.color.border.subtle}`,
                            borderRadius: 0.75,
                            bgcolor: tokens.color.bg.surface,
                            fontFamily: tokens.font.mono,
                            fontSize: 11.5,
                            color: tokens.color.text.secondary,
                            whiteSpace: "pre-wrap",
                            wordBreak: "break-word",
                            maxHeight: 220,
                            overflow: "auto",
                          }}
                        >
                          {payloadStr}
                        </Box>
                      )}
                    </Box>
                    {hasPayload && (
                      <IconButton
                        size="small"
                        onClick={() =>
                          setExpanded((m) => ({ ...m, [e.id]: !m[e.id] }))
                        }
                        aria-label={isOpen ? "Collapse payload" : "Expand payload"}
                        sx={{ color: tokens.color.text.secondary }}
                      >
                        {isOpen ? (
                          <ExpandLessRounded fontSize="small" />
                        ) : (
                          <ExpandMoreRounded fontSize="small" />
                        )}
                      </IconButton>
                    )}
                  </Stack>
                </Box>
              );
            })}
          </Box>
        )}
      </Box>
    </Stack>
  );
}

function formatPayload(payload: unknown): string | null {
  if (payload === null || payload === undefined) return null;
  if (typeof payload === "string") {
    const s = payload.trim();
    return s.length > 0 ? s : null;
  }
  try {
    const s = JSON.stringify(payload, null, 2);
    if (!s || s === "null" || s === "{}" || s === "[]") return null;
    return s;
  } catch {
    return null;
  }
}

function TimelineDot({
  color,
  connector,
}: {
  color: string;
  connector?: boolean;
}) {
  return (
    <Box
      sx={{
        position: "relative",
        flex: "0 0 16px",
        display: "flex",
        alignItems: "stretch",
        justifyContent: "center",
      }}
    >
      <Box
        sx={{
          width: 10,
          height: 10,
          borderRadius: "50%",
          bgcolor: color,
          mt: 0.5,
          flexShrink: 0,
          boxShadow: `0 0 0 2px ${color}33`,
        }}
      />
      {connector && (
        <Box
          sx={{
            position: "absolute",
            top: 14,
            bottom: -8,
            width: 1,
            bgcolor: tokens.color.border.subtle,
          }}
        />
      )}
    </Box>
  );
}

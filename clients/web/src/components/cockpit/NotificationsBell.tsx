"use client";

// NotificationsBell — P1 notification surface in the cockpit nav.
//
// Renders an icon button + badge next to the wallet chip. Click opens
// a Popover with the recent notifications list and a footer link to
// /settings/notifications. A GraphQL subscription is opened while the
// component is mounted AND the caller is authenticated; new
// notifications prepend to the cached list and surface one toast at a
// time via the existing swal toast pipeline.
//
// Backend contract:
//   query Notifications, query NotificationPreferences,
//   mutation MarkNotificationRead, mutation MarkAllNotificationsRead,
//   subscription NotificationStream
// (see /Users/.../core/orchestrator/.../graph/schema/notifications.graphql
//  once it lands).
//
// Graceful degradation: while the schema is being merged on the
// orchestrator the query returns a "Cannot query field" error. We
// detect that via `isSchemaMissing` and render nothing instead of a
// red banner — the bell becomes visible the moment the resolvers
// ship.

import { useApolloClient, useMutation, useQuery, useSubscription } from "@apollo/client";
import {
  AccountBalanceWalletOutlined,
  DeviceUnknownOutlined,
  InsightsOutlined,
  NotificationsOutlined,
  ShieldOutlined,
} from "@mui/icons-material";
import type { SvgIconComponent } from "@mui/icons-material";
import {
  Badge,
  Box,
  Button,
  Divider,
  IconButton,
  Popover,
  Stack,
  Tooltip,
  Typography,
} from "@mui/material";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useAuth } from "../../lib/auth";
import {
  isSchemaMissing,
  MarkAllNotificationsReadDocument,
  MarkNotificationReadDocument,
  NotificationsDocument,
  NotificationStreamDocument,
  type MarkAllNotificationsReadMutation,
  type MarkAllNotificationsReadVariables,
  type MarkNotificationReadMutation,
  type MarkNotificationReadVariables,
  type Notification,
  type NotificationsQuery,
  type NotificationsQueryVariables,
  type NotificationStreamSubscription,
  type NotificationStreamSubscriptionVariables,
} from "../../lib/gql/notifications.types";
import { relativeTime } from "../../lib/relativeTime";
import { toast as swalToast } from "../../lib/swal";
import { tokens } from "../../theme";

// Severity → color. The locked palette only allows violet/coral/red/
// mint families; we use violet for info, coral for warning, red for
// critical. No lime, no raw hex.
function severityColor(severity: string): string {
  switch (severity.toLowerCase()) {
    case "critical":
    case "error":
    case "danger":
      return tokens.color.accent.danger;
    case "warning":
    case "warn":
      return tokens.color.accent.coral;
    case "success":
      return tokens.color.brand.mint;
    default:
      return tokens.color.accent.violet;
  }
}

// Map the orchestrator notification `kind` string to a small icon +
// accent color so operators can scan the new lifecycle events at a
// glance. Existing kinds fall through to `null`, preserving the
// severity-dot treatment from P1.
//
// All colors are sourced from `tokens.color.*`. The icon is rendered
// only when present; when absent the row keeps the original severity
// dot. This keeps the row layout compact and avoids regressing list
// density for the original event vocabulary.
interface KindGlyph {
  Icon: SvgIconComponent;
  color: string;
}
function kindGlyph(kind: string): KindGlyph | null {
  switch (kind) {
    case "mfa_enabled":
      return { Icon: ShieldOutlined, color: tokens.color.brand.mint };
    case "new_device_login":
      return { Icon: DeviceUnknownOutlined, color: tokens.color.accent.coral };
    case "low_balance":
      return {
        Icon: AccountBalanceWalletOutlined,
        color: tokens.color.accent.coral,
      };
    case "weekly_digest":
      return { Icon: InsightsOutlined, color: tokens.color.accent.violet };
    default:
      return null;
  }
}

function severityIcon(severity: string): "info" | "warning" | "error" | "success" {
  switch (severity.toLowerCase()) {
    case "critical":
    case "error":
    case "danger":
      return "error";
    case "warning":
    case "warn":
      return "warning";
    case "success":
      return "success";
    default:
      return "info";
  }
}

export function NotificationsBell() {
  const { authenticated, user } = useAuth();
  const apollo = useApolloClient();
  const router = useRouter();

  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  const open = Boolean(anchor);

  // One toast at a time: hold a tiny FIFO queue and drain it via swal.
  // swalToast resolves when the toast closes, so we serialise the
  // queue by chaining the promises.
  const toastQueueRef = useRef<Notification[]>([]);
  const draining = useRef(false);
  const drainQueue = useCallback(async () => {
    if (draining.current) return;
    draining.current = true;
    try {
      while (toastQueueRef.current.length > 0) {
        const next = toastQueueRef.current.shift();
        if (!next) break;
        await swalToast(next.title, severityIcon(next.severity), {
          timer: 4000,
          position: "bottom-end",
        });
      }
    } finally {
      draining.current = false;
    }
  }, []);

  // ---- query ------------------------------------------------------
  const listQuery = useQuery<NotificationsQuery, NotificationsQueryVariables>(
    NotificationsDocument,
    {
      skip: !authenticated,
      fetchPolicy: "cache-and-network",
      nextFetchPolicy: "cache-first",
      variables: { unreadOnly: false },
      errorPolicy: "all",
    },
  );

  // Refetch on user change. Apollo's cache reset (driven by signIn /
  // signOut in auth.tsx) already drops stale entities; this hook
  // ensures the bell rehydrates against the new identity.
  useEffect(() => {
    if (!authenticated) return;
    void listQuery.refetch().catch(() => undefined);
    // listQuery.refetch is stable
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [user?.id, authenticated]);

  const schemaMissing = useMemo(
    () => isSchemaMissing(listQuery.error),
    [listQuery.error],
  );

  const notifications: Notification[] = useMemo(
    () => listQuery.data?.notifications ?? [],
    [listQuery.data],
  );
  const unreadCount = listQuery.data?.unreadNotificationCount ?? 0;

  // ---- subscription -----------------------------------------------
  // Apollo's useSubscription hook tears the underlying graphql-ws
  // subscription down on unmount + when `skip` flips true, so signOut
  // (which sets `authenticated` to false) cleanly closes the WS
  // operation. No manual cleanup needed.
  useSubscription<
    NotificationStreamSubscription,
    NotificationStreamSubscriptionVariables
  >(NotificationStreamDocument, {
    skip: !authenticated || schemaMissing,
    onData: ({ data, client }) => {
      const next = data.data?.notificationStream;
      if (!next) return;
      // The Query.notifications typePolicy in apollo.tsx merges
      // incoming refs against existing ones (key = Notification:<id>)
      // and re-sorts by createdAt desc, so we can write the single
      // new entry and let the cache dedupe + order. The unread count
      // is a sibling scalar on the same query — we bump it only when
      // (a) the existing query is in cache (otherwise the upcoming
      // initial fetch will deliver the authoritative count) and (b)
      // the notification we just received is genuinely new (not a
      // late echo of an entry already in the list).
      client.cache.updateQuery<NotificationsQuery, NotificationsQueryVariables>(
        { query: NotificationsDocument, variables: { unreadOnly: false } },
        (existing) => {
          if (!existing) return existing ?? undefined;
          const alreadyHave = existing.notifications.some((n) => n.id === next.id);
          const wasUnread = !next.readAt && !alreadyHave;
          return {
            __typename: "Query",
            // Prepend; merge() will dedupe + sort.
            notifications: alreadyHave
              ? existing.notifications
              : [next, ...existing.notifications],
            unreadNotificationCount:
              existing.unreadNotificationCount + (wasUnread ? 1 : 0),
          };
        },
      );
      // Queue toast.
      toastQueueRef.current.push(next);
      void drainQueue();
    },
    onError: () => {
      // Subscription failures (auth drop, WS bounce) are recoverable;
      // graphql-ws will retry per its retryAttempts config.
    },
  });

  // ---- mutations --------------------------------------------------
  const [markRead] = useMutation<
    MarkNotificationReadMutation,
    MarkNotificationReadVariables
  >(MarkNotificationReadDocument);
  const [markAllRead, markAllReadState] = useMutation<
    MarkAllNotificationsReadMutation,
    MarkAllNotificationsReadVariables
  >(MarkAllNotificationsReadDocument);

  const handleMarkRead = useCallback(
    async (id: string) => {
      try {
        await markRead({
          variables: { id },
          optimisticResponse: {
            __typename: "Mutation",
            markNotificationRead: {
              __typename: "Notification",
              id,
              readAt: new Date().toISOString(),
            },
          },
          update: (cache) => {
            cache.updateQuery<NotificationsQuery, NotificationsQueryVariables>(
              { query: NotificationsDocument, variables: { unreadOnly: false } },
              (existing) => {
                if (!existing) return existing ?? undefined;
                const list = existing.notifications.map((n) =>
                  n.id === id && !n.readAt
                    ? { ...n, readAt: new Date().toISOString() }
                    : n,
                );
                const wasUnread = existing.notifications.some(
                  (n) => n.id === id && !n.readAt,
                );
                return {
                  ...existing,
                  notifications: list,
                  unreadNotificationCount: Math.max(
                    0,
                    existing.unreadNotificationCount - (wasUnread ? 1 : 0),
                  ),
                };
              },
            );
          },
        });
      } catch {
        // Swallow — Apollo error link surfaces auth failures and the
        // local optimistic update will be reverted automatically.
      }
    },
    [markRead],
  );

  const handleMarkAllRead = useCallback(async () => {
    try {
      await markAllRead({
        update: (cache) => {
          cache.updateQuery<NotificationsQuery, NotificationsQueryVariables>(
            { query: NotificationsDocument, variables: { unreadOnly: false } },
            (existing) => {
              if (!existing) return existing ?? undefined;
              const now = new Date().toISOString();
              return {
                ...existing,
                notifications: existing.notifications.map((n) =>
                  n.readAt ? n : { ...n, readAt: now },
                ),
                unreadNotificationCount: 0,
              };
            },
          );
        },
      });
    } catch {
      // Same rationale as handleMarkRead.
    }
  }, [markAllRead]);

  const handleRowClick = useCallback(
    (n: Notification) => {
      setAnchor(null);
      if (!n.readAt) void handleMarkRead(n.id);
      if (n.link) router.push(n.link);
    },
    [handleMarkRead, router],
  );

  // ---- render -----------------------------------------------------
  // Don't render at all for anonymous visitors or while the schema is
  // mid-deploy. The latter prevents a flash of empty chrome.
  if (!authenticated) return null;
  if (schemaMissing) return null;
  // Suppress while user record is hydrating to avoid a hover flash
  // before the auth context settles. Apollo's cache-and-network
  // policy still keeps the data fresh once the user resolves.
  if (!user) return null;

  // Use `void` so the import isn't elided by TS — apollo is reserved
  // for future fetchPolicy switches without churning the import list.
  void apollo;

  return (
    <>
      <Tooltip title="Notifications" arrow>
        <IconButton
          size="small"
          onClick={(e) => setAnchor(e.currentTarget)}
          aria-label="Notifications"
          sx={{
            color: tokens.color.text.secondary,
            border: `1px solid ${tokens.color.border.subtle}`,
            borderRadius: 1,
            width: 34,
            height: 34,
            transition: `color ${tokens.motion.fast} ${tokens.motion.snap}, background-color ${tokens.motion.fast} ${tokens.motion.snap}`,
            "&:hover": {
              bgcolor: tokens.color.bg.surfaceHover,
              color: tokens.color.text.primary,
            },
          }}
        >
          <Badge
            badgeContent={unreadCount}
            invisible={unreadCount === 0}
            max={99}
            sx={{
              "& .MuiBadge-badge": {
                bgcolor: tokens.color.accent.coral,
                color: tokens.color.text.primary,
                fontWeight: 800,
                fontSize: 10,
                minWidth: 16,
                height: 16,
              },
            }}
          >
            <NotificationsOutlined sx={{ fontSize: 18 }} />
          </Badge>
        </IconButton>
      </Tooltip>
      <Popover
        anchorEl={anchor}
        open={open}
        onClose={() => setAnchor(null)}
        anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
        transformOrigin={{ vertical: "top", horizontal: "right" }}
        slotProps={{
          paper: {
            sx: {
              mt: 1,
              width: 380,
              maxHeight: 480,
              bgcolor: tokens.color.bg.surface,
              border: `1px solid ${tokens.color.border.subtle}`,
              borderRadius: "8px",
              overflow: "hidden",
              display: "flex",
              flexDirection: "column",
            },
          },
        }}
      >
        <Stack
          direction="row"
          alignItems="center"
          justifyContent="space-between"
          sx={{
            px: 2,
            py: 1.25,
            borderBottom: `1px solid ${tokens.color.border.subtle}`,
          }}
        >
          <Typography
            sx={{
              fontSize: 14,
              fontWeight: 800,
              color: tokens.color.text.primary,
              letterSpacing: 0.1,
            }}
          >
            Notifications
          </Typography>
          <Button
            size="small"
            variant="text"
            disabled={unreadCount === 0 || markAllReadState.loading}
            onClick={() => void handleMarkAllRead()}
            sx={{
              minWidth: 0,
              px: 1,
              py: 0.25,
              fontSize: 12,
              fontWeight: 700,
              color:
                unreadCount === 0
                  ? tokens.color.text.muted
                  : tokens.color.accent.violet,
              "&:hover": { bgcolor: tokens.color.bg.surfaceHover },
            }}
          >
            Mark all read
          </Button>
        </Stack>
        <Box sx={{ overflowY: "auto", flex: 1, minHeight: 0 }}>
          {notifications.length === 0 ? (
            <Box
              sx={{
                px: 2,
                py: 6,
                textAlign: "center",
                color: tokens.color.text.muted,
              }}
            >
              <Typography sx={{ fontSize: 13, color: tokens.color.text.muted }}>
                No notifications yet
              </Typography>
            </Box>
          ) : (
            notifications.map((n) => {
              const unread = !n.readAt;
              const dotColor = severityColor(n.severity);
              const glyph = kindGlyph(n.kind);
              const bg = unread
                ? `${tokens.color.accent.violet}11`
                : "transparent";
              const rowProps = n.link
                ? {
                    component: Link,
                    href: n.link,
                    onClick: (e: React.MouseEvent) => {
                      // Let Next.js Link handle navigation; we still
                      // want to mark read + close the popover.
                      if (!n.readAt) void handleMarkRead(n.id);
                      setAnchor(null);
                      // Don't preventDefault — Link does the routing.
                      void e;
                    },
                  }
                : {
                    component: "button" as const,
                    onClick: () => handleRowClick(n),
                  };
              return (
                <Box
                  key={n.id}
                  {...rowProps}
                  sx={{
                    display: "flex",
                    alignItems: "flex-start",
                    gap: 1.25,
                    width: "100%",
                    textAlign: "left",
                    border: "none",
                    cursor: "pointer",
                    px: 2,
                    py: 1.25,
                    bgcolor: bg,
                    borderBottom: `1px solid ${tokens.color.border.subtle}`,
                    color: tokens.color.text.primary,
                    textDecoration: "none",
                    transition: `background-color ${tokens.motion.fast} ${tokens.motion.snap}`,
                    "&:hover": { bgcolor: tokens.color.bg.surfaceHover },
                    "&:last-of-type": { borderBottom: "none" },
                  }}
                >
                  {glyph ? (
                    <Box
                      aria-hidden
                      sx={{
                        mt: 0.25,
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "center",
                        flexShrink: 0,
                        color: glyph.color,
                      }}
                    >
                      <glyph.Icon sx={{ fontSize: 16 }} />
                    </Box>
                  ) : (
                    <Box
                      aria-hidden
                      sx={{
                        width: 4,
                        height: 4,
                        mt: 1,
                        borderRadius: tokens.radius.pill,
                        bgcolor: dotColor,
                        flexShrink: 0,
                      }}
                    />
                  )}
                  <Box sx={{ minWidth: 0, flex: 1 }}>
                    <Typography
                      sx={{
                        fontSize: 13,
                        fontWeight: 700,
                        color: tokens.color.text.primary,
                        lineHeight: 1.3,
                      }}
                    >
                      {n.title}
                    </Typography>
                    {n.body ? (
                      <Typography
                        sx={{
                          mt: 0.25,
                          fontSize: 12,
                          color: tokens.color.text.secondary,
                          lineHeight: 1.4,
                          overflow: "hidden",
                          display: "-webkit-box",
                          WebkitLineClamp: 2,
                          WebkitBoxOrient: "vertical",
                        }}
                      >
                        {n.body}
                      </Typography>
                    ) : null}
                    <Typography
                      sx={{
                        mt: 0.5,
                        fontSize: 11,
                        color: tokens.color.text.muted,
                        fontFamily: tokens.font.mono,
                        letterSpacing: 0.3,
                      }}
                    >
                      {relativeTime(n.createdAt)}
                    </Typography>
                  </Box>
                </Box>
              );
            })
          )}
        </Box>
        <Divider sx={{ borderColor: tokens.color.border.subtle }} />
        <Box
          sx={{
            px: 2,
            py: 1,
            display: "flex",
            justifyContent: "center",
            borderTop: `1px solid ${tokens.color.border.subtle}`,
          }}
        >
          <Button
            component={Link}
            href="/settings/notifications"
            size="small"
            variant="text"
            onClick={() => setAnchor(null)}
            sx={{
              fontSize: 12,
              fontWeight: 700,
              color: tokens.color.text.secondary,
              "&:hover": {
                color: tokens.color.text.primary,
                bgcolor: tokens.color.bg.surfaceHover,
              },
            }}
          >
            Open settings
          </Button>
        </Box>
      </Popover>
    </>
  );
}

"use client";

// /settings/notifications — P1 notification preferences surface.
//
// Five topics × two channels (email / in-app) plus a master pauseAll
// switch. Receipt.email is locked-on at the UI layer (the orchestrator
// rejects opting out of accounting events regardless).
//
// Data flow:
//   useQuery(NotificationPreferences) → seed local state
//   user toggles → diff against the seed
//   Save → updateNotificationPreferences(diff) → toast confirmation
//
// Auth: wrapped in <RequireAuth /> so unauthenticated visitors land on
// /login with the existing redirect pattern. The bell on the cockpit
// nav uses the same convention.

import { useMutation, useQuery } from "@apollo/client";
import { NotificationsNoneRounded } from "@mui/icons-material";
import {
  Box,
  Button,
  Card,
  Skeleton,
  Stack,
  Switch,
  Tooltip,
  Typography,
} from "@mui/material";
import { useEffect, useMemo, useState } from "react";
import { ErrorPanel, PageHeader } from "../../../src/components/cockpit";
import { RequireAuth } from "../../../src/lib/auth";
import {
  isSchemaMissing,
  NotificationPreferencesDocument,
  UpdateNotificationPreferencesDocument,
  type NotificationPreferences,
  type NotificationPreferencesInput,
  type NotificationPreferencesQuery,
  type UpdateNotificationPreferencesMutation,
  type UpdateNotificationPreferencesVariables,
} from "../../../src/lib/gql/notifications.types";
import { toast as swalToast } from "../../../src/lib/swal";
import { tokens } from "../../../src/theme";

type TopicKey =
  | "onRunComplete"
  | "onGateFailed"
  | "onDeployDone"
  | "onBudgetWarning"
  | "onReceipt";

interface TopicDef {
  key: TopicKey;
  label: string;
  description: string;
  // Receipt locks its email channel on for accounting (law 1 hygiene —
  // every paid execution lands in an auditable ledger entry).
  lockEmail?: boolean;
  lockEmailReason?: string;
}

const TOPICS: TopicDef[] = [
  {
    key: "onRunComplete",
    label: "Run complete",
    description: "A paid execution finished — succeed or fail.",
  },
  {
    key: "onGateFailed",
    label: "Gate failed",
    description: "A finisher gate blocked the run; review the verdict.",
  },
  {
    key: "onDeployDone",
    label: "Deploy done",
    description: "A deploy artifact landed or its approval is awaiting you.",
  },
  {
    key: "onBudgetWarning",
    label: "Budget warning",
    description: "Wallet balance crossed the low-balance floor.",
  },
  {
    key: "onReceipt",
    label: "Receipt",
    description: "Wallet top-up + execution settlement receipts for accounting.",
    lockEmail: true,
    lockEmailReason:
      "Receipts are sent for accounting and cannot be disabled.",
  },
];

type DraftPrefs = {
  pauseAll: boolean;
  weeklyDigest: boolean;
  onRunComplete: { email: boolean; inApp: boolean };
  onGateFailed: { email: boolean; inApp: boolean };
  onDeployDone: { email: boolean; inApp: boolean };
  onBudgetWarning: { email: boolean; inApp: boolean };
  onReceipt: { email: boolean; inApp: boolean };
};

function fromServer(p: NotificationPreferences): DraftPrefs {
  return {
    pauseAll: p.pauseAll,
    // Opt-in default. The orchestrator may not yet return the field
    // (rolling deploy) — coerce to a boolean so the switch never
    // renders against `undefined`.
    weeklyDigest: Boolean(p.weeklyDigest),
    onRunComplete: { email: p.onRunComplete.email, inApp: p.onRunComplete.inApp },
    onGateFailed: { email: p.onGateFailed.email, inApp: p.onGateFailed.inApp },
    onDeployDone: { email: p.onDeployDone.email, inApp: p.onDeployDone.inApp },
    onBudgetWarning: {
      email: p.onBudgetWarning.email,
      inApp: p.onBudgetWarning.inApp,
    },
    onReceipt: { email: p.onReceipt.email, inApp: p.onReceipt.inApp },
  };
}

function equalDraft(a: DraftPrefs, b: DraftPrefs): boolean {
  return (
    a.pauseAll === b.pauseAll &&
    a.weeklyDigest === b.weeklyDigest &&
    (Object.keys(a) as Array<keyof DraftPrefs>).every((k) => {
      if (k === "pauseAll" || k === "weeklyDigest") return true;
      const av = a[k] as { email: boolean; inApp: boolean };
      const bv = b[k] as { email: boolean; inApp: boolean };
      return av.email === bv.email && av.inApp === bv.inApp;
    })
  );
}

export default function NotificationsSettingsPage() {
  return (
    <RequireAuth>
      <NotificationsSettingsView />
    </RequireAuth>
  );
}

function NotificationsSettingsView() {
  const { data, loading, error, refetch } = useQuery<NotificationPreferencesQuery>(
    NotificationPreferencesDocument,
    { fetchPolicy: "cache-and-network", errorPolicy: "all" },
  );
  const [updatePrefs, updateState] = useMutation<
    UpdateNotificationPreferencesMutation,
    UpdateNotificationPreferencesVariables
  >(UpdateNotificationPreferencesDocument);

  const seed = useMemo<DraftPrefs | null>(() => {
    if (!data?.notificationPreferences) return null;
    return fromServer(data.notificationPreferences);
  }, [data]);

  const [draft, setDraft] = useState<DraftPrefs | null>(null);

  useEffect(() => {
    if (seed) setDraft(seed);
  }, [seed]);

  const dirty = useMemo(() => {
    if (!draft || !seed) return false;
    return !equalDraft(draft, seed);
  }, [draft, seed]);

  const schemaMissing = isSchemaMissing(error);

  const onSave = async () => {
    if (!draft || !seed) return;
    const input: NotificationPreferencesInput = {
      pauseAll: draft.pauseAll,
      weeklyDigest: draft.weeklyDigest,
      onRunComplete: draft.onRunComplete,
      onGateFailed: draft.onGateFailed,
      onDeployDone: draft.onDeployDone,
      onBudgetWarning: draft.onBudgetWarning,
      // Receipt.email always sent as true — UI lock mirrors the
      // server-side accounting rule.
      onReceipt: { email: true, inApp: draft.onReceipt.inApp },
    };
    try {
      await updatePrefs({ variables: { input } });
      void swalToast("Preferences saved", "success");
    } catch {
      void swalToast("Could not save preferences", "error");
    }
  };

  return (
    <Box>
      <PageHeader
        eyebrow="Settings"
        title="Notifications"
        description="Choose how Ironflyer reaches you when builds, gates, deploys, or wallet events happen."
      />

      {schemaMissing ? (
        <Card sx={{ p: { xs: 2.5, md: 3 } }}>
          <Typography
            sx={{
              fontSize: 14,
              fontWeight: 700,
              color: tokens.color.text.primary,
            }}
          >
            Notification preferences are rolling out
          </Typography>
          <Typography
            sx={{
              mt: 1,
              fontSize: 13,
              color: tokens.color.text.secondary,
              maxWidth: 560,
            }}
          >
            The orchestrator is wiring up the preferences resolver. The
            cockpit surface lights up automatically once it ships — no
            redeploy on the client side needed.
          </Typography>
        </Card>
      ) : error && !data ? (
        <ErrorPanel
          error={error}
          title="Could not load preferences"
          onRetry={() => void refetch()}
        />
      ) : loading && !draft ? (
        <Stack spacing={2}>
          <Skeleton variant="rounded" height={88} />
          <Skeleton variant="rounded" height={420} />
        </Stack>
      ) : draft ? (
        <Stack spacing={{ xs: 2.5, md: 3 }}>
          <Card sx={{ p: { xs: 2.5, md: 3 } }}>
            <Stack
              direction={{ xs: "column", sm: "row" }}
              spacing={2}
              alignItems={{ sm: "center" }}
              justifyContent="space-between"
            >
              <Box sx={{ minWidth: 0, flex: 1 }}>
                <Typography
                  sx={{
                    fontSize: 16,
                    fontWeight: 800,
                    color: tokens.color.text.primary,
                  }}
                >
                  Pause all notifications
                </Typography>
                <Typography
                  sx={{
                    mt: 0.5,
                    fontSize: 13,
                    color: tokens.color.text.secondary,
                    maxWidth: 520,
                  }}
                >
                  Master kill-switch — nothing reaches you across any channel
                  until you turn this off. Wallet receipts still land in your
                  ledger.
                </Typography>
              </Box>
              <Switch
                checked={draft.pauseAll}
                onChange={(_, next) => setDraft({ ...draft, pauseAll: next })}
                color="primary"
              />
            </Stack>
          </Card>

          <Card sx={{ p: { xs: 2.5, md: 3 } }}>
            <Stack
              direction={{ xs: "column", sm: "row" }}
              spacing={2}
              alignItems={{ sm: "center" }}
              justifyContent="space-between"
            >
              <Box sx={{ minWidth: 0, flex: 1 }}>
                <Typography
                  sx={{
                    fontSize: 16,
                    fontWeight: 800,
                    color: tokens.color.text.primary,
                  }}
                >
                  Weekly digest
                </Typography>
                <Typography
                  sx={{
                    mt: 0.5,
                    fontSize: 13,
                    color: tokens.color.text.secondary,
                    maxWidth: 520,
                  }}
                >
                  A weekly summary of runs, gates, deploys, and spend —
                  delivered Sunday mornings.
                </Typography>
                {draft.weeklyDigest && draft.pauseAll ? (
                  <Typography
                    sx={{
                      mt: 1,
                      fontSize: 12,
                      color: tokens.color.text.muted,
                      fontStyle: "italic",
                    }}
                  >
                    Currently paused. Resume notifications to receive the digest.
                  </Typography>
                ) : null}
              </Box>
              <Switch
                checked={draft.weeklyDigest}
                onChange={(_, next) =>
                  setDraft({ ...draft, weeklyDigest: next })
                }
                color="primary"
                inputProps={{ "aria-label": "Send me a weekly digest" }}
              />
            </Stack>
          </Card>

          <Card
            sx={{
              p: 0,
              opacity: draft.pauseAll ? 0.55 : 1,
              transition: `opacity ${tokens.motion.fast} ${tokens.motion.snap}`,
            }}
          >
            <Box
              sx={{
                px: { xs: 2, md: 3 },
                py: 1.5,
                borderBottom: `1px solid ${tokens.color.border.subtle}`,
                display: "grid",
                gridTemplateColumns: "1fr 84px 84px",
                gap: 1.5,
                alignItems: "center",
              }}
            >
              <Typography
                variant="overline"
                sx={{
                  color: tokens.color.text.muted,
                  letterSpacing: 1.2,
                }}
              >
                Topic
              </Typography>
              <Typography
                variant="overline"
                sx={{
                  color: tokens.color.text.muted,
                  letterSpacing: 1.2,
                  textAlign: "center",
                }}
              >
                Email
              </Typography>
              <Typography
                variant="overline"
                sx={{
                  color: tokens.color.text.muted,
                  letterSpacing: 1.2,
                  textAlign: "center",
                }}
              >
                In-app
              </Typography>
            </Box>
            {TOPICS.map((topic, i) => {
              const row = draft[topic.key];
              const last = i === TOPICS.length - 1;
              return (
                <Box
                  key={topic.key}
                  sx={{
                    px: { xs: 2, md: 3 },
                    py: 2,
                    display: "grid",
                    gridTemplateColumns: "1fr 84px 84px",
                    gap: 1.5,
                    alignItems: "center",
                    borderBottom: last
                      ? "none"
                      : `1px solid ${tokens.color.border.subtle}`,
                  }}
                >
                  <Box sx={{ minWidth: 0 }}>
                    <Typography
                      sx={{
                        fontSize: 14,
                        fontWeight: 700,
                        color: tokens.color.text.primary,
                      }}
                    >
                      {topic.label}
                    </Typography>
                    <Typography
                      sx={{
                        mt: 0.25,
                        fontSize: 12.5,
                        color: tokens.color.text.secondary,
                        lineHeight: 1.4,
                      }}
                    >
                      {topic.description}
                    </Typography>
                  </Box>
                  <Box sx={{ display: "flex", justifyContent: "center" }}>
                    {topic.lockEmail ? (
                      <Tooltip title={topic.lockEmailReason ?? ""} arrow>
                        <span>
                          <Switch
                            checked
                            disabled
                            color="primary"
                            inputProps={{
                              "aria-label": `${topic.label} email (locked)`,
                            }}
                          />
                        </span>
                      </Tooltip>
                    ) : (
                      <Switch
                        checked={row.email}
                        disabled={draft.pauseAll}
                        onChange={(_, next) =>
                          setDraft({
                            ...draft,
                            [topic.key]: { ...row, email: next },
                          })
                        }
                        color="primary"
                        inputProps={{
                          "aria-label": `${topic.label} email`,
                        }}
                      />
                    )}
                  </Box>
                  <Box sx={{ display: "flex", justifyContent: "center" }}>
                    <Switch
                      checked={row.inApp}
                      disabled={draft.pauseAll}
                      onChange={(_, next) =>
                        setDraft({
                          ...draft,
                          [topic.key]: { ...row, inApp: next },
                        })
                      }
                      color="primary"
                      inputProps={{
                        "aria-label": `${topic.label} in-app`,
                      }}
                    />
                  </Box>
                </Box>
              );
            })}
          </Card>

          <Stack direction="row" spacing={1.25} sx={{ flexWrap: "wrap" }}>
            <Button
              variant="contained"
              color="primary"
              size="medium"
              startIcon={<NotificationsNoneRounded sx={{ fontSize: 16 }} />}
              disabled={!dirty || updateState.loading}
              onClick={() => void onSave()}
            >
              {updateState.loading ? "Saving…" : "Save"}
            </Button>
          </Stack>
        </Stack>
      ) : null}
    </Box>
  );
}

"use client";

// /settings — tabbed account surface.
//
// Sections:
//   - Account       (identity + plan + sign out)
//   - API keys      (placeholder — orchestrator does not yet expose
//                    per-user API keys via GraphQL)
//   - Integrations  (placeholder — GitHub / Stripe / OpenAI / Anthropic)
//   - Notifications (placeholder — email + in-product prefs)
//   - Billing       (wallet balance via useWalletBalance + last-30d
//                    spend rollup via useExecutionsQuery)
//   - Danger        (delete account placeholder)
//
// Live data:
//   - CurrentUser (id, email, name, plan, orgId, telemetryOptOut,
//     emailVerifiedAt, createdAt) — auth.graphql
//   - Wallet via useWalletBalance — wallet.graphql
//   - Executions for last-month spend — executions.graphql
//
// Self-service mutations for password / email / telemetry / api keys /
// integrations / notifications / account deletion are stubbed on the
// orchestrator; the relevant UI is rendered but disabled with a TODO
// pointer so the wiring is one resolver away.

import {
  AccountBalanceWalletOutlined,
  CheckCircleOutline,
  DeleteOutline,
  EmailOutlined,
  FingerprintOutlined,
  KeyOutlined,
  LinkRounded,
  LogoutRounded,
  NotificationsNoneRounded,
  ReceiptLongOutlined,
  ScheduleOutlined,
  ShieldOutlined,
  WorkspacesOutlined,
} from "@mui/icons-material";
import {
  Box,
  Button,
  Card,
  Chip,
  Divider,
  FormControlLabel,
  Stack,
  Switch,
  Tab,
  Tabs,
  Tooltip,
  Typography,
} from "@mui/material";
import Link from "next/link";
import { useMemo, useState, type ReactNode } from "react";
import {
  EmptyState,
  ErrorPanel,
  LoadingPanel,
  PageHeader,
  StatusBadge,
} from "../../src/components/cockpit";
import { RequireAuth, useAuth } from "../../src/lib/auth";
import { useExecutionsQuery } from "../../src/lib/gql/__generated__";
import { useWalletBalance } from "../../src/lib/hooks";
import { formatDate, formatDateTime, formatMoney } from "../../src/lib/format";
import { tokens } from "../../src/theme";

const TAB_KEYS = [
  "account",
  "keys",
  "integrations",
  "notifications",
  "billing",
  "danger",
] as const;
type TabKey = (typeof TAB_KEYS)[number];

const TABS: { key: TabKey; label: string }[] = [
  { key: "account", label: "Account" },
  { key: "keys", label: "API keys" },
  { key: "integrations", label: "Integrations" },
  { key: "notifications", label: "Notifications" },
  { key: "billing", label: "Billing" },
  { key: "danger", label: "Danger" },
];

export default function SettingsPage() {
  return (
    <RequireAuth>
      <SettingsView />
    </RequireAuth>
  );
}

function SettingsView() {
  const { user } = useAuth();
  const [tab, setTab] = useState<TabKey>("account");

  if (!user) return null;

  return (
    <Box>
      <PageHeader
        eyebrow="Account"
        title="Profile & settings"
        description="Identity, API keys, integrations, notification preferences, and billing — every surface that controls how Ironflyer behaves on your behalf."
      />

      <Tabs
        value={tab}
        onChange={(_, next: TabKey) => setTab(next)}
        variant="scrollable"
        scrollButtons="auto"
        allowScrollButtonsMobile
        sx={{
          mb: { xs: 2.5, md: 3 },
          borderBottom: `1px solid ${tokens.color.border.subtle}`,
          minHeight: 42,
          "& .MuiTab-root": {
            minHeight: 42,
            fontWeight: 700,
            letterSpacing: 0.2,
            textTransform: "none",
            color: tokens.color.text.secondary,
            "&.Mui-selected": { color: tokens.color.text.primary },
          },
        }}
      >
        {TABS.map((t) => (
          <Tab key={t.key} value={t.key} label={t.label} />
        ))}
      </Tabs>

      {tab === "account" && <AccountPanel />}
      {tab === "keys" && <ApiKeysPanel />}
      {tab === "integrations" && <IntegrationsPanel />}
      {tab === "notifications" && <NotificationsPanel />}
      {tab === "billing" && <BillingPanel />}
      {tab === "danger" && <DangerPanel />}
    </Box>
  );
}

function AccountPanel() {
  const { user, signOut } = useAuth();
  if (!user) return null;

  const emailVerified = !!user.emailVerifiedAt;
  const initials = (user.name || user.email).slice(0, 1).toUpperCase();

  return (
    <Stack spacing={{ xs: 2.5, md: 3 }}>
      <Card sx={{ p: { xs: 2.5, md: 3 } }}>
        <Stack
          direction={{ xs: "column", sm: "row" }}
          spacing={2.5}
          alignItems={{ sm: "center" }}
          sx={{ mb: 3 }}
        >
          <Box
            aria-hidden
            sx={{
              width: 64,
              height: 64,
              borderRadius: tokens.radius.pill,
              display: "grid",
              placeItems: "center",
              fontWeight: 800,
              fontSize: 26,
              color: tokens.color.text.primary,
              background: `linear-gradient(135deg, ${tokens.color.accent.violet} 0%, ${tokens.color.accent.purple} 100%)`,
              border: `1px solid ${tokens.color.border.subtle}`,
              flexShrink: 0,
            }}
          >
            {initials}
          </Box>
          <Box sx={{ minWidth: 0, flex: 1 }}>
            <Typography
              sx={{
                fontSize: 22,
                fontWeight: 800,
                color: tokens.color.text.primary,
                lineHeight: 1.2,
              }}
            >
              {user.name || "Unnamed operator"}
            </Typography>
            <Typography
              sx={{
                mt: 0.5,
                fontFamily: tokens.font.mono,
                fontSize: 13,
                color: tokens.color.text.secondary,
                wordBreak: "break-all",
              }}
            >
              {user.email}
            </Typography>
            <Stack
              direction="row"
              spacing={1}
              sx={{ mt: 1.5, flexWrap: "wrap", gap: 1 }}
            >
              <Chip
                size="small"
                icon={<WorkspacesOutlined sx={{ fontSize: 14 }} />}
                label={`Plan · ${user.plan || "starter"}`}
                sx={{
                  bgcolor: `${tokens.color.accent.violet}1a`,
                  color: tokens.color.accent.violet,
                  border: `1px solid ${tokens.color.border.subtle}`,
                  fontFamily: tokens.font.mono,
                  fontWeight: 700,
                  letterSpacing: 0.6,
                }}
              />
              <Chip
                size="small"
                icon={<CheckCircleOutline sx={{ fontSize: 14 }} />}
                label={emailVerified ? "Email verified" : "Email unverified"}
                sx={{
                  bgcolor: emailVerified
                    ? `${tokens.color.accent.success}1a`
                    : `${tokens.color.accent.warning}1f`,
                  color: emailVerified
                    ? tokens.color.accent.success
                    : tokens.color.accent.warning,
                  border: `1px solid ${tokens.color.border.subtle}`,
                  fontFamily: tokens.font.mono,
                  fontWeight: 700,
                  letterSpacing: 0.6,
                }}
              />
              {user.orgId && (
                <Chip
                  size="small"
                  label={`Org · ${user.orgId}`}
                  sx={{
                    bgcolor: tokens.color.bg.surfaceRaised,
                    color: tokens.color.text.secondary,
                    border: `1px solid ${tokens.color.border.subtle}`,
                    fontFamily: tokens.font.mono,
                  }}
                />
              )}
            </Stack>
          </Box>
        </Stack>

        <Divider sx={{ my: 2.5 }} />

        <Box
          sx={{
            display: "grid",
            gap: 2.5,
            gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr" },
          }}
        >
          <FieldRow
            icon={<FingerprintOutlined sx={{ fontSize: 18 }} />}
            label="User ID"
            value={user.id}
            mono
          />
          <FieldRow
            icon={<EmailOutlined sx={{ fontSize: 18 }} />}
            label="Email"
            value={user.email}
            mono
          />
          <FieldRow
            icon={<ScheduleOutlined sx={{ fontSize: 18 }} />}
            label="Member since"
            value={formatDate(user.createdAt)}
          />
          <FieldRow
            icon={<ShieldOutlined sx={{ fontSize: 18 }} />}
            label="Telemetry"
            value={
              user.telemetryOptOut
                ? "Opted out"
                : "Sharing anonymous usage"
            }
          />
          {emailVerified && (
            <FieldRow
              icon={<CheckCircleOutline sx={{ fontSize: 18 }} />}
              label="Verified at"
              value={formatDateTime(user.emailVerifiedAt)}
            />
          )}
        </Box>
      </Card>

      <Card sx={{ p: { xs: 2.5, md: 3 } }}>
        <SectionEyebrow label="Security" />
        <Typography
          sx={{
            mt: 0.5,
            fontSize: 18,
            fontWeight: 700,
            color: tokens.color.text.primary,
          }}
        >
          Password & sessions
        </Typography>
        <Typography
          sx={{
            mt: 1,
            fontSize: 13.5,
            color: tokens.color.text.secondary,
            maxWidth: 560,
          }}
        >
          Self-service password change, session revocation, and email change
          land alongside the password-reset resolver. Sign out clears the
          local JWT and Apollo cache.
        </Typography>
        <Stack
          direction="row"
          spacing={1.25}
          sx={{ mt: 2.5, flexWrap: "wrap", gap: 1 }}
        >
          <Tooltip title="Password reset endpoint is pending" arrow>
            <span>
              <Button
                size="small"
                variant="outlined"
                startIcon={<KeyOutlined sx={{ fontSize: 16 }} />}
                disabled
              >
                Change password
              </Button>
            </span>
          </Tooltip>
          <Tooltip title="Email change resolver is pending" arrow>
            <span>
              <Button
                size="small"
                variant="outlined"
                startIcon={<EmailOutlined sx={{ fontSize: 16 }} />}
                disabled
              >
                Change email
              </Button>
            </span>
          </Tooltip>
          <Button
            size="small"
            variant="contained"
            color="primary"
            startIcon={<LogoutRounded sx={{ fontSize: 16 }} />}
            onClick={() => void signOut()}
          >
            Sign out
          </Button>
        </Stack>
      </Card>
    </Stack>
  );
}

function ApiKeysPanel() {
  // TODO(settings-wire): swap to useApiKeysQuery + create/revoke
  // mutations when the orchestrator exposes per-user API keys.
  return (
    <Card sx={{ p: { xs: 2.5, md: 3 } }}>
      <SectionEyebrow label="API keys" />
      <Typography
        sx={{
          mt: 0.5,
          fontSize: 18,
          fontWeight: 700,
          color: tokens.color.text.primary,
        }}
      >
        Programmatic access
      </Typography>
      <Typography
        sx={{
          mt: 1,
          fontSize: 13.5,
          color: tokens.color.text.secondary,
          maxWidth: 560,
        }}
      >
        Issue keys, scope them per-project, and revoke compromised tokens.
        Every key inherits the wallet contract — paid executions still
        reserve and debit through the ledger.
      </Typography>
      <Box sx={{ mt: 2.5 }}>
        <EmptyState
          title="API keys arrive when the orchestrator exposes them"
          body="The auth schema currently issues short-lived JWTs only. Long-lived API keys land alongside the upcoming personal-access-token resolver."
        />
      </Box>
    </Card>
  );
}

interface IntegrationRow {
  provider: string;
  description: string;
  status: "connected" | "available" | "pending";
}

const INTEGRATIONS: IntegrationRow[] = [
  {
    provider: "GitHub",
    description: "Push generated artifacts and open review PRs.",
    status: "available",
  },
  {
    provider: "Stripe",
    description: "Wallet top-up provider for prepaid credits.",
    status: "connected",
  },
  {
    provider: "OpenAI",
    description: "Bring-your-own-key for the OpenAI provider tier.",
    status: "pending",
  },
  {
    provider: "Anthropic",
    description: "Bring-your-own-key for Claude reasoning tiers.",
    status: "pending",
  },
];

function IntegrationsPanel() {
  // TODO(settings-wire): swap to a real integrations query +
  // connect / disconnect mutations once user-managed provider keys
  // land in the auth schema.
  return (
    <Card sx={{ p: { xs: 2.5, md: 3 } }}>
      <SectionEyebrow label="Integrations" />
      <Typography
        sx={{
          mt: 0.5,
          fontSize: 18,
          fontWeight: 700,
          color: tokens.color.text.primary,
        }}
      >
        External providers
      </Typography>
      <Typography
        sx={{
          mt: 1,
          fontSize: 13.5,
          color: tokens.color.text.secondary,
          maxWidth: 560,
        }}
      >
        Connect external accounts so Ironflyer can deliver finished work
        and route paid model calls through your own provider quotas where
        applicable.
      </Typography>
      <Stack
        sx={{ mt: 2.5 }}
        divider={
          <Box sx={{ borderTop: `1px solid ${tokens.color.border.subtle}` }} />
        }
      >
        {INTEGRATIONS.map((row) => (
          <Stack
            key={row.provider}
            direction={{ xs: "column", sm: "row" }}
            alignItems={{ sm: "center" }}
            justifyContent="space-between"
            spacing={1.5}
            sx={{ py: 2 }}
          >
            <Box sx={{ minWidth: 0, flex: 1 }}>
              <Stack direction="row" alignItems="center" spacing={1}>
                <Typography
                  sx={{
                    fontWeight: 700,
                    fontSize: 14.5,
                    color: tokens.color.text.primary,
                  }}
                >
                  {row.provider}
                </Typography>
                <StatusBadge status={row.status} />
              </Stack>
              <Typography
                sx={{
                  mt: 0.5,
                  fontSize: 13,
                  color: tokens.color.text.secondary,
                }}
              >
                {row.description}
              </Typography>
            </Box>
            <Tooltip
              title={
                row.status === "connected"
                  ? "Managed automatically — disconnect lands with the integrations resolver."
                  : "Connect surface arrives with the integrations resolver."
              }
              arrow
            >
              <span>
                <Button
                  size="small"
                  variant="outlined"
                  startIcon={<LinkRounded sx={{ fontSize: 15 }} />}
                  disabled
                >
                  {row.status === "connected" ? "Manage" : "Connect"}
                </Button>
              </span>
            </Tooltip>
          </Stack>
        ))}
      </Stack>
    </Card>
  );
}

function NotificationsPanel() {
  // TODO(settings-wire): wire to a notification-preferences mutation
  // once the orchestrator exposes it. Local state lets the controls
  // demo the future behavior without persisting.
  const [emailExecutionDone, setEmailExecutionDone] = useState(true);
  const [emailWalletLow, setEmailWalletLow] = useState(true);
  const [emailDeployApproval, setEmailDeployApproval] = useState(true);
  const [productGateAlerts, setProductGateAlerts] = useState(true);

  return (
    <Card sx={{ p: { xs: 2.5, md: 3 } }}>
      <SectionEyebrow label="Notifications" />
      <Typography
        sx={{
          mt: 0.5,
          fontSize: 18,
          fontWeight: 700,
          color: tokens.color.text.primary,
        }}
      >
        How we reach you
      </Typography>
      <Typography
        sx={{
          mt: 1,
          fontSize: 13.5,
          color: tokens.color.text.secondary,
          maxWidth: 560,
        }}
      >
        Choose which finisher events trigger an email and which surface
        in-product only. Wallet alerts default on because they protect law 1.
      </Typography>

      <Stack spacing={1.5} sx={{ mt: 2.5 }}>
        <PrefRow
          label="Email me when a paid execution finishes"
          value={emailExecutionDone}
          onChange={setEmailExecutionDone}
        />
        <PrefRow
          label="Email me when my wallet drops below the low-balance floor"
          value={emailWalletLow}
          onChange={setEmailWalletLow}
        />
        <PrefRow
          label="Email me when a deploy needs my approval"
          value={emailDeployApproval}
          onChange={setEmailDeployApproval}
        />
        <PrefRow
          label="In-product: surface gate verdicts in the cockpit notifications tray"
          value={productGateAlerts}
          onChange={setProductGateAlerts}
        />
      </Stack>

      <Stack direction="row" spacing={1.25} sx={{ mt: 3, flexWrap: "wrap" }}>
        <Tooltip
          title="Preferences resolver is pending — selections aren't persisted yet."
          arrow
        >
          <span>
            <Button
              variant="contained"
              color="primary"
              size="small"
              startIcon={<NotificationsNoneRounded sx={{ fontSize: 16 }} />}
              disabled
            >
              Save preferences
            </Button>
          </span>
        </Tooltip>
      </Stack>
    </Card>
  );
}

function PrefRow({
  label,
  value,
  onChange,
}: {
  label: string;
  value: boolean;
  onChange: (next: boolean) => void;
}) {
  return (
    <FormControlLabel
      control={
        <Switch
          checked={value}
          onChange={(_, next) => onChange(next)}
          color="primary"
        />
      }
      label={
        <Typography
          sx={{ fontSize: 13.5, color: tokens.color.text.primary }}
        >
          {label}
        </Typography>
      }
      sx={{
        m: 0,
        py: 0.5,
        justifyContent: "space-between",
        width: "100%",
        "& .MuiFormControlLabel-label": { flex: 1 },
      }}
      labelPlacement="start"
    />
  );
}

const LAST_30D_MS = 30 * 24 * 60 * 60 * 1000;

function BillingPanel() {
  const wallet = useWalletBalance();
  const execQ = useExecutionsQuery({
    variables: { limit: 100, offset: 0 },
    fetchPolicy: "cache-and-network",
  });

  const last30Spend = useMemo(() => {
    const list = execQ.data?.executions ?? [];
    const since = Date.now() - LAST_30D_MS;
    let total = 0;
    for (const e of list) {
      const ts = new Date(
        e.endedAt ?? e.startedAt ?? e.admittedAt ?? e.createdAt,
      ).getTime();
      if (!Number.isFinite(ts) || ts < since) continue;
      total +=
        (e.providerCostUSD || 0) +
        (e.sandboxCostUSD || 0) +
        (e.storageCostUSD || 0) +
        (e.deploymentCostUSD || 0);
    }
    return total;
  }, [execQ.data]);

  return (
    <Stack spacing={{ xs: 2.5, md: 3 }}>
      <Card sx={{ p: { xs: 2.5, md: 3 } }}>
        <Stack direction="row" alignItems="center" spacing={1}>
          <AccountBalanceWalletOutlined
            sx={{ fontSize: 18, color: tokens.color.accent.violet }}
          />
          <SectionEyebrow label="Wallet" />
        </Stack>
        {wallet.loading && wallet.totalUSD === 0 && wallet.availableUSD === 0 ? (
          <LoadingPanel minHeight={120} label="Loading wallet" />
        ) : wallet.error ? (
          <ErrorPanel
            error={wallet.error}
            title="Could not load wallet"
            onRetry={() => void wallet.refetch()}
          />
        ) : (
          <>
            <Typography
              sx={{
                mt: 1,
                fontFamily: tokens.font.mono,
                fontSize: 32,
                fontWeight: 700,
                color: wallet.lowBalance
                  ? tokens.color.accent.warning
                  : tokens.color.text.primary,
                letterSpacing: -0.5,
              }}
            >
              {formatMoney(wallet.availableUSD)}
            </Typography>
            <Typography
              sx={{ fontSize: 12, color: tokens.color.text.muted }}
            >
              available · {formatMoney(wallet.reservedUSD)} held
            </Typography>
            <Stack direction="row" spacing={1} sx={{ mt: 2.5 }}>
              <Button
                component={Link}
                href="/wallet"
                size="small"
                variant="outlined"
                fullWidth
              >
                Wallet detail
              </Button>
              <Button
                component={Link}
                href="/wallet/topup"
                size="small"
                variant="contained"
                color="primary"
                fullWidth
              >
                Top up
              </Button>
            </Stack>
          </>
        )}
      </Card>

      <Card sx={{ p: { xs: 2.5, md: 3 } }}>
        <Stack direction="row" alignItems="center" spacing={1}>
          <ReceiptLongOutlined
            sx={{ fontSize: 18, color: tokens.color.text.secondary }}
          />
          <SectionEyebrow label="Spend · last 30 days" />
        </Stack>
        {execQ.loading && !execQ.data ? (
          <LoadingPanel minHeight={100} label="Loading recent executions" />
        ) : execQ.error && !execQ.data ? (
          <ErrorPanel
            error={execQ.error}
            title="Could not load executions"
            onRetry={() => void execQ.refetch()}
          />
        ) : (
          <>
            <Typography
              sx={{
                mt: 1,
                fontFamily: tokens.font.mono,
                fontSize: 28,
                fontWeight: 700,
                color: tokens.color.text.primary,
                letterSpacing: -0.4,
              }}
            >
              {formatMoney(last30Spend)}
            </Typography>
            <Typography
              sx={{
                mt: 0.25,
                fontSize: 12,
                color: tokens.color.text.muted,
              }}
            >
              provider + sandbox + storage + deploy cost across paid runs
            </Typography>
            <Stack direction="row" spacing={1} sx={{ mt: 2 }}>
              <Button
                component={Link}
                href="/executions"
                size="small"
                variant="outlined"
              >
                See executions
              </Button>
            </Stack>
          </>
        )}
      </Card>
    </Stack>
  );
}

function DangerPanel() {
  // TODO(settings-wire): wire a deleteAccount mutation once the
  // orchestrator exposes it. Until then we show the affordance
  // disabled with a "contact support" hint.
  return (
    <Card
      sx={{
        p: { xs: 2.5, md: 3 },
        borderColor: `${tokens.color.accent.danger}55`,
      }}
    >
      <SectionEyebrow label="Danger" color={tokens.color.accent.danger} />
      <Typography
        sx={{
          mt: 0.5,
          fontSize: 18,
          fontWeight: 700,
          color: tokens.color.text.primary,
        }}
      >
        Delete account
      </Typography>
      <Typography
        sx={{
          mt: 1,
          fontSize: 13.5,
          color: tokens.color.text.secondary,
          maxWidth: 560,
        }}
      >
        Removes your user record, revokes outstanding sessions, and forfeits
        any wallet balance that has not been refunded. This is irreversible
        and is intentionally gated on a human request while the resolver is
        being hardened.
      </Typography>
      <Stack
        direction="row"
        spacing={1.25}
        sx={{ mt: 2.5, flexWrap: "wrap", gap: 1 }}
      >
        <Tooltip
          title="Delete-account resolver is pending — contact support to escalate."
          arrow
        >
          <span>
            <Button
              size="small"
              variant="outlined"
              color="error"
              startIcon={<DeleteOutline sx={{ fontSize: 16 }} />}
              disabled
            >
              Delete account
            </Button>
          </span>
        </Tooltip>
      </Stack>
    </Card>
  );
}

function SectionEyebrow({ label, color }: { label: string; color?: string }) {
  return (
    <Typography
      variant="overline"
      sx={{
        color: color ?? tokens.color.accent.violet,
        letterSpacing: 1.2,
        fontWeight: 700,
      }}
    >
      {label}
    </Typography>
  );
}

function FieldRow({
  icon,
  label,
  value,
  mono,
}: {
  icon: ReactNode;
  label: string;
  value: string;
  mono?: boolean;
}) {
  return (
    <Stack direction="row" spacing={1.5} alignItems="flex-start">
      <Box sx={{ color: tokens.color.text.muted, mt: 0.25, flexShrink: 0 }}>
        {icon}
      </Box>
      <Box sx={{ minWidth: 0, flex: 1 }}>
        <Typography
          variant="overline"
          sx={{
            color: tokens.color.text.muted,
            letterSpacing: 1.1,
            fontSize: 10.5,
          }}
        >
          {label}
        </Typography>
        <Typography
          sx={{
            mt: 0.25,
            color: tokens.color.text.primary,
            fontFamily: mono ? tokens.font.mono : undefined,
            fontSize: mono ? 13 : 14,
            wordBreak: "break-all",
          }}
        >
          {value || "—"}
        </Typography>
      </Box>
    </Stack>
  );
}

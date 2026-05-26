"use client";

// DeviceCloudPanel — the Pro-tier real-device surface. Renders the
// catalogue of BrowserStack App Live + AWS Device Farm devices, lets
// the operator pick one, opens an interactive session in a phone-frame
// iframe, and ticks a live billable-minute counter so the cost stays
// visible while the session runs.
//
// Free-tier callers see the upgrade card instead of the picker. The
// resolver enforces the same boundary server-side so this is a UX
// optimisation, not the security boundary.

import { gql, useMutation, useQuery } from "@apollo/client";
import {
  Box,
  CircularProgress,
  Stack,
  Typography,
} from "@mui/material";
import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { tokens } from "../../../theme";

const LIST_DEVICES = gql`
  query DeviceCloudDevices($platform: String!) {
    deviceCloudDevices(platform: $platform) {
      id
      provider
      platform
      osVersion
      model
      manufacturer
      real
    }
  }
`;

const START_SESSION = gql`
  mutation DeviceCloudStartSession($input: DeviceCloudStartInput!) {
    deviceCloudStartSession(input: $input) {
      id
      provider
      deviceId
      status
      sessionUrl
      startedAt
      expiresAt
      billableMinutesUsed
    }
  }
`;

const END_SESSION = gql`
  mutation DeviceCloudEndSession($sessionId: String!, $provider: String!) {
    deviceCloudEndSession(sessionId: $sessionId, provider: $provider)
  }
`;

type DeviceCloudProvider = "browserstack" | "aws-device-farm";

interface DeviceCloudDevice {
  id: string;
  provider: DeviceCloudProvider;
  platform: string;
  osVersion: string;
  model: string;
  manufacturer?: string | null;
  real: boolean;
}

interface DeviceCloudSession {
  id: string;
  provider: DeviceCloudProvider;
  deviceId: string;
  status: string;
  sessionUrl?: string | null;
  startedAt: string;
  expiresAt: string;
  billableMinutesUsed: number;
}

interface DeviceCloudPanelProps {
  projectId: string;
  workspaceId: string;
  appUrl?: string;
  isPro?: boolean;
}

export function DeviceCloudPanel({
  projectId,
  workspaceId,
  appUrl,
  isPro = false,
}: DeviceCloudPanelProps) {
  const [provider, setProvider] = useState<DeviceCloudProvider>("browserstack");
  const [platform, setPlatform] = useState<"" | "android" | "ios">("");
  const [selectedDevice, setSelectedDevice] = useState<string>("");
  const [session, setSession] = useState<DeviceCloudSession | null>(null);

  const { data, loading } = useQuery<{ deviceCloudDevices: DeviceCloudDevice[] }>(
    LIST_DEVICES,
    { variables: { platform }, skip: !isPro },
  );

  const [startSession, { loading: starting }] = useMutation<{
    deviceCloudStartSession: DeviceCloudSession;
  }>(START_SESSION);
  const [endSession] = useMutation(END_SESSION);

  const devices = useMemo(() => {
    const all = data?.deviceCloudDevices ?? [];
    return all.filter((d) => d.provider === provider);
  }, [data, provider]);

  const groupedDevices = useMemo(() => {
    const groups: Record<string, DeviceCloudDevice[]> = {};
    for (const d of devices) {
      const key = d.platform || "other";
      groups[key] = groups[key] || [];
      groups[key].push(d);
    }
    return groups;
  }, [devices]);

  // Live billable-minutes ticker — updates every second while a session
  // is running so the operator sees spend accrue without polling the
  // server. The server's reconciliation entry on EndSession is the
  // source of truth; this is a UX mirror.
  const [elapsedMin, setElapsedMin] = useState(0);
  useEffect(() => {
    if (!session) {
      setElapsedMin(0);
      return;
    }
    const started = new Date(session.startedAt).getTime();
    const tick = () => {
      const ms = Date.now() - started;
      setElapsedMin(ms / 60000);
    };
    tick();
    const id = window.setInterval(tick, 1000);
    return () => window.clearInterval(id);
  }, [session]);

  if (!isPro) {
    return <ProTierGate />;
  }

  const handleStart = async () => {
    if (!selectedDevice) return;
    const result = await startSession({
      variables: {
        input: {
          projectId,
          workspaceId,
          provider,
          deviceId: selectedDevice,
          appUrl: appUrl ?? "",
          sessionLengthMinutes: 30,
        },
      },
    });
    if (result.data?.deviceCloudStartSession) {
      setSession(result.data.deviceCloudStartSession);
    }
  };

  const handleEnd = async () => {
    if (!session) return;
    await endSession({
      variables: { sessionId: session.id, provider: session.provider },
    });
    setSession(null);
  };

  return (
    <Stack spacing={2} sx={{ p: 2 }}>
      <Stack direction="row" spacing={1} alignItems="center">
        <ProviderChip
          provider="browserstack"
          active={provider === "browserstack"}
          onClick={() => setProvider("browserstack")}
        />
        <ProviderChip
          provider="aws-device-farm"
          active={provider === "aws-device-farm"}
          onClick={() => setProvider("aws-device-farm")}
        />
        <Box sx={{ flex: 1 }} />
        <PlatformChip
          label="All"
          active={platform === ""}
          onClick={() => setPlatform("")}
        />
        <PlatformChip
          label="Android"
          active={platform === "android"}
          onClick={() => setPlatform("android")}
        />
        <PlatformChip
          label="iOS"
          active={platform === "ios"}
          onClick={() => setPlatform("ios")}
        />
      </Stack>

      {session ? (
        <SessionRunningView
          session={session}
          elapsedMin={elapsedMin}
          onEnd={handleEnd}
        />
      ) : (
        <Stack direction={{ xs: "column", md: "row" }} spacing={2}>
          <Box sx={{ flex: 1, minWidth: 0 }}>
            {loading ? (
              <Stack
                alignItems="center"
                justifyContent="center"
                sx={{ minHeight: 120 }}
              >
                <CircularProgress size={20} />
              </Stack>
            ) : (
              <DeviceList
                groups={groupedDevices}
                selectedId={selectedDevice}
                onSelect={setSelectedDevice}
              />
            )}
          </Box>
          <Stack spacing={1.5} sx={{ width: { xs: "100%", md: 220 } }}>
            <PrimaryCTA
              disabled={!selectedDevice || starting}
              loading={starting}
              onClick={handleStart}
            />
            <Typography
              sx={{
                color: tokens.color.text.muted,
                fontSize: 11.5,
                lineHeight: 1.55,
              }}
            >
              Sessions run up to 30 minutes. Billable minutes accrue in
              real time and reconcile against the wallet ledger.
            </Typography>
          </Stack>
        </Stack>
      )}
    </Stack>
  );
}

function ProviderChip({
  provider,
  active,
  onClick,
}: {
  provider: DeviceCloudProvider;
  active: boolean;
  onClick: () => void;
}) {
  const label =
    provider === "browserstack" ? "BrowserStack" : "AWS Device Farm";
  return (
    <Box
      component="button"
      type="button"
      onClick={onClick}
      sx={{
        background: active ? tokens.color.accent.violet : "transparent",
        border: `1px solid ${
          active ? tokens.color.accent.violet : tokens.color.border.subtle
        }`,
        borderRadius: 1,
        color: active ? tokens.color.text.inverse : tokens.color.text.primary,
        cursor: "pointer",
        fontSize: 12.5,
        fontWeight: 600,
        px: 1.25,
        py: 0.6,
      }}
    >
      {label}
    </Box>
  );
}

function PlatformChip({
  label,
  active,
  onClick,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <Box
      component="button"
      type="button"
      onClick={onClick}
      sx={{
        background: "transparent",
        border: `1px solid ${
          active ? tokens.color.accent.violet : tokens.color.border.subtle
        }`,
        borderRadius: 1,
        color: active
          ? tokens.color.accent.violet
          : tokens.color.text.secondary,
        cursor: "pointer",
        fontSize: 11.5,
        fontWeight: 600,
        px: 1,
        py: 0.4,
      }}
    >
      {label}
    </Box>
  );
}

function DeviceList({
  groups,
  selectedId,
  onSelect,
}: {
  groups: Record<string, DeviceCloudDevice[]>;
  selectedId: string;
  onSelect: (id: string) => void;
}) {
  const keys = Object.keys(groups);
  if (keys.length === 0) {
    return (
      <Typography
        sx={{
          color: tokens.color.text.muted,
          fontFamily: tokens.font.mono,
          fontSize: 12,
          py: 2,
          textAlign: "center",
        }}
      >
        No devices available — check provider credentials.
      </Typography>
    );
  }
  return (
    <Stack spacing={1.5}>
      {keys.map((k) => (
        <Stack key={k} spacing={0.5}>
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 10.5,
              letterSpacing: 1.2,
              textTransform: "uppercase",
            }}
          >
            {k}
          </Typography>
          <Stack spacing={0.25}>
            {groups[k].map((d) => (
              <Box
                key={d.id}
                component="button"
                type="button"
                onClick={() => onSelect(d.id)}
                sx={{
                  background:
                    selectedId === d.id
                      ? `${tokens.color.accent.violet}1f`
                      : "transparent",
                  border: `1px solid ${
                    selectedId === d.id
                      ? tokens.color.accent.violet
                      : tokens.color.border.subtle
                  }`,
                  borderRadius: 1,
                  color: tokens.color.text.primary,
                  cursor: "pointer",
                  display: "flex",
                  fontSize: 12.5,
                  justifyContent: "space-between",
                  px: 1.25,
                  py: 0.6,
                  textAlign: "left",
                }}
              >
                <Box>
                  <Typography sx={{ fontSize: 12.5, fontWeight: 600 }}>
                    {d.manufacturer ? `${d.manufacturer} ${d.model}` : d.model}
                  </Typography>
                  <Typography
                    sx={{
                      color: tokens.color.text.muted,
                      fontFamily: tokens.font.mono,
                      fontSize: 10.5,
                    }}
                  >
                    {d.osVersion} · {d.real ? "real device" : "emulator"}
                  </Typography>
                </Box>
              </Box>
            ))}
          </Stack>
        </Stack>
      ))}
    </Stack>
  );
}

function PrimaryCTA({
  disabled,
  loading,
  onClick,
}: {
  disabled: boolean;
  loading: boolean;
  onClick: () => void;
}) {
  return (
    <Box
      component="button"
      type="button"
      disabled={disabled}
      onClick={onClick}
      sx={{
        background: disabled
          ? tokens.color.bg.inset
          : tokens.color.accent.violet,
        border: "none",
        borderRadius: 1,
        color: disabled
          ? tokens.color.text.muted
          : tokens.color.text.inverse,
        cursor: disabled ? "not-allowed" : "pointer",
        fontSize: 13,
        fontWeight: 700,
        opacity: loading ? 0.6 : 1,
        py: 1,
      }}
    >
      {loading ? "Starting…" : "Start session"}
    </Box>
  );
}

function SessionRunningView({
  session,
  elapsedMin,
  onEnd,
}: {
  session: DeviceCloudSession;
  elapsedMin: number;
  onEnd: () => void;
}) {
  return (
    <Stack direction={{ xs: "column", md: "row" }} spacing={2}>
      <Box
        sx={{
          alignItems: "center",
          aspectRatio: "9 / 16",
          bgcolor: tokens.color.brand.graphite,
          border: `1px solid ${tokens.color.border.subtle}`,
          borderRadius: `${tokens.radius.xl}px`,
          display: "flex",
          justifyContent: "center",
          maxHeight: "60vh",
          overflow: "hidden",
          width: "min(320px, 100%)",
        }}
      >
        {session.sessionUrl ? (
          <Box
            component="iframe"
            src={session.sessionUrl}
            title="Real-device session"
            sandbox="allow-scripts allow-same-origin"
            sx={{ border: "none", height: "100%", width: "100%" }}
          />
        ) : (
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 11,
              letterSpacing: 1.2,
              textTransform: "uppercase",
            }}
          >
            Connecting to device…
          </Typography>
        )}
      </Box>
      <Stack spacing={1.25} sx={{ flex: 1, minWidth: 0 }}>
        <Typography
          sx={{
            color: tokens.color.text.muted,
            fontFamily: tokens.font.mono,
            fontSize: 10.5,
            letterSpacing: 1.2,
            textTransform: "uppercase",
          }}
        >
          Live session · {session.provider}
        </Typography>
        <Typography sx={{ fontSize: 14, fontWeight: 700 }}>
          {session.deviceId}
        </Typography>
        <Box
          sx={{
            bgcolor: tokens.color.bg.inset,
            border: `1px solid ${tokens.color.border.subtle}`,
            borderRadius: 1,
            display: "flex",
            justifyContent: "space-between",
            px: 1.25,
            py: 1,
          }}
        >
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 11,
              letterSpacing: 1.2,
              textTransform: "uppercase",
            }}
          >
            Billable minutes
          </Typography>
          <Typography
            sx={{
              color: tokens.color.accent.violet,
              fontFamily: tokens.font.mono,
              fontSize: 14,
              fontWeight: 700,
            }}
          >
            {elapsedMin.toFixed(2)}
          </Typography>
        </Box>
        <Box
          component="button"
          type="button"
          onClick={onEnd}
          sx={{
            background: "transparent",
            border: `1px solid ${tokens.color.border.strong}`,
            borderRadius: 1,
            color: tokens.color.text.primary,
            cursor: "pointer",
            fontSize: 12.5,
            fontWeight: 700,
            py: 0.85,
          }}
        >
          End session
        </Box>
      </Stack>
    </Stack>
  );
}

function ProTierGate() {
  return (
    <Box
      sx={{
        bgcolor: tokens.color.bg.surface,
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: 1,
        m: 2,
        p: 2.5,
      }}
    >
      <Stack spacing={1}>
        <Typography
          sx={{
            color: tokens.color.text.primary,
            fontSize: 15,
            fontWeight: 700,
          }}
        >
          Real devices require the Pro tier
        </Typography>
        <Typography
          sx={{
            color: tokens.color.text.secondary,
            fontSize: 13,
            lineHeight: 1.55,
          }}
        >
          Ironflyer Pro plugs into BrowserStack App Live and AWS Device
          Farm to run your build on physical Pixel, Galaxy, and iPhone
          hardware. Sessions are wallet-metered per minute, with the
          ledger entry visible in the cost panel as soon as the device
          boots.
        </Typography>
        <Box
          component={Link}
          href="/pricing"
          sx={{
            alignSelf: "flex-start",
            bgcolor: "transparent",
            border: `1px solid ${tokens.color.accent.violet}`,
            borderRadius: 1,
            color: tokens.color.accent.violet,
            display: "inline-block",
            fontSize: 12.5,
            fontWeight: 700,
            mt: 0.5,
            px: 1.5,
            py: 0.75,
            textDecoration: "none",
            "&:hover": {
              backgroundColor: `${tokens.color.accent.violet}1f`,
            },
          }}
        >
          See pricing →
        </Box>
      </Stack>
    </Box>
  );
}

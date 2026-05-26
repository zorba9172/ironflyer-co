"use client";

// MobilePreviewPane — the operator's at-a-glance mirror of the mobile
// side of a project. Four internal tabs:
//
//   * Expo  — QR + LAN/tunnel URLs (only for expo / react-native-bare)
//   * Android emulator — phone-framed scrcpy stream
//   * iOS preview — Mac-pool simulator (gated by env flag)
//   * Builds — recent build history with progress + log tail
//
// Visualization-first: each tab lands on a visual mirror of state,
// never a raw text dump. The component itself is a thin shell; the
// per-tab cards live alongside in /studio/mobile/.

import { Box, Stack, Tab, Tabs, Typography } from "@mui/material";
import Link from "next/link";
import { useMemo, useState } from "react";
import { tokens } from "../../../theme";
import { AssetGeneratorPanel } from "./AssetGeneratorPanel";
import { DeviceCloudPanel } from "./DeviceCloudPanel";
import { ExpoQRCard } from "./ExpoQRCard";
import { EmulatorStreamView } from "./EmulatorStreamView";
import { MobileBuildHistory } from "./MobileBuildHistory";
import { useMobileSession } from "../../../lib/mobile/useMobileSession";
import type { MobileStackKind, MobileStackTarget } from "./MobileStackPicker";

const MAC_POOL_ENABLED = process.env.NEXT_PUBLIC_MAC_POOL_ENABLED === "1";

export interface MobilePreviewPaneProps {
  workspaceId: string;
  mobileKind: MobileStackKind;
  targets?: MobileStackTarget[];
  // projectId is required for the Assets tab — the generator resolver
  // owner-checks the project before rendering anything.
  projectId?: string;
}

type MobileTab =
  | "expo"
  | "android"
  | "ios"
  | "builds"
  | "assets"
  | "devices";

function defaultTab(
  mobileKind: MobileStackKind,
  targets: MobileStackTarget[],
): MobileTab {
  if (mobileKind === "android-native") return "android";
  if (mobileKind === "ios-native") return "ios";
  if (mobileKind === "expo" || mobileKind === "react-native-bare") return "expo";
  if (targets.includes("ios")) return "ios";
  if (targets.includes("android")) return "android";
  return "builds";
}

function PaneShell({
  title,
  subtitle,
  children,
}: {
  title: string;
  subtitle?: string;
  children: React.ReactNode;
}) {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5, p: 2 }}>
      <Stack spacing={0.25}>
        <Typography
          sx={{
            color: tokens.color.text.muted,
            fontFamily: tokens.font.mono,
            fontSize: 10.5,
            letterSpacing: 1.2,
            textTransform: "uppercase",
          }}
        >
          {title}
        </Typography>
        {subtitle ? (
          <Typography
            sx={{ color: tokens.color.text.secondary, fontSize: 13 }}
          >
            {subtitle}
          </Typography>
        ) : null}
      </Stack>
      {children}
    </Box>
  );
}

export function MobilePreviewPane({
  workspaceId,
  mobileKind,
  targets = [],
  projectId,
}: MobilePreviewPaneProps) {
  const { data, refetch } = useMobileSession(workspaceId);
  const initialTab = useMemo(
    () => defaultTab(mobileKind, targets),
    [mobileKind, targets],
  );
  const [tab, setTab] = useState<MobileTab>(initialTab);

  const expoAvailable =
    mobileKind === "expo" || mobileKind === "react-native-bare";
  const iosTarget = mobileKind === "ios-native" || targets.includes("ios");

  return (
    <Box
      sx={{
        bgcolor: tokens.color.bg.base,
        display: "flex",
        flex: 1,
        flexDirection: "column",
        height: "100%",
        minHeight: 0,
        overflow: "hidden",
      }}
    >
      <Tabs
        value={tab}
        onChange={(_, v: MobileTab) => setTab(v)}
        variant="scrollable"
        scrollButtons={false}
        sx={{
          bgcolor: tokens.color.bg.surface,
          borderBottom: `1px solid ${tokens.color.border.subtle}`,
          minHeight: 40,
          px: 1,
          "& .MuiTab-root": {
            color: tokens.color.text.secondary,
            fontSize: 12.5,
            fontWeight: 600,
            minHeight: 40,
            px: 1.5,
            textTransform: "none",
          },
          "& .Mui-selected": { color: tokens.color.text.primary },
          "& .MuiTabs-indicator": {
            backgroundColor: tokens.color.accent.violet,
            height: 2,
          },
        }}
      >
        {expoAvailable ? <Tab value="expo" label="Expo" /> : null}
        <Tab value="android" label="Android emulator" />
        {iosTarget ? <Tab value="ios" label="iOS preview" /> : null}
        <Tab value="builds" label="Builds" />
        {projectId ? <Tab value="assets" label="Assets" /> : null}
        {projectId ? <Tab value="devices" label="Real devices" /> : null}
      </Tabs>

      <Box sx={{ flex: 1, minHeight: 0, overflow: "auto" }}>
        {tab === "expo" && expoAvailable ? (
          <PaneShell
            title="Expo Metro"
            subtitle="Scan the QR with Expo Go to load the development build."
          >
            <ExpoQRCard
              payload={
                data.metro?.qrPayload ?? data.expo?.qrPayload ?? "exp://offline"
              }
              lanUrl={data.metro?.metroUrl ?? data.expo?.lanUrl ?? "—"}
              tunnelUrl={data.metro?.tunnelUrl ?? data.expo?.tunnelUrl}
              running={Boolean(data.metro?.running ?? data.expo?.running)}
              metroPort={data.metro?.metroPort}
              startedAt={data.metro?.startedAt}
              onRefresh={refetch}
            />
          </PaneShell>
        ) : null}

        {tab === "android" ? (
          <PaneShell
            title="Android emulator"
            subtitle="Streams a workspace-allocated AVD into the cockpit."
          >
            <Stack direction={{ xs: "column", md: "row" }} spacing={2}>
              <EmulatorStreamView
                workspaceId={workspaceId}
                running={Boolean(data.emulator?.running)}
                sessionUrl={data.emulator?.sessionUrl}
              />
              <Stack spacing={1} sx={{ flex: 1, minWidth: 0 }}>
                <Typography
                  sx={{
                    color: tokens.color.text.muted,
                    fontFamily: tokens.font.mono,
                    fontSize: 11,
                    letterSpacing: 1,
                    textTransform: "uppercase",
                  }}
                >
                  AVD
                </Typography>
                <AvdPicker />
              </Stack>
            </Stack>
          </PaneShell>
        ) : null}

        {tab === "ios" && iosTarget ? (
          <PaneShell title="iOS preview">
            {MAC_POOL_ENABLED ? (
              <IosSimulatorFrame sessionUrl={undefined} />
            ) : data.appetize ? (
              <AppetizeIframe embedUrl={data.appetize.embedUrl} />
            ) : data.latestIOSBuildId ? (
              <AppetizeUploadCTA buildId={data.latestIOSBuildId} />
            ) : (
              <ProTierGate />
            )}
          </PaneShell>
        ) : null}

        {tab === "builds" ? (
          <PaneShell
            title="Recent builds"
            subtitle="EAS Build and Linux build sandbox artifacts for this workspace."
          >
            <MobileBuildHistory builds={data.builds} />
          </PaneShell>
        ) : null}

        {tab === "assets" && projectId ? (
          <PaneShell
            title="Mobile assets"
            subtitle="Generate the full Android + iOS + Expo icon and splash bundle from a single square logo."
          >
            <AssetGeneratorPanel projectId={projectId} />
          </PaneShell>
        ) : null}

        {tab === "devices" && projectId ? (
          <PaneShell
            title="Real devices"
            subtitle="BrowserStack App Live and AWS Device Farm sessions on physical Pixel, Galaxy, and iPhone hardware."
          >
            <DeviceCloudPanel
              projectId={projectId}
              workspaceId={workspaceId}
              isPro={Boolean(MAC_POOL_ENABLED)}
            />
          </PaneShell>
        ) : null}
      </Box>
    </Box>
  );
}

function AvdPicker() {
  const [avd, setAvd] = useState("Pixel 8");
  const options = ["Pixel 8", "Pixel Tablet", "Wear OS"];
  return (
    <Stack spacing={1}>
      <Box
        component="select"
        value={avd}
        onChange={(e: React.ChangeEvent<HTMLSelectElement>) =>
          setAvd(e.target.value)
        }
        sx={{
          appearance: "none",
          bgcolor: tokens.color.bg.inset,
          border: `1px solid ${tokens.color.border.subtle}`,
          borderRadius: 1,
          color: tokens.color.text.primary,
          fontFamily: tokens.font.mono,
          fontSize: 12.5,
          outline: "none",
          px: 1.25,
          py: 0.75,
          "&:focus": { borderColor: tokens.color.accent.violet },
        }}
      >
        {options.map((o) => (
          <option key={o} value={o}>
            {o}
          </option>
        ))}
      </Box>
      <Stack direction="row" spacing={1}>
        <Box
          component="button"
          type="button"
          sx={{
            bgcolor: tokens.color.accent.success,
            border: "none",
            borderRadius: 1,
            color: tokens.color.text.inverse,
            cursor: "pointer",
            fontSize: 12.5,
            fontWeight: 700,
            px: 1.5,
            py: 0.75,
          }}
        >
          START
        </Box>
        <Box
          component="button"
          type="button"
          sx={{
            bgcolor: "transparent",
            border: `1px solid ${tokens.color.border.strong}`,
            borderRadius: 1,
            color: tokens.color.text.secondary,
            cursor: "pointer",
            fontSize: 12.5,
            fontWeight: 700,
            px: 1.5,
            py: 0.75,
          }}
        >
          STOP
        </Box>
      </Stack>
    </Stack>
  );
}

function IosSimulatorFrame({ sessionUrl }: { sessionUrl?: string }) {
  if (!sessionUrl) {
    return (
      <Box
        sx={{
          alignItems: "center",
          aspectRatio: "9 / 16",
          bgcolor: tokens.color.brand.graphite,
          border: `1px solid ${tokens.color.border.subtle}`,
          borderRadius: `${tokens.radius.xl}px`,
          color: tokens.color.text.muted,
          display: "flex",
          fontFamily: tokens.font.mono,
          fontSize: 11,
          justifyContent: "center",
          letterSpacing: 1.2,
          maxHeight: "60vh",
          mx: "auto",
          textTransform: "uppercase",
          width: "min(300px, 100%)",
        }}
      >
        Waiting for Mac-pool allocation…
      </Box>
    );
  }
  return (
    <Box
      component="iframe"
      src={sessionUrl}
      title="iOS simulator stream"
      sandbox="allow-scripts allow-same-origin"
      sx={{
        aspectRatio: "9 / 16",
        bgcolor: tokens.color.brand.graphite,
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: `${tokens.radius.xl}px`,
        display: "block",
        maxHeight: "60vh",
        mx: "auto",
        width: "min(300px, 100%)",
      }}
    />
  );
}

function ProTierGate() {
  return (
    <Box
      sx={{
        bgcolor: tokens.color.bg.surface,
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: 1,
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
          iOS preview requires the Pro tier
        </Typography>
        <Typography
          sx={{
            color: tokens.color.text.secondary,
            fontSize: 13,
            lineHeight: 1.55,
          }}
        >
          Ironflyer runs iOS simulators on a managed Mac pool. Upgrade to
          unlock simulator streaming, EAS iOS builds, and TestFlight
          uploads from the cockpit.
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

// AppetizeIframe renders the Free-tier iOS preview. Appetize.io runs
// the simulator in their cloud and exposes a sandboxed iframe; the
// `allow` directives are required for the simulator to fake device
// sensors (camera tilt, microphone, gyroscope).
function AppetizeIframe({ embedUrl }: { embedUrl: string }) {
  return (
    <Box
      component="iframe"
      src={embedUrl}
      title="Appetize iOS preview"
      allow="camera; microphone; gyroscope; accelerometer"
      sandbox="allow-scripts allow-same-origin"
      sx={{
        aspectRatio: "9 / 16",
        bgcolor: tokens.color.brand.graphite,
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: `${tokens.radius.xl}px`,
        display: "block",
        maxHeight: "60vh",
        mx: "auto",
        width: "min(300px, 100%)",
      }}
    />
  );
}

// AppetizeUploadCTA is rendered when an iOS build is available but
// hasn't been uploaded to Appetize yet. Clicking the button fires the
// appetizeUploadBuild mutation — wired here as a no-op placeholder
// until the GraphQL operation lands; the cockpit will swap this for a
// real useMutation hook once the schema regenerates.
function AppetizeUploadCTA({ buildId }: { buildId: string }) {
  return (
    <Box
      sx={{
        bgcolor: tokens.color.bg.surface,
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: 1,
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
          Preview this iOS build in the browser
        </Typography>
        <Typography
          sx={{
            color: tokens.color.text.secondary,
            fontSize: 13,
            lineHeight: 1.55,
          }}
        >
          Ironflyer can upload build {buildId} to Appetize.io for a
          browser-embedded iOS simulator. No Mac required. Free-tier
          minutes are metered per session and surface in the cost
          panel.
        </Typography>
        <Box
          component="button"
          type="button"
          data-build-id={buildId}
          sx={{
            alignSelf: "flex-start",
            bgcolor: tokens.color.accent.violet,
            border: "none",
            borderRadius: 1,
            color: tokens.color.text.inverse,
            cursor: "pointer",
            fontSize: 12.5,
            fontWeight: 700,
            mt: 0.5,
            px: 1.5,
            py: 0.75,
            "&:hover": {
              filter: "brightness(1.1)",
            },
          }}
        >
          Upload to Appetize preview
        </Box>
      </Stack>
    </Box>
  );
}

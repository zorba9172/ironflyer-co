// TODO(mobile-runtime): swap to GraphQL useQuery once core/runtime mobile HTTP routes are exposed via the orchestrator schema. For now this is mock data shaped like the real response.

"use client";

import { useMemo } from "react";

export type MobileBuildPlatform = "android" | "ios";
export type MobileBuildProfile = "development" | "preview" | "production";
export type MobileBuildStatus =
  | "queued"
  | "running"
  | "succeeded"
  | "failed"
  | "cancelled";

export interface MobileBuild {
  id: string;
  platform: MobileBuildPlatform;
  profile: MobileBuildProfile;
  status: MobileBuildStatus;
  startedAt: string;
  durationMs?: number;
  artifactUrl?: string;
  artifactSizeBytes?: number;
  logTail?: string;
}

export interface MobileExpoSession {
  running: boolean;
  lanUrl: string;
  tunnelUrl?: string;
  qrPayload: string;
}

export interface MobileEmulatorSession {
  running: boolean;
  sessionUrl?: string;
}

// MobileMetroSession mirrors the runtime's MetroSession shape (see
// core/runtime/internal/mobile/metro.go) so the cockpit can render the
// hot-reload state — including the public tunnel URL physical-device
// Expo Go connects to from outside the LAN.
export interface MobileMetroSession {
  running: boolean;
  metroUrl: string;
  metroPort?: number;
  tunnelUrl?: string;
  qrPayload?: string;
  startedAt?: string;
}

// AppetizeSession is the Free-tier iOS preview path: Appetize.io runs
// the simulator in their cloud and we iframe the embed URL. Null when
// no build has been uploaded yet — the pane keeps the Pro-tier card.
export interface AppetizeSession {
  publicKey: string;
  embedUrl: string;
  platform: "ios" | "android";
}

export interface MobileSession {
  expo: MobileExpoSession | null;
  emulator: MobileEmulatorSession | null;
  metro: MobileMetroSession | null;
  builds: MobileBuild[];
  // appetize is populated when a recent build has been uploaded to
  // Appetize. The iOS preview tab renders an iframe pointing at
  // appetize.embedUrl.
  appetize: AppetizeSession | null;
  // latestIOSBuildId is the most recent succeeded iOS build artifact.
  // When non-null and appetize is null, the iOS preview tab renders
  // an "Upload to Appetize" CTA wired to appetizeUploadBuild.
  latestIOSBuildId: string | null;
}

export interface UseMobileSessionResult {
  data: MobileSession;
  loading: boolean;
  error?: Error;
  refetch: () => void;
}

// Returned object is deliberately empty/inactive — the runtime hasn't
// been wired through GraphQL yet, so the cockpit should render its
// "not running" affordances rather than fake live state. The `refetch`
// noop matches the shape the cockpit's Refresh button will call once
// the GraphQL plumbing lands.
export function useMobileSession(_workspaceId: string): UseMobileSessionResult {
  const data = useMemo<MobileSession>(
    () => ({
      expo: null,
      emulator: null,
      metro: null,
      builds: [],
      appetize: null,
      latestIOSBuildId: null,
    }),
    [],
  );

  const refetch = () => {
    /* no-op until runtime mobile routes are wired through the orchestrator */
  };

  return { data, loading: false, refetch };
}

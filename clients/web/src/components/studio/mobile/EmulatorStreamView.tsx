"use client";

// EmulatorStreamView — phone-framed surface that hosts the live
// scrcpy + WebRTC stream of an Android emulator. The component owns
// the <video> element and the pointer-event → normalised-coordinate
// translation; the negotiation logic lives in useEmulatorWebRTC.

import {
  ArrowBackRounded,
  HomeRounded,
  ViewAgendaRounded,
} from "@mui/icons-material";
import { Box, Button, IconButton, Stack, Typography } from "@mui/material";
import { useCallback, useRef, useState } from "react";
import {
  useEmulatorWebRTC,
  type EmulatorInputEvent,
} from "../../../lib/mobile/useEmulatorWebRTC";
import { tokens } from "../../../theme";

// Curated Android keycodes the hardware-button row dispatches. Mirrors
// the allow-list enforced by the bridge service.
const KEYCODE_BACK = 4;
const KEYCODE_HOME = 3;
const KEYCODE_RECENT_APPS = 187;

export interface EmulatorStreamViewProps {
  workspaceId: string;
  running: boolean;
  sessionUrl?: string;
  onStart?: () => void;
}

export function EmulatorStreamView({
  workspaceId,
  running,
  sessionUrl,
  onStart,
}: EmulatorStreamViewProps) {
  const { videoRef, sendInput, connected, error } = useEmulatorWebRTC({
    sessionUrl,
    running,
  });
  const surfaceRef = useRef<HTMLDivElement | null>(null);
  // pointerStart caches the down-event so we can decide between
  // "tap" and "swipe" on pointerup.
  const pointerStart = useRef<{ x: number; y: number; t: number } | null>(null);
  const [interacting, setInteracting] = useState(false);

  const live = running && Boolean(sessionUrl);
  const showVideo = live && connected;

  const normaliseCoords = useCallback(
    (clientX: number, clientY: number) => {
      const el = surfaceRef.current;
      if (!el) return null;
      const rect = el.getBoundingClientRect();
      if (rect.width <= 0 || rect.height <= 0) return null;
      const x = Math.max(0, Math.min(1, (clientX - rect.left) / rect.width));
      const y = Math.max(0, Math.min(1, (clientY - rect.top) / rect.height));
      return { x, y };
    },
    [],
  );

  const dispatch = useCallback(
    (ev: EmulatorInputEvent) => {
      if (!live) return;
      sendInput(ev);
    },
    [live, sendInput],
  );

  const onPointerDown = useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      if (!live) return;
      const coords = normaliseCoords(event.clientX, event.clientY);
      if (!coords) return;
      pointerStart.current = { x: coords.x, y: coords.y, t: performance.now() };
      setInteracting(true);
      event.currentTarget.setPointerCapture(event.pointerId);
    },
    [live, normaliseCoords],
  );

  const onPointerUp = useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      if (!live) return;
      const start = pointerStart.current;
      pointerStart.current = null;
      setInteracting(false);
      try {
        event.currentTarget.releasePointerCapture(event.pointerId);
      } catch {
        /* pointer may have been canceled */
      }
      if (!start) return;
      const coords = normaliseCoords(event.clientX, event.clientY);
      if (!coords) return;
      const dx = coords.x - start.x;
      const dy = coords.y - start.y;
      const dist = Math.hypot(dx, dy);
      const duration = Math.max(80, Math.round(performance.now() - start.t));
      if (dist < 0.015) {
        dispatch({ type: "touch", x: coords.x, y: coords.y });
        return;
      }
      dispatch({
        type: "swipe",
        x: start.x,
        y: start.y,
        x2: coords.x,
        y2: coords.y,
        duration,
      });
    },
    [dispatch, live, normaliseCoords],
  );

  const onPointerCancel = useCallback(() => {
    pointerStart.current = null;
    setInteracting(false);
  }, []);

  return (
    <Stack spacing={1.25} sx={{ alignItems: "center" }}>
      <Box
        ref={surfaceRef}
        onPointerDown={onPointerDown}
        onPointerUp={onPointerUp}
        onPointerCancel={onPointerCancel}
        sx={{
          aspectRatio: "9 / 16",
          bgcolor: tokens.color.brand.graphite,
          border: `1px solid ${
            live ? tokens.color.accent.violet : tokens.color.border.subtle
          }`,
          borderRadius: `${tokens.radius.xl}px`,
          boxShadow: live
            ? `0 0 0 1px ${tokens.color.accent.violet}55, 0 24px 60px ${tokens.color.accent.purple}40`
            : tokens.shadow.md,
          cursor: live ? (interacting ? "grabbing" : "pointer") : "default",
          maxHeight: "60vh",
          overflow: "hidden",
          position: "relative",
          transition: "box-shadow 260ms ease, border-color 260ms ease",
          touchAction: "none",
          userSelect: "none",
          width: "min(300px, 100%)",
        }}
        aria-label="Android emulator phone frame"
        data-workspace-id={workspaceId}
      >
        {live ? (
          <Box
            component="video"
            ref={videoRef}
            autoPlay
            muted
            playsInline
            sx={{
              display: "block",
              height: "100%",
              objectFit: "cover",
              opacity: showVideo ? 1 : 0,
              transition: "opacity 220ms ease",
              width: "100%",
              bgcolor: tokens.color.bg.base,
            }}
          />
        ) : null}
        {live && !showVideo ? (
          <StatusOverlay
            primary={error ? "Stream error" : "Connecting to emulator…"}
            secondary={error ?? "Negotiating scrcpy WebRTC channel."}
          />
        ) : null}
        {!live ? (
          <Stack
            spacing={1.25}
            sx={{
              alignItems: "center",
              color: tokens.color.text.secondary,
              height: "100%",
              justifyContent: "center",
              px: 3,
              textAlign: "center",
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
              {running ? "Allocating session…" : "Emulator not running"}
            </Typography>
            <Typography
              sx={{
                color: tokens.color.text.secondary,
                fontSize: 13,
                maxWidth: 220,
              }}
            >
              {running
                ? "Waiting for the bridge to publish a session URL."
                : "Allocate an AVD to stream Android into the cockpit."}
            </Typography>
            {!running ? (
              <Button
                variant="contained"
                color="primary"
                size="small"
                onClick={onStart}
                disabled={!onStart}
              >
                START
              </Button>
            ) : null}
          </Stack>
        ) : null}
      </Box>
      <Stack
        direction="row"
        spacing={0.75}
        sx={{
          bgcolor: tokens.color.bg.surface,
          border: `1px solid ${tokens.color.border.subtle}`,
          borderRadius: `${tokens.radius.pill}px`,
          px: 1.5,
          py: 0.5,
        }}
      >
        <IconButton
          size="small"
          aria-label="Back"
          disabled={!showVideo}
          onClick={() => dispatch({ type: "key", keycode: KEYCODE_BACK })}
          sx={{ color: tokens.color.text.secondary }}
        >
          <ArrowBackRounded sx={{ fontSize: 18 }} />
        </IconButton>
        <IconButton
          size="small"
          aria-label="Home"
          disabled={!showVideo}
          onClick={() => dispatch({ type: "key", keycode: KEYCODE_HOME })}
          sx={{ color: tokens.color.text.secondary }}
        >
          <HomeRounded sx={{ fontSize: 18 }} />
        </IconButton>
        <IconButton
          size="small"
          aria-label="Recent apps"
          disabled={!showVideo}
          onClick={() =>
            dispatch({ type: "key", keycode: KEYCODE_RECENT_APPS })
          }
          sx={{ color: tokens.color.text.secondary }}
        >
          <ViewAgendaRounded sx={{ fontSize: 18 }} />
        </IconButton>
      </Stack>
    </Stack>
  );
}

function StatusOverlay({
  primary,
  secondary,
}: {
  primary: string;
  secondary: string;
}) {
  return (
    <Stack
      spacing={1}
      sx={{
        alignItems: "center",
        color: tokens.color.text.secondary,
        height: "100%",
        justifyContent: "center",
        left: 0,
        position: "absolute",
        px: 3,
        textAlign: "center",
        top: 0,
        width: "100%",
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
        {primary}
      </Typography>
      <Typography
        sx={{
          color: tokens.color.text.secondary,
          fontSize: 13,
          maxWidth: 220,
        }}
      >
        {secondary}
      </Typography>
    </Stack>
  );
}

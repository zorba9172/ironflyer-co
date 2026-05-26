"use client";

// MobileStackPicker — radio-card chooser used inside the StackDecision
// flow. Each kind selection reveals secondary fields (appId, display
// name, version, targets) so the operator sees the full mobile shape
// in a single visual surface before committing.
//
// Colors come exclusively from tokens / MUI palette. No raw hex.

import {
  Box,
  Radio,
  RadioGroup,
  Stack,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from "@mui/material";
import { useMemo } from "react";
import { tokens } from "../../../theme";

export type MobileStackKind =
  | "none"
  | "expo"
  | "react-native-bare"
  | "android-native"
  | "ios-native"
  | "flutter";

export type MobileStackTarget = "android" | "ios";

export interface MobileStackValue {
  kind: MobileStackKind;
  targets: MobileStackTarget[];
  appId: string;
  displayName: string;
  version: string;
}

export interface MobileStackPickerProps {
  value: MobileStackValue;
  onChange: (v: MobileStackValue) => void;
  disabled?: boolean;
}

interface StackOption {
  kind: MobileStackKind;
  title: string;
  subtitle: string;
}

const OPTIONS: StackOption[] = [
  { kind: "none", title: "None — web only", subtitle: "Skip mobile entirely." },
  {
    kind: "expo",
    title: "Expo (recommended)",
    subtitle: "Hot reload + EAS Build + iOS without a Mac",
  },
  {
    kind: "react-native-bare",
    title: "React Native bare",
    subtitle: "Eject if you need custom native modules",
  },
  {
    kind: "android-native",
    title: "Android native (Kotlin)",
    subtitle: "Compose UI; Linux build sandbox",
  },
  {
    kind: "ios-native",
    title: "iOS native (Swift)",
    subtitle: "SwiftUI; requires Pro tier Mac workspace",
  },
  {
    kind: "flutter",
    title: "Flutter",
    subtitle: "Single codebase; Dart",
  },
];

// Reverse-DNS check: at least two dot-separated lowercase segments.
const APP_ID_RE = /^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$/;

function targetLockFor(kind: MobileStackKind): MobileStackTarget[] | null {
  switch (kind) {
    case "android-native":
      return ["android"];
    case "ios-native":
      return ["ios"];
    case "none":
      return [];
    default:
      return null;
  }
}

export function MobileStackPicker({
  value,
  onChange,
  disabled,
}: MobileStackPickerProps) {
  const showSecondary = value.kind !== "none";
  const locked = targetLockFor(value.kind);
  const targetsDisabled = disabled || locked !== null;

  const appIdValid = useMemo(
    () => (value.appId ? APP_ID_RE.test(value.appId) : true),
    [value.appId],
  );

  const setKind = (kind: MobileStackKind) => {
    const lock = targetLockFor(kind);
    let next: MobileStackTarget[] = value.targets;
    if (lock !== null) next = lock;
    else if (next.length === 0) next = ["android"];
    onChange({ ...value, kind, targets: next });
  };

  const setTargets = (next: MobileStackTarget[]) => {
    if (next.length === 0) return;
    onChange({ ...value, targets: next });
  };

  return (
    <Stack spacing={2}>
      <RadioGroup
        value={value.kind}
        onChange={(_, v) => setKind(v as MobileStackKind)}
      >
        <Stack
          spacing={1}
          sx={{
            display: "grid",
            gap: 1,
            gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
          }}
        >
          {OPTIONS.map((opt) => {
            const selected = value.kind === opt.kind;
            return (
              <Box
                key={opt.kind}
                component="label"
                htmlFor={`mobile-stack-${opt.kind}`}
                sx={{
                  alignItems: "flex-start",
                  bgcolor: selected
                    ? tokens.color.bg.surfaceHover
                    : tokens.color.bg.surface,
                  border: `1px solid ${
                    selected
                      ? tokens.color.accent.violet
                      : tokens.color.border.subtle
                  }`,
                  borderRadius: 1,
                  cursor: disabled ? "not-allowed" : "pointer",
                  display: "flex",
                  gap: 1,
                  opacity: disabled ? 0.6 : 1,
                  p: 1.25,
                  transition: "border-color 160ms ease, background 160ms ease",
                }}
              >
                <Radio
                  id={`mobile-stack-${opt.kind}`}
                  value={opt.kind}
                  disabled={disabled}
                  size="small"
                  sx={{ p: 0.25, mt: 0.25 }}
                />
                <Box sx={{ minWidth: 0 }}>
                  <Typography
                    sx={{
                      color: tokens.color.text.primary,
                      fontSize: 13.5,
                      fontWeight: 600,
                    }}
                  >
                    {opt.title}
                  </Typography>
                  <Typography
                    sx={{
                      color: tokens.color.text.muted,
                      fontSize: 12,
                      lineHeight: 1.4,
                    }}
                  >
                    {opt.subtitle}
                  </Typography>
                </Box>
              </Box>
            );
          })}
        </Stack>
      </RadioGroup>

      {showSecondary ? (
        <Stack
          spacing={1.5}
          sx={{
            bgcolor: tokens.color.bg.surface,
            border: `1px solid ${tokens.color.border.subtle}`,
            borderRadius: 1,
            p: 1.5,
          }}
        >
          <Stack
            direction={{ xs: "column", md: "row" }}
            spacing={1.5}
            sx={{ alignItems: { md: "flex-start" } }}
          >
            <TextField
              label="App ID"
              placeholder="com.example.app"
              value={value.appId}
              onChange={(e) => onChange({ ...value, appId: e.target.value })}
              disabled={disabled}
              fullWidth
              error={!appIdValid}
              helperText={
                appIdValid
                  ? "Reverse-DNS (com.company.app)"
                  : "Use reverse-DNS: com.company.app"
              }
            />
            <TextField
              label="Display name"
              placeholder="My App"
              value={value.displayName}
              onChange={(e) =>
                onChange({ ...value, displayName: e.target.value })
              }
              disabled={disabled}
              fullWidth
            />
            <TextField
              label="Version"
              placeholder="0.1.0"
              value={value.version}
              onChange={(e) =>
                onChange({ ...value, version: e.target.value })
              }
              disabled={disabled}
              sx={{ width: { xs: "100%", md: 140 } }}
            />
          </Stack>
          <Stack spacing={0.5}>
            <Typography
              sx={{
                color: tokens.color.text.muted,
                fontFamily: tokens.font.mono,
                fontSize: 11,
                letterSpacing: 1,
                textTransform: "uppercase",
              }}
            >
              Targets
            </Typography>
            <ToggleButtonGroup
              value={value.targets}
              onChange={(_, next: MobileStackTarget[] | null) =>
                next && setTargets(next)
              }
              disabled={targetsDisabled}
              size="small"
              color="primary"
            >
              <ToggleButton value="android">Android</ToggleButton>
              <ToggleButton value="ios">iOS</ToggleButton>
            </ToggleButtonGroup>
          </Stack>
        </Stack>
      ) : null}
    </Stack>
  );
}

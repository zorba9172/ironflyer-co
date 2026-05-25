"use client";

// WorkbenchHeader — sticky top bar of the workbench shell.
//
//   Left   : breadcrumb back to /studio · project name · workspace chip
//   Center : segmented "Preview / Mobile / Code" primary-pane selector
//   Right  : focus-mode toggle, run controls, publish button
//
// A second sub-row sits beneath the header and renders the live status
// strip — execution status pill, last patch summary, wallet chip — so
// the operator can read the run health without leaving the workbench.

import {
  ArrowBackRounded,
  CenterFocusStrongRounded,
  CenterFocusWeakRounded,
  CodeRounded,
  LaptopMacRounded,
  PhoneIphoneRounded,
} from "@mui/icons-material";
import type { SvgIconComponent } from "@mui/icons-material";
import {
  Box,
  Chip,
  IconButton,
  Stack,
  Tooltip,
  Typography,
} from "@mui/material";
import Link from "next/link";
import { tokens } from "../../theme";
import { formatMoney } from "../../lib/format";
import {
  useWalletQuery,
  type ExecutionCoreFragment,
} from "../../lib/gql/__generated__";
import { StatusBadge } from "../cockpit/StatusBadge";
import { PublishButton } from "./PublishButton";
import { RunControls } from "./RunControls";
import type { WorkbenchPrimary } from "./useWorkbenchLayout";

export interface WorkbenchHeaderProps {
  projectName: string;
  projectID?: string;
  execution: ExecutionCoreFragment | null;
  primary: WorkbenchPrimary;
  onPrimaryChange: (next: WorkbenchPrimary) => void;
  focus: boolean;
  onToggleFocus: () => void;
  lastPatchSummary?: string | null;
}

interface PrimaryTab {
  key: WorkbenchPrimary;
  label: string;
  icon: SvgIconComponent;
}

const PRIMARY_TABS: PrimaryTab[] = [
  { key: "preview", label: "Preview", icon: LaptopMacRounded },
  { key: "mobile", label: "Mobile", icon: PhoneIphoneRounded },
  { key: "code", label: "Code", icon: CodeRounded },
];

const TERMINAL = new Set(["succeeded", "failed", "stopped", "killed", "refunded"]);

export function WorkbenchHeader({
  projectName,
  projectID,
  execution,
  primary,
  onPrimaryChange,
  focus,
  onToggleFocus,
  lastPatchSummary,
}: WorkbenchHeaderProps) {
  const isTerminal = execution ? TERMINAL.has(execution.status) : true;
  const walletQuery = useWalletQuery({
    fetchPolicy: "cache-and-network",
    pollInterval: isTerminal ? 0 : 15000,
  });
  const wallet = walletQuery.data?.wallet;

  return (
    <Box
      sx={{
        bgcolor: tokens.color.bg.surface,
        borderBottom: `1px solid ${tokens.color.border.subtle}`,
        minWidth: 0,
      }}
    >
      <Stack
        direction="row"
        spacing={{ xs: 0.75, md: 1.25 }}
        sx={{
          alignItems: "center",
          height: 56,
          minWidth: 0,
          px: { xs: 1, md: 1.6 },
        }}
      >
        <Tooltip title="Back to Studio" arrow>
          <IconButton
            component={Link}
            href="/studio"
            size="small"
            aria-label="Back to Studio"
            sx={{
              color: tokens.color.text.secondary,
              "&:hover": { color: tokens.color.text.primary },
            }}
          >
            <ArrowBackRounded sx={{ fontSize: 17 }} />
          </IconButton>
        </Tooltip>
        <Stack direction="row" spacing={0.6} sx={{ alignItems: "center", minWidth: 0 }}>
          <Box
            component={Link}
            href="/studio"
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 10.5,
              fontWeight: 700,
              letterSpacing: 0.6,
              textDecoration: "none",
              textTransform: "uppercase",
              "&:hover": { color: tokens.color.text.primary },
            }}
          >
            Studio
          </Box>
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 11,
            }}
          >
            /
          </Typography>
          <Typography
            sx={{
              color: tokens.color.text.primary,
              fontSize: 14,
              fontWeight: 800,
              maxWidth: { xs: 140, sm: 260, md: 320 },
              overflow: "hidden",
              textOverflow: "ellipsis",
              whiteSpace: "nowrap",
            }}
          >
            {projectName}
          </Typography>
        </Stack>

        <Chip
          size="small"
          label={
            execution?.workspaceID
              ? `ws ${execution.workspaceID.slice(0, 8)}`
              : "Default workspace"
          }
          sx={{
            bgcolor: tokens.color.bg.surfaceRaised,
            border: `1px solid ${tokens.color.border.subtle}`,
            borderRadius: 0.75,
            color: tokens.color.text.secondary,
            display: { xs: "none", md: "inline-flex" },
            fontFamily: tokens.font.mono,
            fontSize: 10,
            fontWeight: 700,
            height: 22,
            letterSpacing: 0.6,
            "& .MuiChip-label": { px: 1 },
          }}
        />

        <Box sx={{ flex: 1 }} />

        <PrimarySegmented primary={primary} onChange={onPrimaryChange} />

        <Box sx={{ flex: 1 }} />

        <Stack direction="row" spacing={0.6} sx={{ alignItems: "center" }}>
          <Tooltip title={focus ? "Exit focus (F)" : "Focus mode (F)"} arrow>
            <IconButton
              size="small"
              aria-label="Focus mode"
              onClick={onToggleFocus}
              sx={{
                border: `1px solid ${focus ? tokens.color.border.accent : tokens.color.border.subtle}`,
                bgcolor: focus
                  ? `${tokens.color.accent.purple}33`
                  : "transparent",
                color: focus ? tokens.color.text.primary : tokens.color.text.secondary,
                "&:hover": {
                  bgcolor: tokens.color.bg.surfaceHover,
                  color: tokens.color.text.primary,
                },
              }}
            >
              {focus ? (
                <CenterFocusStrongRounded sx={{ fontSize: 17 }} />
              ) : (
                <CenterFocusWeakRounded sx={{ fontSize: 17 }} />
              )}
            </IconButton>
          </Tooltip>
          <Box sx={{ display: { xs: "none", sm: "contents" } }}>
            {execution && projectID ? (
              <RunControls execution={execution} projectID={projectID} />
            ) : null}
          </Box>
          {execution ? <PublishButton execution={execution} /> : null}
        </Stack>
      </Stack>

      <StatusStrip
        execution={execution}
        lastPatchSummary={lastPatchSummary ?? null}
        walletAvailable={wallet?.availableUSD ?? null}
      />
    </Box>
  );
}

function PrimarySegmented({
  primary,
  onChange,
}: {
  primary: WorkbenchPrimary;
  onChange: (next: WorkbenchPrimary) => void;
}) {
  return (
    <Stack
      direction="row"
      spacing={0.25}
      role="tablist"
      aria-label="Primary pane"
      sx={{
        alignItems: "center",
        bgcolor: tokens.color.bg.inset,
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: 1,
        display: { xs: "none", sm: "inline-flex" },
        p: 0.25,
      }}
    >
      {PRIMARY_TABS.map((t) => {
        const active = primary === t.key;
        const Icon = t.icon;
        return (
          <Box
            key={t.key}
            role="tab"
            aria-selected={active}
            onClick={() => onChange(t.key)}
            sx={{
              alignItems: "center",
              bgcolor: active
                ? `${tokens.color.accent.purple}55`
                : "transparent",
              border: `1px solid ${active ? tokens.color.border.accent : "transparent"}`,
              borderRadius: 0.75,
              color: active
                ? tokens.color.text.primary
                : tokens.color.text.secondary,
              cursor: "pointer",
              display: "flex",
              fontFamily: tokens.font.mono,
              fontSize: 11,
              fontWeight: 700,
              gap: 0.5,
              height: 28,
              letterSpacing: 0.6,
              px: 1.1,
              textTransform: "uppercase",
              transition: `background ${tokens.motion.fast} ease, color ${tokens.motion.fast} ease`,
              "&:hover": {
                color: tokens.color.text.primary,
              },
            }}
          >
            <Icon sx={{ fontSize: 14 }} />
            {t.label}
          </Box>
        );
      })}
    </Stack>
  );
}

function StatusStrip({
  execution,
  lastPatchSummary,
  walletAvailable,
}: {
  execution: ExecutionCoreFragment | null;
  lastPatchSummary: string | null;
  walletAvailable: number | null;
}) {
  if (!execution) {
    return (
      <Box
        sx={{
          alignItems: "center",
          borderTop: `1px solid ${tokens.color.border.subtle}`,
          color: tokens.color.text.muted,
          display: "flex",
          fontFamily: tokens.font.mono,
          fontSize: 11,
          height: 32,
          letterSpacing: 0.6,
          minWidth: 0,
          px: { xs: 1.2, md: 1.6 },
          textTransform: "uppercase",
        }}
      >
        No execution yet
      </Box>
    );
  }

  const budget = execution.budgetUSD || 0;
  const spent = execution.spentUSD || 0;
  const pct = budget > 0 ? Math.min(1, Math.max(0, spent / budget)) : 0;
  const spendColor =
    pct >= 0.8
      ? tokens.color.accent.danger
      : pct >= 0.5
        ? tokens.color.accent.warning
        : tokens.color.accent.violet;
  const scorePct = Math.round((execution.completionScore ?? 0) * 100);

  return (
    <Box
      sx={{
        alignItems: "center",
        borderTop: `1px solid ${tokens.color.border.subtle}`,
        display: "flex",
        gap: { xs: 1, md: 1.6 },
        height: 36,
        minWidth: 0,
        overflowX: "auto",
        px: { xs: 1.2, md: 1.6 },
      }}
      role="status"
    >
      <StatusBadge status={execution.status} />
      <Box
        sx={{
          alignItems: "center",
          display: "flex",
          gap: 0.5,
          minWidth: 0,
        }}
      >
        <Typography
          sx={{
            color: spendColor,
            fontFamily: tokens.font.mono,
            fontSize: 11,
            fontWeight: 800,
          }}
        >
          {formatMoney(spent)}
        </Typography>
        <Typography
          sx={{
            color: tokens.color.text.muted,
            fontFamily: tokens.font.mono,
            fontSize: 10.5,
          }}
        >
          / {formatMoney(budget)}
        </Typography>
        <Box
          sx={{
            bgcolor: tokens.color.bg.inset,
            borderRadius: 999,
            height: 4,
            ml: 1,
            position: "relative",
            width: 90,
          }}
        >
          <Box
            sx={{
              bgcolor: spendColor,
              borderRadius: 999,
              height: 4,
              width: `${pct * 100}%`,
            }}
          />
        </Box>
      </Box>
      <Typography
        sx={{
          color: tokens.color.text.secondary,
          fontFamily: tokens.font.mono,
          fontSize: 11,
        }}
      >
        Gate {scorePct}/100
      </Typography>
      {lastPatchSummary ? (
        <Typography
          sx={{
            color: tokens.color.text.muted,
            display: { xs: "none", md: "inline" },
            flex: 1,
            fontFamily: tokens.font.mono,
            fontSize: 11,
            minWidth: 0,
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
          }}
        >
          last patch · {lastPatchSummary}
        </Typography>
      ) : (
        <Box sx={{ flex: 1 }} />
      )}
      <Tooltip title="Wallet" arrow>
        <Box
          component={Link}
          href="/wallet"
          sx={{
            alignItems: "center",
            color: tokens.color.text.secondary,
            display: "inline-flex",
            fontFamily: tokens.font.mono,
            fontSize: 11,
            gap: 0.4,
            textDecoration: "none",
            "&:hover": { color: tokens.color.accent.violet },
          }}
        >
          Wallet
          <Box
            component="span"
            sx={{
              color: tokens.color.accent.violet,
              fontWeight: 800,
            }}
          >
            {walletAvailable != null ? formatMoney(walletAvailable) : "—"}
          </Box>
        </Box>
      </Tooltip>
    </Box>
  );
}

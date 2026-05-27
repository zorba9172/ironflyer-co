"use client";

// HeroPromptInput — the chat-as-hero composer that converts visitors
// into paid executions. The textarea is large, autosizes on input, and
// cycles a placeholder so an empty hero never feels dead. The right
// edge hosts the budget chip (auto or pinned via a slider in the
// "Adjust budget" popover) and the submit button. A plan-toggle sits
// below for users who want to review the parsed idea before the wallet
// hold lands.
//
// The component is a pure controlled composer — value, budget, and
// plan-first state are owned by the parent so other home-page widgets
// (category chips, blueprint cards) can drop text in and focus the
// field.

import {
  AccountBalanceWalletOutlined,
  ArrowForwardRounded,
  AutoAwesomeRounded,
  TuneRounded,
} from "@mui/icons-material";
import {
  Box,
  Button,
  IconButton,
  Popover,
  Slider,
  Stack,
  Switch,
  TextField,
  Tooltip,
  Typography,
} from "@mui/material";
import {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
  type FormEvent,
  type KeyboardEvent,
} from "react";
import { formatMoney } from "../../lib/format";
import { tokens } from "../../theme";

const PLACEHOLDERS = [
  "A CRM for landscape contractors with a kanban deal pipeline",
  "A landing page for my Pilates studio with class booking",
  "A Discord moderation bot with audit logs",
  "An invoicing tool that emails reminders on overdue balances",
  "A Next.js dashboard for a podcast network with episode stats",
  "A Go HTTP API for managing a meal-delivery inventory",
  "A booking site for a private chef with deposit collection",
  "A blog with admin, Markdown posts, and tag search",
];

export interface HeroPromptInputHandle {
  focus: () => void;
}

export interface HeroPromptSubmitPayload {
  text: string;
  budgetUSD: number | null;
  planFirst: boolean;
}

export interface HeroPromptInputProps {
  timing?: "dark" | "light";
  value: string;
  onChange: (next: string) => void;
  onSubmit: (payload: HeroPromptSubmitPayload) => void;
  submitting?: boolean;
  // When null, we treat the budget as "auto" (orchestrator parser
  // suggests). When a number, the user has pinned it.
  budgetUSD: number | null;
  onBudgetChange: (next: number | null) => void;
  planFirst: boolean;
  onPlanFirstChange: (next: boolean) => void;
  // Optional extra row of seed prompt chips rendered ABOVE the
  // textarea (e.g. the "More" idea picker). The home page provides
  // the category strip below.
}

export const HeroPromptInput = forwardRef<
  HeroPromptInputHandle,
  HeroPromptInputProps
>(function HeroPromptInput(
  {
    timing = "dark",
    value,
    onChange,
    onSubmit,
    submitting = false,
    budgetUSD,
    onBudgetChange,
    planFirst,
    onPlanFirstChange,
  },
  ref,
) {
  const inputRef = useRef<HTMLTextAreaElement | null>(null);
  const [placeholderIndex, setPlaceholderIndex] = useState(0);
  const [budgetAnchor, setBudgetAnchor] = useState<HTMLElement | null>(null);

  useImperativeHandle(ref, () => ({
    focus: () => inputRef.current?.focus(),
  }));

  // Cycle the placeholder every 4s while the user hasn't typed.
  useEffect(() => {
    if (value.trim().length > 0) return;
    const t = window.setInterval(() => {
      setPlaceholderIndex((i) => (i + 1) % PLACEHOLDERS.length);
    }, 4000);
    return () => window.clearInterval(t);
  }, [value]);

  const placeholder = PLACEHOLDERS[placeholderIndex];

  const canSubmit = value.trim().length >= 8 && !submitting;
  const light = timing === "light";
  const c = {
    text: light ? "#111638" : tokens.color.text.primary,
    secondary: light ? "#636b8e" : tokens.color.text.secondary,
    muted: light ? "#858ca8" : tokens.color.text.muted,
    surface: light ? "rgba(255,255,255,0.88)" : "rgba(15,17,42,0.86)",
    control: light ? "rgba(255,255,255,0.72)" : tokens.color.bg.surfaceRaised,
    border: light ? "rgba(128,84,255,0.20)" : tokens.color.border.subtle,
    hover: light ? "rgba(143,77,255,0.08)" : tokens.color.bg.surfaceHover,
    shadow: light
      ? "0 34px 120px rgba(151,73,255,0.18), 0 18px 90px rgba(239,72,186,0.12)"
      : "0 30px 110px rgba(92,34,214,0.34), 0 0 80px rgba(225,73,201,0.12)",
  };

  const handleSubmit = (e?: FormEvent) => {
    e?.preventDefault();
    if (!canSubmit) return;
    onSubmit({
      text: value.trim(),
      budgetUSD: budgetUSD,
      planFirst,
    });
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    // Submit on Cmd/Ctrl+Enter; plain Enter inserts a newline like a
    // real chat composer.
    if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
      e.preventDefault();
      handleSubmit();
    }
  };

  return (
    <Box
      component="form"
      onSubmit={handleSubmit}
      sx={{
        position: "relative",
        width: "100%",
        maxWidth: 846,
        mx: "auto",
        minHeight: { xs: 198, sm: 206 },
        borderRadius: "22px",
        border: `1px solid ${c.border}`,
        bgcolor: c.surface,
        backdropFilter: "saturate(155%) blur(22px)",
        boxShadow: c.shadow,
        backgroundImage: light
          ? "linear-gradient(145deg, rgba(255,255,255,0.96), rgba(255,255,255,0.78))"
          : "linear-gradient(145deg, rgba(21,23,54,0.94), rgba(10,11,29,0.88))",
        transition: `border-color ${tokens.motion.fast} ease, box-shadow ${tokens.motion.base} ease`,
        overflow: "hidden",
        "&::before": {
          content: '""',
          position: "absolute",
          inset: 0,
          pointerEvents: "none",
          background: light
            ? "radial-gradient(360px 150px at 86% 18%, rgba(183,91,255,0.16), transparent 70%), radial-gradient(320px 170px at 9% 108%, rgba(255,102,92,0.12), transparent 72%)"
            : "radial-gradient(360px 150px at 86% 18%, rgba(183,91,255,0.18), transparent 70%), radial-gradient(320px 170px at 9% 108%, rgba(255,102,92,0.12), transparent 72%)",
        },
        "&:focus-within": {
          borderColor: tokens.color.border.accent,
          boxShadow: `${c.shadow}, 0 0 0 3px ${tokens.color.accent.purple}2e`,
        },
      }}
    >
      <TextField
        inputRef={inputRef}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={handleKeyDown}
        placeholder={placeholder}
        multiline
        minRows={2}
        maxRows={4}
        fullWidth
        variant="standard"
        slotProps={{
          input: {
            disableUnderline: true,
            sx: {
              px: { xs: 2, sm: 3.2 },
              pt: { xs: 2.1, sm: 2.5 },
              pb: 1.8,
              fontSize: { xs: 17.5, sm: 19.5 },
              lineHeight: 1.42,
              color: c.text,
              fontFamily: tokens.font.family,
              fontWeight: 600,
              "& textarea::placeholder": {
                color: c.text,
                opacity: 1,
              },
            },
          },
        }}
      />

      <Stack
        direction="row"
        alignItems="center"
        spacing={{ xs: 0.6, sm: 1 }}
        useFlexGap
        flexWrap="wrap"
        sx={{
          position: "relative",
          px: { xs: 1.55, sm: 2.1 },
          pb: { xs: 1.35, sm: 1.75 },
          pt: 1.1,
        }}
      >
        <Tooltip
          title={
            budgetUSD === null
              ? "Auto-budget: parser suggests, capped by wallet"
              : "Pinned budget — click to adjust"
          }
          arrow
        >
          <Button
            size="small"
            variant="text"
            onClick={(e) => setBudgetAnchor(e.currentTarget)}
            startIcon={
              budgetUSD === null ? (
                <AutoAwesomeRounded sx={{ fontSize: 16 }} />
              ) : (
                <AccountBalanceWalletOutlined sx={{ fontSize: 16 }} />
              )
            }
            endIcon={<TuneRounded sx={{ fontSize: 14 }} />}
            sx={{
              color: c.secondary,
              bgcolor: c.control,
              border: `1px solid ${c.border}`,
              px: 1.35,
              py: 0.62,
              minHeight: 38,
              fontFamily: tokens.font.mono,
              fontSize: 11.8,
              borderRadius: 1.5,
              "&:hover": { bgcolor: c.hover },
            }}
          >
            {budgetUSD === null
              ? "Auto budget"
              : `Budget ${formatMoney(budgetUSD)}`}
          </Button>
        </Tooltip>

        <Popover
          open={!!budgetAnchor}
          anchorEl={budgetAnchor}
          onClose={() => setBudgetAnchor(null)}
          anchorOrigin={{ vertical: "bottom", horizontal: "left" }}
          slotProps={{
            paper: {
              sx: {
                mt: 1,
                p: 2,
                width: 280,
                border: `1px solid ${c.border}`,
                bgcolor: light ? "#fff" : tokens.color.bg.surface,
              },
            },
          }}
        >
          <Stack spacing={1.5}>
            <Stack
              direction="row"
              justifyContent="space-between"
              alignItems="center"
            >
              <Typography sx={{ fontSize: 13, fontWeight: 700 }}>
                Wallet hold
              </Typography>
              <Typography
                sx={{
                  fontFamily: tokens.font.mono,
                  fontSize: 13,
                  color: tokens.color.accent.violet,
                }}
              >
                {budgetUSD === null ? "Auto" : formatMoney(budgetUSD)}
              </Typography>
            </Stack>
            <Slider
              size="small"
              min={1}
              max={100}
              step={1}
              value={budgetUSD ?? 5}
              onChange={(_, v) =>
                onBudgetChange(typeof v === "number" ? v : v[0])
              }
              sx={{
                color: tokens.color.accent.violet,
                "& .MuiSlider-rail": { opacity: 0.3 },
              }}
            />
            <Stack direction="row" justifyContent="space-between">
              <Button
                size="small"
                variant="text"
                onClick={() => onBudgetChange(null)}
                sx={{ fontSize: 12, color: tokens.color.text.secondary }}
              >
                Use auto-budget
              </Button>
              <Button
                size="small"
                variant="text"
                onClick={() => setBudgetAnchor(null)}
                sx={{ fontSize: 12, color: tokens.color.text.primary }}
              >
                Done
              </Button>
            </Stack>
            <Typography
              sx={{
                fontSize: 11,
                color: tokens.color.text.muted,
                lineHeight: 1.4,
              }}
            >
              Funds are held against your wallet before any expensive call runs.
              Unused hold is released at commit.
            </Typography>
          </Stack>
        </Popover>

        <Stack direction="row" alignItems="center" spacing={0.5}>
          <Typography
            sx={{
              fontSize: 11.5,
              fontFamily: tokens.font.mono,
              color: planFirst ? c.text : c.muted,
              letterSpacing: 0.4,
            }}
          >
            PLAN FIRST
          </Typography>
          <Switch
            size="small"
            checked={planFirst}
            onChange={(_, v) => onPlanFirstChange(v)}
            sx={{
              "& .MuiSwitch-thumb": { boxShadow: "none" },
              "& .Mui-checked + .MuiSwitch-track": {
                bgcolor: `${tokens.color.accent.violet} !important`,
                opacity: "0.7 !important",
              },
              "& .Mui-checked .MuiSwitch-thumb": {
                color: tokens.color.accent.violet,
              },
            }}
          />
        </Stack>

        <Box sx={{ flex: 1 }} />

        <Tooltip title="Enhance prompt" arrow>
          <IconButton
            onClick={() => handleSubmit()}
            disabled={!canSubmit}
            aria-label="Enhance prompt"
            sx={{
              width: 44,
              height: 44,
              borderRadius: 1.5,
              border: `1px solid ${c.border}`,
              color: tokens.color.accent.violet,
              bgcolor: light
                ? "rgba(255,255,255,0.52)"
                : "rgba(255,255,255,0.04)",
              "&:hover": {
                bgcolor: c.hover,
                borderColor: c.border,
              },
            }}
          >
            <AutoAwesomeRounded sx={{ fontSize: 22 }} />
          </IconButton>
        </Tooltip>

        <Button
          type="submit"
          variant="contained"
          color="primary"
          disabled={!canSubmit}
          endIcon={<ArrowForwardRounded sx={{ fontSize: 18 }} />}
          sx={{
            minHeight: 44,
            px: { xs: 2.1, sm: 2.8 },
            fontSize: 15.5,
            fontWeight: 900,
            letterSpacing: 0.2,
            ml: { xs: "auto", sm: 0 },
            borderRadius: 1.5,
            background: light
              ? `linear-gradient(100deg, ${tokens.color.accent.coral}, ${tokens.color.brand.magenta} 52%, ${tokens.color.accent.violet})`
              : `linear-gradient(100deg, ${tokens.color.accent.coral}, ${tokens.color.brand.magenta} 52%, ${tokens.color.accent.violet})`,
            boxShadow: light
              ? "0 12px 30px rgba(183,78,255,0.18)"
              : "0 14px 34px rgba(183,78,255,0.28)",
          }}
        >
          {submitting ? "Launching..." : "Build it"}
        </Button>
      </Stack>
    </Box>
  );
});

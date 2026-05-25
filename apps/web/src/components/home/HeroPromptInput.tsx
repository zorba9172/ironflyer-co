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
  AutoAwesomeRounded,
  RocketLaunchRounded,
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

export const HeroPromptInput = forwardRef<HeroPromptInputHandle, HeroPromptInputProps>(
  function HeroPromptInput(
    {
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
          maxWidth: 880,
          mx: "auto",
          borderRadius: `${tokens.radius.lg}px`,
          border: `1px solid ${tokens.color.border.subtle}`,
          bgcolor: tokens.color.bg.surfaceRaised,
          backdropFilter: "saturate(140%) blur(10px)",
          boxShadow: tokens.shadow.md,
          transition: `border-color ${tokens.motion.fast} ease, box-shadow ${tokens.motion.base} ease`,
          "&:focus-within": {
            borderColor: tokens.color.border.accent,
            boxShadow: `${tokens.shadow.lg}, 0 0 0 3px ${tokens.color.accent.purple}2e`,
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
          minRows={3}
          maxRows={8}
          fullWidth
          variant="standard"
          slotProps={{
            input: {
              disableUnderline: true,
              sx: {
                px: { xs: 1.75, sm: 2.5 },
                pt: { xs: 1.5, sm: 2 },
                pb: 1,
                fontSize: { xs: 15.5, sm: 18 },
                lineHeight: 1.5,
                color: tokens.color.text.primary,
                fontFamily: tokens.font.family,
                "& textarea::placeholder": {
                  color: tokens.color.text.muted,
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
            px: { xs: 1, sm: 1.5 },
            py: 1,
            borderTop: `1px solid ${tokens.color.border.subtle}`,
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
                color: tokens.color.text.secondary,
                bgcolor: tokens.color.bg.surfaceRaised,
                border: `1px solid ${tokens.color.border.subtle}`,
                px: 1.25,
                py: 0.5,
                minHeight: 32,
                fontFamily: tokens.font.mono,
                fontSize: 12,
                "&:hover": { bgcolor: tokens.color.bg.surfaceHover },
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
                  border: `1px solid ${tokens.color.border.subtle}`,
                  bgcolor: tokens.color.bg.surface,
                },
              },
            }}
          >
            <Stack spacing={1.5}>
              <Stack direction="row" justifyContent="space-between" alignItems="center">
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
                Funds are held against your wallet before any expensive
                call runs. Unused hold is released at commit.
              </Typography>
            </Stack>
          </Popover>

          <Stack direction="row" alignItems="center" spacing={0.5}>
            <Typography
              sx={{
                fontSize: 11.5,
                fontFamily: tokens.font.mono,
                color: planFirst
                  ? tokens.color.text.primary
                  : tokens.color.text.muted,
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

          <Typography
            sx={{
              display: { xs: "none", md: "block" },
              fontSize: 11,
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              mr: 1,
            }}
          >
            ⌘↵
          </Typography>

          <Button
            type="submit"
            variant="contained"
            color="primary"
            disabled={!canSubmit}
            startIcon={<RocketLaunchRounded sx={{ fontSize: 18 }} />}
            sx={{
              minHeight: 44,
              px: { xs: 2, sm: 2.5 },
              fontWeight: 800,
              letterSpacing: 0.2,
              ml: { xs: "auto", sm: 0 },
            }}
          >
            {submitting ? "Launching…" : planFirst ? "Plan it" : "Build it"}
          </Button>
        </Stack>

        {/* Floating "submit" affordance in the textarea top-right when
            the user has typed something — gives a clear keyboard hint
            on mobile too. */}
        {value.trim().length > 0 && (
          <Box
            sx={{
              position: "absolute",
              top: 10,
              right: 12,
              display: { xs: "none", sm: "flex" },
              alignItems: "center",
              gap: 0.5,
              pointerEvents: "none",
            }}
          >
            <IconButton
              size="small"
              tabIndex={-1}
              sx={{
                pointerEvents: "auto",
                color: tokens.color.accent.violet,
                "&:hover": { bgcolor: `${tokens.color.accent.violet}1a` },
              }}
              onClick={() => handleSubmit()}
              aria-label="Submit prompt"
            >
              <RocketLaunchRounded sx={{ fontSize: 18 }} />
            </IconButton>
          </Box>
        )}
      </Box>
    );
  },
);

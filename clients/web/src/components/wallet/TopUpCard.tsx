"use client";

// TopUpCard — preset amount chips + custom amount input + lime "Pay with
// Stripe" CTA. Calls walletCreateTopUp and redirects to the returned
// Stripe Checkout url. Server enforces the supported tier list; we keep
// the six presets the orchestrator accepts (10, 25, 50, 100, 250, 500).

import { OpenInNewRounded } from "@mui/icons-material";
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Stack,
  TextField,
  Typography,
} from "@mui/material";
import { useState } from "react";
import { useWalletCreateTopUpMutation } from "../../lib/gql/__generated__";
import { tokens } from "../../theme";

const PRESETS = [10, 25, 50, 100, 250, 500];
const MIN_CUSTOM = 5;

export function TopUpCard() {
  const [selected, setSelected] = useState<number>(25);
  const [custom, setCustom] = useState<string>("");
  const [error, setError] = useState<string | null>(null);
  const [createTopUp, { loading }] = useWalletCreateTopUpMutation();

  const useCustom = custom.trim().length > 0;
  const customAmount = Number(custom);
  const customValid = useCustom && Number.isFinite(customAmount) && customAmount >= MIN_CUSTOM;
  const amount = useCustom ? customAmount : selected;
  const disabled = loading || (useCustom && !customValid);

  const onPay = async () => {
    if (disabled) return;
    setError(null);
    try {
      const res = await createTopUp({ variables: { amountUSD: amount } });
      const url = res.data?.walletCreateTopUp.url;
      if (url) {
        window.location.assign(url);
      } else {
        setError("Stripe did not return a checkout URL.");
      }
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Could not open Stripe Checkout. Try again.",
      );
    }
  };

  return (
    <Box
      sx={{
        bgcolor: tokens.color.bg.surface,
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: 1,
        p: { xs: 3, md: 4 },
      }}
    >
      <Stack spacing={2.5}>
        <Stack spacing={0.5}>
          <Typography
            component="h2"
            sx={{ fontSize: 18, fontWeight: 700, color: tokens.color.text.primary }}
          >
            Add credits
          </Typography>
          <Typography sx={{ fontSize: 13, color: tokens.color.text.secondary }}>
            Prepaid credits fund every paid execution. Pick a preset or set your own
            amount.
          </Typography>
        </Stack>

        <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
          {PRESETS.map((value) => {
            const active = !useCustom && value === selected;
            return (
              <Button
                key={value}
                onClick={() => {
                  setSelected(value);
                  setCustom("");
                  setError(null);
                }}
                size="small"
                sx={{
                  minWidth: 72,
                  bgcolor: active ? tokens.color.accent.violet : tokens.color.bg.surfaceRaised,
                  color: active ? tokens.color.text.inverse : tokens.color.text.primary,
                  border: `1px solid ${
                    active ? tokens.color.accent.violet : tokens.color.border.subtle
                  }`,
                  fontFamily: tokens.font.mono,
                  fontWeight: 700,
                  "&:hover": {
                    bgcolor: active
                      ? tokens.color.accent.violet
                      : tokens.color.bg.surfaceHover,
                    filter: active ? "brightness(0.95)" : "none",
                  },
                }}
              >
                ${value}
              </Button>
            );
          })}
        </Stack>

        <Stack spacing={1}>
          <Typography
            variant="overline"
            sx={{ color: tokens.color.text.muted, letterSpacing: 1.2 }}
          >
            Or custom amount (min ${MIN_CUSTOM})
          </Typography>
          <TextField
            value={custom}
            onChange={(e) => {
              const next = e.target.value.replace(/[^0-9.]/g, "");
              setCustom(next);
              setError(null);
            }}
            placeholder="e.g. 75"
            inputMode="decimal"
            sx={{
              maxWidth: 220,
              "& .MuiOutlinedInput-root": {
                bgcolor: tokens.color.bg.surfaceRaised,
                fontFamily: tokens.font.mono,
              },
            }}
            slotProps={{
              input: {
                startAdornment: (
                  <Typography
                    sx={{
                      color: tokens.color.text.muted,
                      mr: 0.5,
                      fontFamily: tokens.font.mono,
                    }}
                  >
                    $
                  </Typography>
                ),
              },
            }}
          />
          {useCustom && !customValid && (
            <Typography sx={{ fontSize: 12, color: tokens.color.accent.warning }}>
              Minimum top-up is ${MIN_CUSTOM}.
            </Typography>
          )}
        </Stack>

        {error && (
          <Alert
            severity="error"
            variant="outlined"
            sx={{
              border: `1px solid ${tokens.color.accent.danger}55`,
              color: tokens.color.text.primary,
              bgcolor: `${tokens.color.accent.danger}10`,
            }}
          >
            {error}
          </Alert>
        )}

        <Stack direction="row" alignItems="center" spacing={2}>
          <Button
            onClick={onPay}
            disabled={disabled}
            variant="contained"
            color="primary"
            size="large"
            startIcon={
              loading ? (
                <CircularProgress size={14} thickness={5} sx={{ color: "inherit" }} />
              ) : (
                <OpenInNewRounded sx={{ fontSize: 16 }} />
              )
            }
          >
            Pay {`$${amount.toFixed(2)}`} with Stripe
          </Button>
          <Typography sx={{ fontSize: 12, color: tokens.color.text.muted }}>
            Redirects to Stripe Checkout.
          </Typography>
        </Stack>
      </Stack>
    </Box>
  );
}

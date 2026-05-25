"use client";

// IdeaSubmitDialog — modal shown after a failed describeIdea call.
// Two flavours:
//   - "topup"  → wallet shortfall (INSUFFICIENT_FUNDS). CTA → /wallet.
//   - "error"  → generic failure. CTA → close + (optional) retry.
// We keep the language concrete: "$X to start" instead of "insufficient
// funds" so the visitor knows exactly what to do.

import { AccountBalanceWalletOutlined, CloseRounded } from "@mui/icons-material";
import {
  Box,
  Button,
  Dialog,
  DialogContent,
  IconButton,
  Stack,
  Typography,
} from "@mui/material";
import Link from "next/link";
import { formatMoney } from "../../lib/format";
import { tokens } from "../../theme";

export interface IdeaSubmitDialogProps {
  open: boolean;
  onClose: () => void;
  variant: "topup" | "error";
  title?: string;
  message: string;
  // Optional shortfall (USD). When present, we render it as a money chip.
  shortfallUSD?: number | null;
  // Optional top-up URL surfaced by the orchestrator in the error
  // extensions. Falls back to /wallet.
  topUpURL?: string | null;
  onRetry?: () => void;
}

export function IdeaSubmitDialog({
  open,
  onClose,
  variant,
  title,
  message,
  shortfallUSD,
  topUpURL,
  onRetry,
}: IdeaSubmitDialogProps) {
  const headline =
    title ??
    (variant === "topup" ? "Top up your wallet to launch" : "Could not start the build");
  const topUpHref = topUpURL || "/wallet";

  return (
    <Dialog
      open={open}
      onClose={onClose}
      maxWidth="xs"
      fullWidth
      slotProps={{
        paper: {
          sx: {
            bgcolor: tokens.color.bg.surface,
            border: `1px solid ${
              variant === "topup"
                ? `${tokens.color.accent.warning}66`
                : `${tokens.color.accent.danger}66`
            }`,
            backgroundImage: "none",
          },
        },
      }}
    >
      <DialogContent sx={{ p: 0 }}>
        <Stack
          direction="row"
          alignItems="center"
          justifyContent="space-between"
          sx={{
            px: 2.5,
            py: 1.5,
            borderBottom: `1px solid ${tokens.color.border.subtle}`,
          }}
        >
          <Typography
            sx={{
              fontFamily: tokens.font.mono,
              fontSize: 11,
              letterSpacing: 1.4,
              color:
                variant === "topup"
                  ? tokens.color.accent.warning
                  : tokens.color.accent.danger,
              textTransform: "uppercase",
            }}
          >
            {variant === "topup" ? "Wallet hold" : "Launch failed"}
          </Typography>
          <IconButton
            size="small"
            onClick={onClose}
            sx={{ color: tokens.color.text.secondary }}
          >
            <CloseRounded fontSize="small" />
          </IconButton>
        </Stack>
        <Stack spacing={2} sx={{ px: 2.5, py: 2.5 }}>
          <Typography sx={{ fontSize: 18, fontWeight: 800 }}>{headline}</Typography>
          <Typography sx={{ fontSize: 13.5, color: tokens.color.text.secondary, lineHeight: 1.5 }}>
            {message}
          </Typography>
          {variant === "topup" && typeof shortfallUSD === "number" && shortfallUSD > 0 && (
            <Box
              sx={{
                p: 1.5,
                bgcolor: tokens.color.bg.inset,
                border: `1px solid ${tokens.color.border.subtle}`,
                borderRadius: 1,
              }}
            >
              <Typography sx={{ fontSize: 11, color: tokens.color.text.muted, mb: 0.5 }}>
                Add at least
              </Typography>
              <Typography
                sx={{
                  fontFamily: tokens.font.mono,
                  fontSize: 22,
                  fontWeight: 800,
                  color: tokens.color.accent.violet,
                }}
              >
                {formatMoney(shortfallUSD)}
              </Typography>
            </Box>
          )}
          <Stack direction="row" spacing={1} justifyContent="flex-end">
            <Button
              size="small"
              variant="text"
              onClick={onClose}
              sx={{ color: tokens.color.text.secondary }}
            >
              Not now
            </Button>
            {variant === "topup" ? (
              <Button
                component={Link}
                href={topUpHref}
                variant="contained"
                color="primary"
                startIcon={<AccountBalanceWalletOutlined sx={{ fontSize: 18 }} />}
              >
                Top up first
              </Button>
            ) : (
              <Button
                variant="contained"
                color="primary"
                onClick={() => {
                  onClose();
                  onRetry?.();
                }}
              >
                Try again
              </Button>
            )}
          </Stack>
        </Stack>
      </DialogContent>
    </Dialog>
  );
}

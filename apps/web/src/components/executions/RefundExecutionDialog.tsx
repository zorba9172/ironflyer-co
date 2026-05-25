"use client";

// RefundExecutionDialog — issues a wallet credit-back for a terminal
// execution. Leaving amount blank refunds the unused reserve (the
// server-side default).

import {
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Stack,
  TextField,
  Typography,
} from "@mui/material";
import { useState } from "react";
import { extractErrorMessage } from "../../lib/errors";
import { useRefundExecutionMutation } from "../../lib/gql/__generated__";
import { tokens } from "../../theme";

export interface RefundExecutionDialogProps {
  open: boolean;
  executionID: string;
  unusedReserveUSD: number;
  onClose: () => void;
  onRefunded?: () => void;
}

export function RefundExecutionDialog({
  open,
  executionID,
  unusedReserveUSD,
  onClose,
  onRefunded,
}: RefundExecutionDialogProps) {
  const [amount, setAmount] = useState<string>("");
  const [reason, setReason] = useState<string>("Customer support refund");
  const [error, setError] = useState<string | null>(null);
  const [refund, { loading }] = useRefundExecutionMutation();

  const handleRefund = async () => {
    setError(null);
    const trimmed = amount.trim();
    let amountUSD: number | undefined;
    if (trimmed) {
      const n = Number(trimmed);
      if (!Number.isFinite(n) || n <= 0) {
        setError("Refund amount must be a positive number.");
        return;
      }
      amountUSD = n;
    }
    try {
      await refund({
        variables: {
          id: executionID,
          amountUSD,
          reason: reason.trim() || undefined,
        },
      });
      onRefunded?.();
      onClose();
    } catch (e) {
      setError(extractErrorMessage(e));
    }
  };

  return (
    <Dialog open={open} onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle sx={{ fontWeight: 800 }}>Refund execution</DialogTitle>
      <DialogContent>
        <Typography sx={{ color: tokens.color.text.secondary, fontSize: 13.5, mb: 2 }}>
          Issues a wallet credit-back tied to this execution. Leaving the
          amount blank refunds the unused reserve
          {` (≈ $${unusedReserveUSD.toFixed(2)})`}.
        </Typography>
        <Stack spacing={1.5}>
          <TextField
            autoFocus
            fullWidth
            size="small"
            label="Amount (USD)"
            type="number"
            inputProps={{ step: "0.01", min: 0 }}
            placeholder={`unused reserve ≈ $${unusedReserveUSD.toFixed(2)}`}
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
          />
          <TextField
            fullWidth
            size="small"
            label="Reason"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
          />
        </Stack>
        {error && (
          <Typography sx={{ mt: 1.5, color: tokens.color.accent.danger, fontSize: 12.5 }}>
            {error}
          </Typography>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} variant="text" sx={{ color: tokens.color.text.secondary }}>
          Cancel
        </Button>
        <Button
          onClick={handleRefund}
          disabled={loading}
          variant="contained"
          color="primary"
          startIcon={
            loading ? (
              <CircularProgress size={14} thickness={6} sx={{ color: tokens.color.text.inverse }} />
            ) : null
          }
        >
          Issue refund
        </Button>
      </DialogActions>
    </Dialog>
  );
}

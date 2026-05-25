"use client";

// StopExecutionDialog — terminal action; surfaces the reason field so
// the audit log carries a human note. Uses useStopExecutionMutation.

import {
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Typography,
} from "@mui/material";
import { useState } from "react";
import { extractErrorMessage } from "../../lib/errors";
import { useStopExecutionMutation } from "../../lib/gql/__generated__";
import { tokens } from "../../theme";

export interface StopExecutionDialogProps {
  open: boolean;
  executionID: string;
  onClose: () => void;
  onStopped?: () => void;
}

export function StopExecutionDialog({
  open,
  executionID,
  onClose,
  onStopped,
}: StopExecutionDialogProps) {
  const [reason, setReason] = useState("Operator stop");
  const [error, setError] = useState<string | null>(null);
  const [stop, { loading }] = useStopExecutionMutation();

  const handleStop = async () => {
    setError(null);
    try {
      await stop({ variables: { id: executionID, reason: reason.trim() || "Operator stop" } });
      onStopped?.();
      onClose();
    } catch (e) {
      setError(extractErrorMessage(e));
    }
  };

  return (
    <Dialog open={open} onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle sx={{ fontWeight: 800 }}>Stop execution?</DialogTitle>
      <DialogContent>
        <Typography sx={{ color: tokens.color.text.secondary, fontSize: 13.5, mb: 2 }}>
          Stopping moves this execution to <code>stopped</code>, releases the
          remaining wallet hold, and freezes any in-flight provider calls.
        </Typography>
        <TextField
          autoFocus
          fullWidth
          size="small"
          label="Reason"
          value={reason}
          onChange={(e) => setReason(e.target.value)}
        />
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
          onClick={handleStop}
          disabled={loading}
          variant="contained"
          sx={{
            bgcolor: tokens.color.accent.danger,
            color: tokens.color.text.primary,
            "&:hover": { bgcolor: tokens.color.accent.danger },
          }}
          startIcon={
            loading ? (
              <CircularProgress size={14} thickness={6} sx={{ color: tokens.color.text.primary }} />
            ) : null
          }
        >
          Stop execution
        </Button>
      </DialogActions>
    </Dialog>
  );
}

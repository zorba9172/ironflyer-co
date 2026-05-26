"use client";

// StopExecutionDialog — terminal action.
//
// Backed by sweetalert2 per the swal-everywhere migration: a single
// promise-based confirmDanger() with an input field for the reason,
// no MUI Dialog wrapper. The public API (`open` prop, `onClose`,
// `onStopped`) stays compatible with the existing /execution/[id]
// header so call sites need no change.

import { useEffect, useRef } from "react";
import { extractErrorMessage } from "../../lib/errors";
import { useStopExecutionMutation } from "../../lib/gql/__generated__";
import * as swal from "../../lib/swal";

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
  const [stop] = useStopExecutionMutation();
  // `firedFor` tracks the executionID we already opened the popup
  // for so a parent that keeps `open=true` while the page re-renders
  // doesn't trigger multiple sweetalert popups.
  const firedFor = useRef<string | null>(null);

  useEffect(() => {
    if (!open) {
      firedFor.current = null;
      return;
    }
    if (firedFor.current === executionID) return;
    firedFor.current = executionID;

    void (async () => {
      const res = await swal.fire({
        icon: "warning",
        title: "Stop execution?",
        html:
          "Stopping moves this execution to <code>stopped</code>, releases " +
          "the remaining wallet hold, and freezes any in-flight provider calls.",
        input: "text",
        inputLabel: "Reason",
        inputValue: "Operator stop",
        inputPlaceholder: "Why are you stopping?",
        showCancelButton: true,
        confirmButtonText: "Stop execution",
        cancelButtonText: "Cancel",
        showLoaderOnConfirm: true,
        preConfirm: async (value) => {
          try {
            const reason =
              typeof value === "string" && value.trim().length > 0
                ? value.trim()
                : "Operator stop";
            await stop({ variables: { id: executionID, reason } });
            return true;
          } catch (e) {
            swal.Swal.showValidationMessage(extractErrorMessage(e));
            return false;
          }
        },
      });

      if (res.isConfirmed) {
        onStopped?.();
      }
      onClose();
    })();
  }, [open, executionID, stop, onStopped, onClose]);

  return null;
}

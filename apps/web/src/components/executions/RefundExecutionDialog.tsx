"use client";

// RefundExecutionDialog — wallet credit-back for a terminal execution.
//
// Backed by sweetalert2. Two-field form (amount, reason) rendered as
// custom HTML inside the popup; preConfirm() validates the amount and
// triggers the refund mutation so the popup can show inline errors
// without leaving its lifecycle. Public API matches the legacy
// MUI Dialog so existing call sites keep working.

import { useEffect, useRef } from "react";
import { extractErrorMessage } from "../../lib/errors";
import { useRefundExecutionMutation } from "../../lib/gql/__generated__";
import * as swal from "../../lib/swal";

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
  const [refund] = useRefundExecutionMutation();
  const firedFor = useRef<string | null>(null);

  useEffect(() => {
    if (!open) {
      firedFor.current = null;
      return;
    }
    if (firedFor.current === executionID) return;
    firedFor.current = executionID;

    const unusedLabel = unusedReserveUSD.toFixed(2);

    void (async () => {
      const res = await swal.fire({
        icon: "question",
        title: "Refund execution",
        html:
          `<div style="text-align:left;font-size:13px;line-height:1.45;">` +
          `Issues a wallet credit-back tied to this execution. Leaving the ` +
          `amount blank refunds the unused reserve (≈ $${unusedLabel}).` +
          `</div>` +
          `<div style="display:flex;flex-direction:column;gap:10px;margin-top:14px;">` +
          `<input id="swal-refund-amount" class="swal2-input" type="number" step="0.01" min="0" ` +
          `placeholder="unused reserve ≈ $${unusedLabel}" style="margin:0;" />` +
          `<input id="swal-refund-reason" class="swal2-input" type="text" ` +
          `value="Customer support refund" placeholder="Reason" style="margin:0;" />` +
          `</div>`,
        focusConfirm: false,
        showCancelButton: true,
        confirmButtonText: "Issue refund",
        cancelButtonText: "Cancel",
        showLoaderOnConfirm: true,
        preConfirm: async () => {
          const root = swal.Swal.getHtmlContainer();
          if (!root) return false;
          const amountEl = root.querySelector<HTMLInputElement>(
            "#swal-refund-amount",
          );
          const reasonEl = root.querySelector<HTMLInputElement>(
            "#swal-refund-reason",
          );
          const amountStr = amountEl?.value.trim() ?? "";
          let amountUSD: number | undefined;
          if (amountStr) {
            const n = Number(amountStr);
            if (!Number.isFinite(n) || n <= 0) {
              swal.Swal.showValidationMessage(
                "Refund amount must be a positive number.",
              );
              return false;
            }
            amountUSD = n;
          }
          const reason = reasonEl?.value.trim() || undefined;
          try {
            await refund({
              variables: { id: executionID, amountUSD, reason },
            });
            return true;
          } catch (e) {
            swal.Swal.showValidationMessage(extractErrorMessage(e));
            return false;
          }
        },
      });

      if (res.isConfirmed) {
        onRefunded?.();
      }
      onClose();
    })();
  }, [open, executionID, unusedReserveUSD, refund, onRefunded, onClose]);

  return null;
}

"use client";

// useUIStore — process-global UI state for transient notifications.
//
// As of the sweetalert2 rollout the public API stays identical
// (`pushToast({ message, severity, ... })`) but the *render* is
// delegated to `lib/swal.toast()`. NotificationCenter is a no-op now
// so we don't paint a second toast tray on top of swal's. The store
// still keeps the toast list in memory for callers that may want to
// inspect or dismiss programmatically.

import { create } from "zustand";

export type ToastSeverity = "info" | "success" | "warning" | "error";

export interface Toast {
  id: string;
  message: string;
  severity: ToastSeverity;
  href?: string;
  actionLabel?: string;
  durationMs: number;
}

export interface ToastInput {
  message: string;
  severity?: ToastSeverity;
  href?: string;
  actionLabel?: string;
  durationMs?: number;
}

interface UIState {
  toasts: Toast[];
  pushToast(input: ToastInput): string;
  dismissToast(id: string): void;
  clearToasts(): void;
}

let nextId = 0;
function makeId(): string {
  nextId += 1;
  return `t${Date.now().toString(36)}-${nextId}`;
}

// severityToIcon — bridge our domain language into sweetalert2's icon
// vocabulary. Same four buckets so the mapping is 1:1.
function severityToIcon(s: ToastSeverity): "info" | "success" | "warning" | "error" {
  return s;
}

// renderViaSwal — fire the actual visible toast. Wrapped in a dynamic
// import so SSR never tries to load sweetalert2 (which touches
// document) and so the chunk only ships when a toast actually fires.
function renderViaSwal(t: Toast) {
  if (typeof window === "undefined") return;
  void import("../swal").then((swal) => {
    swal.toast(t.message, severityToIcon(t.severity), {
      timer: t.durationMs > 0 ? t.durationMs : undefined,
      ...(t.href && t.actionLabel
        ? {
            showConfirmButton: true,
            confirmButtonText: t.actionLabel,
            didClose: () => {
              // Best-effort same-tab navigation. The store has no
              // Next router reference; falling back to window.location
              // keeps the helper usable from non-React call sites.
              try {
                window.location.assign(t.href as string);
              } catch {
                // ignore
              }
            },
          }
        : {}),
    });
  });
}

export const useUIStore = create<UIState>((set) => ({
  toasts: [],
  pushToast: (input) => {
    const id = makeId();
    const toast: Toast = {
      id,
      message: input.message,
      severity: input.severity ?? "info",
      href: input.href,
      actionLabel: input.actionLabel,
      durationMs: input.durationMs ?? 3500,
    };
    set((state) => ({ toasts: [...state.toasts, toast] }));
    renderViaSwal(toast);
    return id;
  },
  dismissToast: (id) =>
    set((state) => ({ toasts: state.toasts.filter((t) => t.id !== id) })),
  clearToasts: () => set({ toasts: [] }),
}));

// Imperative shortcut for use inside non-React code (e.g. Apollo
// error link), avoiding a hook call.
export function pushToast(input: ToastInput): string {
  return useUIStore.getState().pushToast(input);
}

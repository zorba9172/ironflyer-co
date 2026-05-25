"use client";

// useUIStore — process-global UI state (toasts today, more later).
//
// Why zustand: any deeply-nested component can push a toast without
// prop-drilling a callback through five layers, and any component that
// renders the toast tray subscribes via selector so it only re-renders
// when the toast list changes — not on every keystroke elsewhere.
//
// The provider that paints toasts is <NotificationCenter /> at the
// app root; producers call useUIStore.getState().pushToast(...) or
// the convenience hook usePushToast().

import { create } from "zustand";

export type ToastSeverity = "info" | "success" | "warning" | "error";

export interface Toast {
  id: string;
  message: string;
  severity: ToastSeverity;
  // Optional CTA — when set, the toast renders an action button that
  // navigates the user to href (or fires onAction).
  href?: string;
  actionLabel?: string;
  // Auto-dismiss delay. 0 disables auto-dismiss (caller must dismiss
  // explicitly).
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

export const useUIStore = create<UIState>((set) => ({
  toasts: [],
  pushToast: (input) => {
    const id = makeId();
    set((state) => ({
      toasts: [
        ...state.toasts,
        {
          id,
          message: input.message,
          severity: input.severity ?? "info",
          href: input.href,
          actionLabel: input.actionLabel,
          durationMs: input.durationMs ?? 5000,
        },
      ],
    }));
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

"use client";

// NotificationCenter — kept as a mount point for backward
// compatibility, but rendering is now delegated to sweetalert2 via
// `lib/swal.toast()`. The MUI Snackbar tray was retired so we don't
// paint a second toast surface on top of swal's. This component is
// safe to leave mounted at the root; it returns null.
//
// If you need to enumerate active toasts (for tests, debug overlays,
// etc.) subscribe to `useUIStore` directly — the in-memory list
// still updates on every push/dismiss.

export function NotificationCenter() {
  return null;
}

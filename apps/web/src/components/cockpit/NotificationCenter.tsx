"use client";

// NotificationCenter — renders the toast stack from useUIStore.
//
// Mount once at the app root (providers.tsx). Each toast auto-dismisses
// after its durationMs, but the user can dismiss earlier via the close
// icon or by clicking the optional action CTA.

import CloseRounded from "@mui/icons-material/CloseRounded";
import {
  Alert,
  Button,
  IconButton,
  Snackbar,
  Stack,
} from "@mui/material";
import Link from "next/link";
import { useShallow } from "zustand/react/shallow";
import { useUIStore } from "../../lib/stores/uiStore";
import { tokens } from "../../theme";

export function NotificationCenter() {
  const toasts = useUIStore(useShallow((s) => s.toasts));
  const dismiss = useUIStore((s) => s.dismissToast);

  if (toasts.length === 0) return null;

  return (
    <Stack
      spacing={1}
      sx={{
        position: "fixed",
        bottom: { xs: 16, md: 24 },
        right: { xs: 16, md: 24 },
        zIndex: (theme) => theme.zIndex.snackbar,
        pointerEvents: "none",
      }}
    >
      {toasts.map((t) => (
        <Snackbar
          key={t.id}
          open
          autoHideDuration={t.durationMs > 0 ? t.durationMs : null}
          onClose={(_, reason) => {
            if (reason === "clickaway") return;
            dismiss(t.id);
          }}
          sx={{ position: "static", transform: "none", pointerEvents: "auto" }}
        >
          <Alert
            severity={t.severity}
            variant="filled"
            icon={false}
            action={
              <Stack direction="row" spacing={0.5} alignItems="center">
                {t.href && t.actionLabel && (
                  <Button
                    component={Link}
                    href={t.href}
                    size="small"
                    onClick={() => dismiss(t.id)}
                    sx={{
                      color: tokens.color.text.primary,
                      fontWeight: 700,
                    }}
                  >
                    {t.actionLabel}
                  </Button>
                )}
                <IconButton
                  size="small"
                  onClick={() => dismiss(t.id)}
                  aria-label="Dismiss"
                  sx={{ color: tokens.color.text.primary }}
                >
                  <CloseRounded fontSize="small" />
                </IconButton>
              </Stack>
            }
            sx={{
              minWidth: { xs: 280, sm: 360 },
              maxWidth: 480,
              bgcolor: tokens.color.bg.surfaceRaised,
              color: tokens.color.text.primary,
              border: `1px solid ${tokens.color.border.subtle}`,
              borderLeft: `3px solid ${severityColor(t.severity)}`,
              fontSize: 13.5,
              "& .MuiAlert-action": { ml: 1, p: 0, alignSelf: "center" },
            }}
          >
            {t.message}
          </Alert>
        </Snackbar>
      ))}
    </Stack>
  );
}

function severityColor(severity: "info" | "success" | "warning" | "error"): string {
  switch (severity) {
    case "success":
      return tokens.color.accent.success;
    case "warning":
      return tokens.color.accent.warning;
    case "error":
      return tokens.color.accent.danger;
    default:
      return tokens.color.accent.violet;
  }
}

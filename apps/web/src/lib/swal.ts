// swal — themed sweetalert2 facade for IronFlyer.
//
// All notification / confirm / error UI flows through this module so:
//   • the IronFlyer dark + violet theme is applied uniformly,
//   • call sites stay terse (one function call, no JSX),
//   • we can swap the implementation later without touching pages.
//
// Per CLAUDE.md → "VISUALIZATION-FIRST, CODE-FOR-PROS" and
// DESIGN_REFERENCE.md → "Design Reference is Law", every color here
// is sourced from `tokens.color.*`. No raw hex literals.

import Swal, { type SweetAlertIcon, type SweetAlertOptions } from "sweetalert2";
import { tokens } from "../theme";

let stylesInjected = false;

// Inject our scoped CSS overrides once per session. We use the
// `.swal2-ironflyer` class so theme rules never bleed into a third-
// party SweetAlert2 that someone might have on the page.
function ensureThemeStyles() {
  if (stylesInjected || typeof document === "undefined") return;
  stylesInjected = true;
  const style = document.createElement("style");
  style.id = "ironflyer-swal-theme";
  style.textContent = `
.swal2-container.swal2-ironflyer { z-index: 14000; }
.swal2-container.swal2-ironflyer .swal2-popup {
  background: ${tokens.color.bg.surface};
  color: ${tokens.color.text.primary};
  border: 1px solid ${tokens.color.border.strong};
  border-radius: 12px;
  box-shadow: 0 24px 64px ${tokens.color.bg.inset};
  font-family: ${tokens.font.family};
}
.swal2-container.swal2-ironflyer .swal2-title {
  color: ${tokens.color.text.primary};
  font-weight: 800;
  letter-spacing: -0.2px;
}
.swal2-container.swal2-ironflyer .swal2-html-container,
.swal2-container.swal2-ironflyer .swal2-content {
  color: ${tokens.color.text.secondary};
  font-size: 14px;
  line-height: 1.5;
  white-space: pre-line;
}
.swal2-container.swal2-ironflyer .swal2-actions { gap: 10px; }
.swal2-container.swal2-ironflyer .swal2-styled.swal2-confirm {
  background: linear-gradient(100deg, ${tokens.color.accent.coral}, ${tokens.color.brand.magenta} 52%, ${tokens.color.accent.purple});
  color: ${tokens.color.text.primary};
  border: 1px solid ${tokens.color.border.accent};
  font-weight: 800;
  padding: 9px 18px;
  border-radius: 8px;
  box-shadow: none;
}
.swal2-container.swal2-ironflyer .swal2-styled.swal2-confirm:focus {
  box-shadow: 0 0 0 3px ${tokens.color.accent.violet}55;
}
.swal2-container.swal2-ironflyer .swal2-styled.swal2-cancel,
.swal2-container.swal2-ironflyer .swal2-styled.swal2-deny {
  background: ${tokens.color.bg.surfaceRaised};
  color: ${tokens.color.text.secondary};
  border: 1px solid ${tokens.color.border.subtle};
  font-weight: 700;
  padding: 9px 18px;
  border-radius: 8px;
  box-shadow: none;
}
.swal2-container.swal2-ironflyer .swal2-styled.swal2-cancel:hover,
.swal2-container.swal2-ironflyer .swal2-styled.swal2-deny:hover {
  background: ${tokens.color.bg.surfaceHover};
  border-color: ${tokens.color.border.strong};
}
.swal2-container.swal2-ironflyer .swal2-input,
.swal2-container.swal2-ironflyer .swal2-textarea,
.swal2-container.swal2-ironflyer .swal2-select {
  background: ${tokens.color.bg.surfaceRaised};
  color: ${tokens.color.text.primary};
  border: 1px solid ${tokens.color.border.subtle};
  border-radius: 8px;
  font-family: ${tokens.font.mono};
}
.swal2-container.swal2-ironflyer .swal2-input:focus,
.swal2-container.swal2-ironflyer .swal2-textarea:focus {
  border-color: ${tokens.color.accent.violet};
  box-shadow: 0 0 0 3px ${tokens.color.accent.violet}33;
}
.swal2-container.swal2-ironflyer .swal2-validation-message {
  background: ${tokens.color.bg.surfaceRaised};
  color: ${tokens.color.accent.danger};
}
.swal2-container.swal2-ironflyer .swal2-icon {
  border-color: ${tokens.color.accent.violet};
  color: ${tokens.color.accent.violet};
}
.swal2-container.swal2-ironflyer .swal2-icon.swal2-success [class^='swal2-success-line'],
.swal2-container.swal2-ironflyer .swal2-icon.swal2-success .swal2-success-ring {
  background-color: transparent;
  border-color: ${tokens.color.accent.success};
}
.swal2-container.swal2-ironflyer .swal2-icon.swal2-success [class^='swal2-success-line'] {
  background-color: ${tokens.color.accent.success};
}
.swal2-container.swal2-ironflyer .swal2-icon.swal2-warning {
  border-color: ${tokens.color.brand.amber};
  color: ${tokens.color.brand.amber};
}
.swal2-container.swal2-ironflyer .swal2-icon.swal2-error {
  border-color: ${tokens.color.accent.danger};
  color: ${tokens.color.accent.danger};
}
.swal2-container.swal2-ironflyer .swal2-icon.swal2-error [class^='swal2-x-mark-line'] {
  background-color: ${tokens.color.accent.danger};
}
.swal2-container.swal2-ironflyer .swal2-icon.swal2-info {
  border-color: ${tokens.color.accent.sky};
  color: ${tokens.color.accent.sky};
}
.swal2-container.swal2-ironflyer .swal2-icon.swal2-question {
  border-color: ${tokens.color.accent.violet};
  color: ${tokens.color.accent.violet};
}
.swal2-container.swal2-ironflyer.swal2-toast .swal2-popup {
  background: ${tokens.color.bg.surfaceRaised};
  border-color: ${tokens.color.border.subtle};
  box-shadow: 0 8px 22px ${tokens.color.bg.inset};
  padding: 10px 14px;
}
.swal2-container.swal2-ironflyer .swal2-close {
  color: ${tokens.color.text.muted};
}
.swal2-container.swal2-ironflyer .swal2-close:hover {
  color: ${tokens.color.text.primary};
}
`;
  document.head.appendChild(style);
}

// SwalIron — a pre-mixed sweetalert2 instance with our class injected
// into the container so theming applies and the dialog never inherits
// upstream defaults.
const SwalIron = Swal.mixin({
  customClass: { container: "swal2-ironflyer" },
  buttonsStyling: false,
  reverseButtons: true,
  focusConfirm: false,
  scrollbarPadding: false,
});

// ------------------------------------------------------------------
// Public helpers
// ------------------------------------------------------------------

export type SwalIcon = SweetAlertIcon;
export type SwalOptions = SweetAlertOptions;

// fire — escape hatch. Prefer one of the typed helpers below.
export function fire(opts: SwalOptions) {
  ensureThemeStyles();
  return SwalIron.fire(opts);
}

// confirm — yes/no prompt; resolves with `true` on confirm.
export async function confirm(
  title: string,
  body?: string,
  opts?: Partial<SwalOptions>,
): Promise<boolean> {
  ensureThemeStyles();
  const res = await SwalIron.fire({
    icon: "question",
    title,
    text: body,
    showCancelButton: true,
    confirmButtonText: "Confirm",
    cancelButtonText: "Cancel",
    ...opts,
  });
  return res.isConfirmed === true;
}

// confirmDanger — destructive variant: red confirm button, "Delete"
// default text. Resolves true on confirm.
export async function confirmDanger(
  title: string,
  body?: string,
  opts?: Partial<SwalOptions>,
): Promise<boolean> {
  ensureThemeStyles();
  const res = await SwalIron.fire({
    icon: "warning",
    title,
    text: body,
    showCancelButton: true,
    confirmButtonText: opts?.confirmButtonText ?? "Delete",
    cancelButtonText: "Cancel",
    confirmButtonColor: tokens.color.accent.danger,
    ...opts,
  });
  return res.isConfirmed === true;
}

// error — blocking error popup with OK. Use for hard failures the
// user must acknowledge (auth lost, wallet shortfall, etc.).
export function error(title: string, body?: string, opts?: Partial<SwalOptions>) {
  ensureThemeStyles();
  return SwalIron.fire({
    icon: "error",
    title,
    text: body,
    confirmButtonText: "Got it",
    ...opts,
  });
}

// success — short confirmation popup. Auto-closes after 2s by default.
export function success(title: string, body?: string, opts?: Partial<SwalOptions>) {
  ensureThemeStyles();
  return SwalIron.fire({
    icon: "success",
    title,
    text: body,
    showConfirmButton: false,
    timer: 2000,
    timerProgressBar: true,
    ...opts,
  });
}

// info — neutral popup; useful for explainers.
export function info(title: string, body?: string, opts?: Partial<SwalOptions>) {
  ensureThemeStyles();
  return SwalIron.fire({
    icon: "info",
    title,
    text: body,
    confirmButtonText: "OK",
    ...opts,
  });
}

// toast — non-blocking toast in the top-right. Replaces the
// MUI Snackbar+Alert combo. Default 3.5s lifetime.
export function toast(
  message: string,
  icon: SwalIcon = "info",
  opts?: Partial<SwalOptions>,
) {
  ensureThemeStyles();
  return SwalIron.fire({
    toast: true,
    position: "top-end",
    showConfirmButton: false,
    timer: 3500,
    timerProgressBar: true,
    icon,
    title: message,
    didOpen: (el) => {
      el.addEventListener("mouseenter", SwalIron.stopTimer);
      el.addEventListener("mouseleave", SwalIron.resumeTimer);
    },
    ...opts,
  });
}

// prompt — single-line text input.
export async function prompt(
  title: string,
  placeholder?: string,
  opts?: Partial<SwalOptions>,
): Promise<string | null> {
  ensureThemeStyles();
  const res = await SwalIron.fire({
    title,
    input: "text",
    inputPlaceholder: placeholder,
    showCancelButton: true,
    confirmButtonText: "OK",
    cancelButtonText: "Cancel",
    ...opts,
  });
  if (res.isConfirmed) return (res.value as string) ?? "";
  return null;
}

// close — programmatically dismiss the open popup (e.g. when the
// underlying state was fixed by a background event).
export function close() {
  SwalIron.close();
}

export { SwalIron as Swal };
export default SwalIron;

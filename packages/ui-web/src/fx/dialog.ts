import Swal, { type SweetAlertResult } from 'sweetalert2';
import 'sweetalert2/dist/sweetalert2.min.css';
import { palette, modes } from '@ironflyer/design-tokens/brand';

// SweetAlert2 themed to the brand. Colors come from tokens; the popup tracks
// the active MUI color scheme so dialogs match dark/light.
function mode(): 'dark' | 'light' {
  if (typeof document === 'undefined') return 'dark';
  return document.documentElement.getAttribute('data-mui-color-scheme') === 'light' ? 'light' : 'dark';
}

function themed() {
  const m = modes[mode()];
  return Swal.mixin({
    background: m.surface,
    color: m.textPrimary,
    confirmButtonColor: palette.cobalt,
    cancelButtonColor: m.surfaceHover,
    buttonsStyling: true,
    reverseButtons: true,
    showClass: { popup: 'swal2-show' },
  });
}

export function confirmAction(opts: {
  title: string;
  text?: string;
  confirmText?: string;
  danger?: boolean;
}): Promise<boolean> {
  return themed()
    .fire({
      icon: opts.danger ? 'warning' : 'question',
      title: opts.title,
      text: opts.text,
      showCancelButton: true,
      confirmButtonText: opts.confirmText ?? 'Confirm',
      confirmButtonColor: opts.danger ? palette.rose : palette.cobalt,
    })
    .then((r: SweetAlertResult) => r.isConfirmed);
}

export function toast(title: string, icon: 'success' | 'error' | 'info' = 'success') {
  const m = modes[mode()];
  return Swal.fire({
    toast: true,
    position: 'bottom-end',
    icon,
    title,
    showConfirmButton: false,
    timer: 2600,
    timerProgressBar: true,
    background: m.surfaceRaised,
    color: m.textPrimary,
  });
}

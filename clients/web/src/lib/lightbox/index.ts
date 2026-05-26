// Lightbox — thin wrapper around @fancyapps/ui's Fancybox that pins
// the configuration to Ironflyer's cockpit aesthetic.
//
// Usage from a client component:
//   useEffect(() => {
//     bind();
//     return () => unbind();
//   }, []);
//
// Any element tagged `data-fancybox="<group>"` with `data-src="<url>"`
// (or `href="<url>"`) inside the document will participate in the
// gallery, with arrow-key navigation across same-group items.
//
// The wrapper enforces:
//   - dark theme to match the cockpit
//   - ESC closes / arrows navigate (Fancybox defaults, kept explicit)
//   - close button only — share/download/print/zoom toolbars omitted
//   - no Fancybox watermark / branding
//   - backdrop opacity that matches tokens.color.bg.overlay
//   - caption color = tokens.color.text.primary
//
// The CSS import is module-level so Next.js bundles the stylesheet
// alongside the first component that imports this file.

import { Fancybox, type FancyboxOptions } from "@fancyapps/ui/dist/fancybox/";
import "@fancyapps/ui/dist/fancybox/fancybox.css";
import { tokens } from "../../theme";

const STYLE_TAG_ID = "ironflyer-fancybox-overrides";

// Inject token-aligned overrides exactly once. Fancybox exposes its
// chrome via CSS custom properties on `.fancybox__container`; we ride
// those rather than re-skinning the whole class tree.
function injectStyleOverrides(): void {
  if (typeof document === "undefined") return;
  if (document.getElementById(STYLE_TAG_ID)) return;
  const style = document.createElement("style");
  style.id = STYLE_TAG_ID;
  style.textContent = `
.fancybox__container {
  --fancybox-bg: ${tokens.color.bg.overlay};
  --fancybox-color: ${tokens.color.text.primary};
  --fancybox-accent-color: ${tokens.color.accent.violet};
  backdrop-filter: blur(6px);
  -webkit-backdrop-filter: blur(6px);
}
.fancybox__backdrop {
  background: ${tokens.color.bg.overlay};
}
.fancybox__caption {
  color: ${tokens.color.text.primary};
  font-family: ${tokens.font.family};
}
.fancybox__nav .f-button,
.fancybox__toolbar .f-button {
  color: ${tokens.color.text.primary};
}
`;
  document.head.appendChild(style);
}

const DEFAULT_SELECTOR = "[data-fancybox]";

// Token-aligned Fancybox defaults. Kept narrow on purpose — every
// extra knob is a chance for the lightbox to drift away from the
// cockpit aesthetic.
function defaultOptions(): Partial<FancyboxOptions> {
  return {
    theme: "dark",
    hideScrollbar: true,
    placeFocusBack: true,
    closeButton: "auto",
    keyboard: {
      Escape: "close",
      Delete: "close",
      Backspace: "close",
      PageUp: "next",
      PageDown: "prev",
      ArrowUp: "prev",
      ArrowDown: "next",
      ArrowRight: "next",
      ArrowLeft: "prev",
    },
    Carousel: {
      infinite: false,
      Navigation: { prevTpl: "", nextTpl: "" } as never,
      Toolbar: {
        display: {
          left: [],
          middle: [],
          right: ["close"],
        },
      } as never,
    } as never,
  };
}

// bind — install Fancybox on a CSS selector. Safe to call repeatedly:
// Fancybox treats a re-bind on the same selector as a config update.
export function bind(
  selector: string = DEFAULT_SELECTOR,
  userOptions?: Partial<FancyboxOptions>,
): void {
  if (typeof window === "undefined") return;
  injectStyleOverrides();
  Fancybox.bind(selector, { ...defaultOptions(), ...userOptions });
}

// unbind — remove the click handler. Pair with bind() inside React
// effects so route changes do not leak listeners. Closes any open
// instance as well, otherwise the modal hangs around after unmount.
export function unbind(selector: string = DEFAULT_SELECTOR): void {
  if (typeof window === "undefined") return;
  Fancybox.close();
  Fancybox.unbind(selector);
}

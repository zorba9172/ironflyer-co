import type { MetadataRoute } from "next";
import { tokens } from "../../../packages/design-tokens";
import { SITE } from "../src/lib/seo/site";

// theme_color / background_color must be CSS color strings per the web
// manifest spec, so we read the hex value straight off the design token.
// The constitutional "no raw hex" rule governs inline component colors
// (sx / style) — not build-time metadata constants that need a CSS
// color string. Pulling from tokens keeps the manifest in lockstep with
// the rest of the dark surface system.
export default function manifest(): MetadataRoute.Manifest {
  return {
    name: SITE.name,
    short_name: SITE.name,
    description: SITE.description,
    start_url: "/",
    display: "standalone",
    theme_color: tokens.color.bg.base,
    background_color: tokens.color.bg.base,
    icons: [
      {
        src: "/brand/ironflyer-logo.svg",
        sizes: "any",
        type: "image/svg+xml",
        purpose: "any",
      },
      {
        src: "/brand/ironflyer-logo.svg",
        sizes: "any",
        type: "image/svg+xml",
        purpose: "maskable",
      },
    ],
  };
}

import "@fontsource/inter/400.css";
import "@fontsource/inter/500.css";
import "@fontsource/inter/600.css";
import "@fontsource/inter/700.css";
import "@fontsource/inter/900.css";
import "@fontsource/geist-mono/400.css";
import "@fontsource/geist-mono/500.css";
import "@fontsource/geist-mono/700.css";
import "swiper/css";
import "swiper/css/navigation";
import "swiper/css/pagination";
import type { Metadata, Viewport } from "next";
import { tokens } from "../../../packages/design-tokens";
import type { ReactNode } from "react";
import { CockpitFrame } from "../src/components/cockpit";
import { Providers } from "./providers";

export const metadata: Metadata = {
  title: "Ironflyer — Profitable Completed Execution",
  description:
    "Ironflyer is a paid AI execution engine that ships finished products end-to-end on prepaid wallet credits, with gates that block, patches that can be reviewed, and ProfitGuard before every expensive call.",
  openGraph: {
    title: "Ironflyer — Profitable Completed Execution",
    description:
      "A paid AI execution engine with gates that block, reviewable patches, live cost visibility, and prepaid wallet enforcement.",
    type: "website",
  },
  icons: { icon: "/favicon.svg" },
};

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  themeColor: tokens.color.bg.base,
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body>
        <Providers>
          <CockpitFrame>{children}</CockpitFrame>
        </Providers>
      </body>
    </html>
  );
}

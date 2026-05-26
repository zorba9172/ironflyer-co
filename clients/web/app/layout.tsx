import "@fontsource/inter/400.css";
import "@fontsource/inter/500.css";
import "@fontsource/inter/600.css";
import "@fontsource/inter/700.css";
import "@fontsource/inter/900.css";
import "@fontsource/geist-mono/400.css";
import "@fontsource/geist-mono/500.css";
import "@fontsource/geist-mono/700.css";
// swiper/css was imported here when the marketing carousel used Swiper.
// The Swiper rail was removed and no component now imports `swiper/*`
// (grep -rn "from 'swiper" returns nothing), so the global CSS payload
// would otherwise ship dead bytes on every route.
import type { Metadata, Viewport } from "next";
import { tokens } from "../../../packages/design-tokens";
import type { ReactNode } from "react";
import { ContentsquareScript } from "../src/components/analytics/Contentsquare";
import {
  GoogleTagManagerNoscript,
  GoogleTagManagerScript,
} from "../src/components/analytics/GoogleTagManager";
import { CockpitFrame } from "../src/components/cockpit";
import {
  OrganizationJsonLd,
  SoftwareApplicationJsonLd,
  WebSiteJsonLd,
} from "../src/components/seo/JsonLd";
import { SITE } from "../src/lib/seo/site";
import { Providers } from "./providers";

export const metadata: Metadata = {
  metadataBase: new URL(SITE.url),
  title: "Ironflyer — Profitable Completed Execution",
  description:
    "Ironflyer is a paid AI execution engine that ships finished products end-to-end on prepaid wallet credits, with gates that block, patches that can be reviewed, and ProfitGuard before every expensive call.",
  alternates: { canonical: "/" },
  openGraph: {
    title: "Ironflyer — Profitable Completed Execution",
    description:
      "A paid AI execution engine with gates that block, reviewable patches, live cost visibility, and prepaid wallet enforcement.",
    type: "website",
    locale: "en_US",
    url: SITE.url,
    siteName: SITE.name,
    images: [
      {
        url: SITE.ogImage,
        width: 1200,
        height: 630,
        alt: "Ironflyer — Profitable Completed Execution",
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    site: SITE.twitter,
    creator: SITE.twitter,
    title: "Ironflyer — Profitable Completed Execution",
    description:
      "A paid AI execution engine with gates that block, reviewable patches, live cost visibility, and prepaid wallet enforcement.",
    images: [SITE.ogImage],
  },
  icons: { icon: "/brand/ironflyer-logo.svg" },
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
        <OrganizationJsonLd />
        <WebSiteJsonLd />
        <SoftwareApplicationJsonLd />
        <GoogleTagManagerNoscript />
        <Providers>
          <CockpitFrame>{children}</CockpitFrame>
        </Providers>
        <GoogleTagManagerScript />
        <ContentsquareScript />
      </body>
    </html>
  );
}

// Site-wide SEO constants. The canonical hostname can be overridden at
// build time via NEXT_PUBLIC_SITE_URL so preview deploys and the
// production domain both resolve to absolute URLs without code edits.
export const SITE = {
  url: process.env.NEXT_PUBLIC_SITE_URL || "https://ironflyer.com",
  name: "Ironflyer",
  description:
    "Profitable Completed Execution. The paid AI engine that ships finished products through gates, patches, and a prepaid wallet.",
  twitter: "@ironflyer",
  // Referenced by Next.js Metadata / Twitter cards. The file lives under
  // clients/web/public/brand/og-image.png — do not generate it here.
  ogImage: "/brand/og-image.png",
} as const;

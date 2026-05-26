// Google Tag Manager wiring for the cockpit web app.
//
// The container ID is read from NEXT_PUBLIC_GTM_ID at build time. When the
// variable is empty (local dev, preview builds without analytics, etc.) both
// the head loader and the <noscript> fallback render nothing — there is no
// dead network request and no GTM iframe in the DOM.
//
// GA4 (gtag.js) is intentionally NOT loaded directly here. Configure the GA4
// property as a tag inside the GTM container instead — that keeps analytics,
// consent, and any future vendor swaps managed in one place and avoids the
// double-counting that happens when both gtag.js and GTM fire GA4 events.

import Script from "next/script";

const GTM_ID = process.env.NEXT_PUBLIC_GTM_ID;

export function GoogleTagManagerScript() {
  if (!GTM_ID) return null;
  return (
    <Script id="gtm-loader" strategy="afterInteractive">
      {`(function(w,d,s,l,i){w[l]=w[l]||[];w[l].push({'gtm.start':
new Date().getTime(),event:'gtm.js'});var f=d.getElementsByTagName(s)[0],
j=d.createElement(s),dl=l!='dataLayer'?'&l='+l:'';j.async=true;j.src=
'https://www.googletagmanager.com/gtm.js?id='+i+dl;f.parentNode.insertBefore(j,f);
})(window,document,'script','dataLayer','${GTM_ID}');`}
    </Script>
  );
}

export function GoogleTagManagerNoscript() {
  if (!GTM_ID) return null;
  return (
    <noscript>
      <iframe
        src={`https://www.googletagmanager.com/ns.html?id=${GTM_ID}`}
        height="0"
        width="0"
        style={{ display: "none", visibility: "hidden" }}
        title="Google Tag Manager"
      />
    </noscript>
  );
}

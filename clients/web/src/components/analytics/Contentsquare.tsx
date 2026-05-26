// Contentsquare (UXA) wiring for the cockpit web app.
//
// The tag ID is read from NEXT_PUBLIC_CONTENTSQUARE_ID at build time. Empty
// value short-circuits the loader so dev builds without session-replay stay
// silent. Contentsquare can also be fired as a tag inside the GTM container
// — when that is the case, leave NEXT_PUBLIC_CONTENTSQUARE_ID empty to avoid
// loading the UXA script twice.

import Script from "next/script";

const CS_ID = process.env.NEXT_PUBLIC_CONTENTSQUARE_ID;

export function ContentsquareScript() {
  if (!CS_ID) return null;
  return (
    <Script
      id="contentsquare-uxa"
      src={`https://t.contentsquare.net/uxa/${CS_ID}.js`}
      strategy="afterInteractive"
    />
  );
}

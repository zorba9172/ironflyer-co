import { SITE } from "../../../src/lib/seo/site";

const entries = [
  {
    date: "2026-05-27",
    version: "v22.4.7",
    title: "Mac pool dispatch lands behind IRONFLYER_MAC_POOL_ENABLED",
    body: "Runtime mobile builds can dispatch to the Mac pool when enabled, with wallet-aware allocation checks.",
  },
  {
    date: "2026-05-21",
    version: "v22.4.6",
    title: "Mobile cost lines split out from generic compute",
    body: "Wallet cost reporting now separates build minutes, emulator minutes, Mac workspace time and EAS credits.",
  },
  {
    date: "2026-05-15",
    version: "v22.4.5",
    title: "GateMobileBuild promoted from preview to default lane",
    body: "Mobile build validation now runs in the default finisher lane before deploy gates.",
  },
];

function escapeXml(value: string) {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&apos;");
}

export function GET() {
  const base = SITE.url.replace(/\/$/, "");
  const items = entries
    .map((entry) => {
      const link = `${base}/changelog#${entry.version}`;
      return `<item>
  <title>${escapeXml(`${entry.version}: ${entry.title}`)}</title>
  <link>${link}</link>
  <guid>${link}</guid>
  <pubDate>${new Date(`${entry.date}T09:00:00.000Z`).toUTCString()}</pubDate>
  <description>${escapeXml(entry.body)}</description>
</item>`;
    })
    .join("\n");

  return new Response(
    `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
  <title>${escapeXml(SITE.name)} changelog</title>
  <link>${base}/changelog</link>
  <description>${escapeXml("Release notes for IronFlyer gates, runtime, mobile, ledger and UI.")}</description>
  <language>en</language>
${items}
</channel>
</rss>`,
    {
      headers: {
        "content-type": "application/rss+xml; charset=utf-8",
      },
    },
  );
}

import { SITE } from "../../lib/seo/site";

// All JSON-LD emitters are server components: they render a single
// <script type="application/ld+json"> tag whose body is the schema.org
// payload for the page. Search engines (Google, Bing) parse these tags
// out-of-band to produce richer SERP entries, knowledge-graph cards,
// and sitelinks search boxes. No client JS required.

type JsonValue =
  | string
  | number
  | boolean
  | null
  | JsonValue[]
  | { [key: string]: JsonValue };

function JsonLdScript({ data }: { data: Record<string, JsonValue> }) {
  // JSON.stringify never emits `</` so the closing-tag attack vector
  // is theoretical here, but we still escape `<` defensively in case
  // a string field ever contains user-influenced markup.
  const json = JSON.stringify(data).replace(/</g, "\\u003c");
  return (
    <script
      type="application/ld+json"
      // eslint-disable-next-line react/no-danger
      dangerouslySetInnerHTML={{ __html: json }}
    />
  );
}

function twitterProfileUrl(): string {
  const handle = SITE.twitter.replace(/^@/, "");
  return `https://twitter.com/${handle}`;
}

export function OrganizationJsonLd() {
  const data: Record<string, JsonValue> = {
    "@context": "https://schema.org",
    "@type": "Organization",
    name: SITE.name,
    url: SITE.url,
    logo: `${SITE.url}/brand/ironflyer-logo.svg`,
    description: SITE.description,
    sameAs: [twitterProfileUrl()],
  };
  return <JsonLdScript data={data} />;
}

export function WebSiteJsonLd() {
  const data: Record<string, JsonValue> = {
    "@context": "https://schema.org",
    "@type": "WebSite",
    name: SITE.name,
    url: SITE.url,
    potentialAction: {
      "@type": "SearchAction",
      target: {
        "@type": "EntryPoint",
        urlTemplate: `${SITE.url}/search?q={search_term_string}`,
      },
      "query-input": "required name=search_term_string",
    },
  };
  return <JsonLdScript data={data} />;
}

export function SoftwareApplicationJsonLd() {
  // The Free tier is the documented entry point — Stripe-backed paid
  // tiers are layered on top via the prepaid wallet. We expose the
  // free price here so SERP rich results show a clear $0 onramp;
  // aggregateRating is intentionally omitted until real reviews exist
  // (Google flags fabricated ratings as a structured-data violation).
  const data: Record<string, JsonValue> = {
    "@context": "https://schema.org",
    "@type": "SoftwareApplication",
    name: SITE.name,
    description: SITE.description,
    url: SITE.url,
    applicationCategory: "DeveloperApplication",
    operatingSystem: "Web, macOS, Windows, Linux",
    offers: {
      "@type": "Offer",
      price: "0",
      priceCurrency: "USD",
    },
  };
  return <JsonLdScript data={data} />;
}

export interface BreadcrumbItem {
  name: string;
  url: string;
}

export function BreadcrumbJsonLd({ items }: { items: BreadcrumbItem[] }) {
  const data: Record<string, JsonValue> = {
    "@context": "https://schema.org",
    "@type": "BreadcrumbList",
    itemListElement: items.map((item, index) => ({
      "@type": "ListItem",
      position: index + 1,
      name: item.name,
      item: item.url.startsWith("http")
        ? item.url
        : `${SITE.url}${item.url.startsWith("/") ? "" : "/"}${item.url}`,
    })),
  };
  return <JsonLdScript data={data} />;
}

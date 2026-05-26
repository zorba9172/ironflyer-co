// app/vs/[slug]/page.tsx — SEO-targeted competitor comparison landing
// page. Server component. Thin: resolves params → competitor record →
// VsPageLayout. Implements generateStaticParams so all five
// comparisons prerender at build time, plus generateMetadata for
// per-slug title / description / OG / canonical. Renders FAQPage
// JSON-LD inline for organic SERP enhancements.

import { notFound } from "next/navigation";
import type { Metadata } from "next";
import { VsPageLayout } from "../../../src/components/vs/VsPageLayout";
import { competitors, getCompetitor } from "../../../src/components/vs/competitors";
import { buildFaq } from "../../../src/components/vs/faq";

const SITE_ORIGIN = "https://ironflyer.com";

export function generateStaticParams(): Array<{ slug: string }> {
  return competitors.map((c) => ({ slug: c.slug }));
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ slug: string }>;
}): Promise<Metadata> {
  const { slug } = await params;
  const c = getCompetitor(slug);
  if (!c) {
    return {
      title: "Comparison not found — Ironflyer",
      description: "The comparison page you requested does not exist.",
    };
  }

  const title = `Ironflyer vs ${c.name} — Production AI App Builder Compared`;
  const description = `Ironflyer vs ${c.name}: prepaid wallet, gate registry, and append-only ledger turn AI app generation into paid, gated, finished executions.`;
  const canonical = `${SITE_ORIGIN}/vs/${c.slug}`;

  return {
    title,
    description,
    alternates: { canonical },
    openGraph: {
      title,
      description,
      type: "website",
      url: canonical,
    },
    twitter: {
      card: "summary_large_image",
      title,
      description,
    },
  };
}

export default async function VsPage({
  params,
}: {
  params: Promise<{ slug: string }>;
}) {
  const { slug } = await params;
  const competitor = getCompetitor(slug);
  if (!competitor) {
    notFound();
  }

  const faq = buildFaq(competitor);
  const faqJsonLd = {
    "@context": "https://schema.org",
    "@type": "FAQPage",
    mainEntity: faq.map((item) => ({
      "@type": "Question",
      name: item.q,
      acceptedAnswer: {
        "@type": "Answer",
        text: item.a,
      },
    })),
  };

  return (
    <>
      <script
        type="application/ld+json"
        // JSON.stringify is sufficient escaping for embedded JSON-LD.
        dangerouslySetInnerHTML={{ __html: JSON.stringify(faqJsonLd) }}
      />
      <VsPageLayout competitor={competitor} />
    </>
  );
}

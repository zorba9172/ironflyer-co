import type { MetadataRoute } from "next";
import { SITE } from "../src/lib/seo/site";

// Marketing surfaces only — the cockpit, dashboard, wallet, projects,
// executions, settings, and auth screens are owner-scoped product
// chrome and are excluded from the sitemap (and disallowed in
// robots.txt). The /vs/* slugs are hardcoded so the sitemap can be
// generated at build time without filesystem traversal; Next.js will
// skip routes that 404 at request time.
type SitemapEntry = MetadataRoute.Sitemap[number];

const staticRoutes: Array<{
  path: string;
  priority: number;
  changeFrequency: SitemapEntry["changeFrequency"];
}> = [
  { path: "/", priority: 1.0, changeFrequency: "daily" },
  { path: "/product", priority: 0.9, changeFrequency: "weekly" },
  { path: "/pricing", priority: 0.9, changeFrequency: "weekly" },
  { path: "/solutions", priority: 0.8, changeFrequency: "weekly" },
  { path: "/enterprise", priority: 0.8, changeFrequency: "weekly" },
  { path: "/templates", priority: 0.7, changeFrequency: "weekly" },
  { path: "/resources", priority: 0.6, changeFrequency: "weekly" },
  { path: "/showcase", priority: 0.7, changeFrequency: "weekly" },
  { path: "/mobile", priority: 0.7, changeFrequency: "weekly" },
  { path: "/security", priority: 0.6, changeFrequency: "monthly" },
  { path: "/developers", priority: 0.7, changeFrequency: "weekly" },
  { path: "/changelog", priority: 0.6, changeFrequency: "weekly" },
  { path: "/blog", priority: 0.6, changeFrequency: "weekly" },
];

const versusSlugs = ["lovable", "bolt", "replit-agent", "v0", "base44"];

export default function sitemap(): MetadataRoute.Sitemap {
  const now = new Date();
  const base = SITE.url.replace(/\/$/, "");

  const staticEntries: MetadataRoute.Sitemap = staticRoutes.map((route) => ({
    url: `${base}${route.path === "/" ? "" : route.path}` || base,
    lastModified: now,
    changeFrequency: route.changeFrequency,
    priority: route.priority,
  }));

  const versusEntries: MetadataRoute.Sitemap = versusSlugs.map((slug) => ({
    url: `${base}/vs/${slug}`,
    lastModified: now,
    changeFrequency: "weekly",
    priority: 0.6,
  }));

  return [...staticEntries, ...versusEntries];
}

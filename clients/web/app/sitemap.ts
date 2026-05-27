import type { MetadataRoute } from "next";
import { SITE } from "../src/lib/seo/site";

// Marketing surfaces only — the cockpit, dashboard, wallet, projects,
// executions, settings, and auth screens are owner-scoped product
// chrome and are excluded from the sitemap (and disallowed in
// robots.txt). The public surface intentionally mirrors the Base44-like
// flow: home, templates, solutions, pricing, resources, enterprise.
type SitemapEntry = MetadataRoute.Sitemap[number];

const staticRoutes: Array<{
  path: string;
  priority: number;
  changeFrequency: SitemapEntry["changeFrequency"];
}> = [
  { path: "/", priority: 1.0, changeFrequency: "daily" },
  { path: "/templates", priority: 0.9, changeFrequency: "weekly" },
  { path: "/solutions", priority: 0.8, changeFrequency: "weekly" },
  { path: "/pricing", priority: 0.9, changeFrequency: "weekly" },
  { path: "/resources", priority: 0.7, changeFrequency: "weekly" },
  { path: "/enterprise", priority: 0.8, changeFrequency: "weekly" },
  { path: "/vscode", priority: 0.8, changeFrequency: "weekly" },
  { path: "/appsec", priority: 0.75, changeFrequency: "weekly" },
  { path: "/compare", priority: 0.75, changeFrequency: "weekly" },
  { path: "/product", priority: 0.8, changeFrequency: "weekly" },
  { path: "/developers", priority: 0.7, changeFrequency: "weekly" },
  { path: "/mobile", priority: 0.7, changeFrequency: "weekly" },
  { path: "/security", priority: 0.75, changeFrequency: "weekly" },
  { path: "/showcase", priority: 0.7, changeFrequency: "weekly" },
  { path: "/blog", priority: 0.6, changeFrequency: "weekly" },
  { path: "/changelog", priority: 0.6, changeFrequency: "weekly" },
  { path: "/vs/lovable", priority: 0.65, changeFrequency: "weekly" },
  { path: "/vs/bolt", priority: 0.65, changeFrequency: "weekly" },
  { path: "/vs/base44", priority: 0.65, changeFrequency: "weekly" },
  { path: "/vs/replit-agent", priority: 0.65, changeFrequency: "weekly" },
  { path: "/vs/v0", priority: 0.65, changeFrequency: "weekly" },
];

export default function sitemap(): MetadataRoute.Sitemap {
  const now = new Date();
  const base = SITE.url.replace(/\/$/, "");

  const staticEntries: MetadataRoute.Sitemap = staticRoutes.map((route) => ({
    url: `${base}${route.path === "/" ? "" : route.path}` || base,
    lastModified: now,
    changeFrequency: route.changeFrequency,
    priority: route.priority,
  }));

  return staticEntries;
}

import type { MetadataRoute } from 'next';

const SITE_URL = process.env.NEXT_PUBLIC_SITE_URL ?? 'https://ironflyer.dev';

// Marketing surface only. The dashboard, project workspaces, and API
// endpoints are deliberately omitted — they're either auth-gated or
// noisy parameterised URLs that don't deserve to be crawled.
type Entry = {
  path: string;
  changeFrequency: MetadataRoute.Sitemap[number]['changeFrequency'];
  priority: number;
};

const ROUTES: Entry[] = [
  { path: '/', changeFrequency: 'weekly', priority: 1.0 },
  { path: '/pricing', changeFrequency: 'weekly', priority: 0.9 },
  { path: '/product', changeFrequency: 'weekly', priority: 0.8 },
  { path: '/solutions', changeFrequency: 'weekly', priority: 0.8 },
  { path: '/templates', changeFrequency: 'weekly', priority: 0.7 },
  { path: '/security', changeFrequency: 'monthly', priority: 0.6 },
  { path: '/enterprise', changeFrequency: 'monthly', priority: 0.6 },
  { path: '/status', changeFrequency: 'always', priority: 0.5 },
  // Docs site (Agent O).
  { path: '/docs', changeFrequency: 'weekly', priority: 0.85 },
  { path: '/docs/getting-started', changeFrequency: 'weekly', priority: 0.85 },
  { path: '/docs/concepts/finisher-gates', changeFrequency: 'monthly', priority: 0.7 },
  { path: '/docs/concepts/patches', changeFrequency: 'monthly', priority: 0.7 },
  { path: '/docs/concepts/budget', changeFrequency: 'monthly', priority: 0.7 },
  { path: '/docs/concepts/runtime-sandbox', changeFrequency: 'monthly', priority: 0.7 },
  { path: '/docs/api/auth', changeFrequency: 'monthly', priority: 0.7 },
  { path: '/docs/api/projects', changeFrequency: 'monthly', priority: 0.7 },
  { path: '/docs/api/patches', changeFrequency: 'monthly', priority: 0.7 },
  { path: '/docs/api/budget', changeFrequency: 'monthly', priority: 0.7 },
  { path: '/docs/api/webhooks', changeFrequency: 'monthly', priority: 0.7 },
  { path: '/docs/api/deploy', changeFrequency: 'monthly', priority: 0.7 },
  { path: '/docs/api/runtime', changeFrequency: 'monthly', priority: 0.7 },
  { path: '/docs/sdk', changeFrequency: 'monthly', priority: 0.7 },
  { path: '/docs/vscode-extension', changeFrequency: 'monthly', priority: 0.7 },
  { path: '/docs/cli', changeFrequency: 'monthly', priority: 0.6 },
  // Changelog + blog (Agent O).
  { path: '/changelog', changeFrequency: 'weekly', priority: 0.75 },
  { path: '/blog', changeFrequency: 'weekly', priority: 0.75 },
  { path: '/blog/why-finished-products', changeFrequency: 'monthly', priority: 0.65 },
  { path: '/blog/the-eight-gates', changeFrequency: 'monthly', priority: 0.65 },
  { path: '/blog/budget-transparency', changeFrequency: 'monthly', priority: 0.65 },
  { path: '/blog/launching-vscode-extension', changeFrequency: 'monthly', priority: 0.65 },
];

export default function sitemap(): MetadataRoute.Sitemap {
  const lastModified = new Date();
  return ROUTES.map((r) => ({
    url: `${SITE_URL}${r.path}`,
    lastModified,
    changeFrequency: r.changeFrequency,
    priority: r.priority,
  }));
}

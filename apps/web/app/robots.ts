import type { MetadataRoute } from 'next';

// SITE_URL falls back to ironflyer.dev so robots.txt is still useful when
// the env var is missing in preview deploys.
const SITE_URL = process.env.NEXT_PUBLIC_SITE_URL ?? 'https://ironflyer.dev';

// Next.js metadata route — emits /robots.txt. We allow the marketing
// surface and disallow the authenticated app + API to keep them out of
// indexes; the sitemap link helps Google find the public pages quickly.
export default function robots(): MetadataRoute.Robots {
  return {
    rules: [
      {
        userAgent: '*',
        allow: ['/', '/sitemap.xml'],
        disallow: ['/app/', '/api/', '/projects/'],
      },
    ],
    sitemap: `${SITE_URL}/sitemap.xml`,
    host: SITE_URL,
  };
}

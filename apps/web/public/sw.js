// Ironflyer service worker — app-shell + asset caching for mobile and PWA.
//
// Strategy:
//   * cache-first for static assets (icons, fonts, opengraph image, /icon, /_next/static)
//   * network-first for HTML navigations (so updates land immediately)
//   * never touch /api/*, /app/projects/*, SSE/WS, or Next data
//   * `cacheName: 'ironflyer-v1'` — bump the version when shipping breaking caching rules
//   * responds to `{ type: 'SKIP_WAITING' }` messages to allow tab refresh flow

const CACHE_NAME = 'ironflyer-v1';
const SHELL_URLS = ['/', '/app', '/manifest.webmanifest'];

const STATIC_ASSET_RE = /\.(?:png|jpe?g|gif|webp|svg|ico|woff2?|ttf|otf|eot|css|js)$/i;
const STATIC_PATHS = ['/icon', '/apple-icon', '/opengraph-image', '/_next/static/'];

self.addEventListener('install', (event) => {
  event.waitUntil(
    (async () => {
      const cache = await caches.open(CACHE_NAME);
      try {
        await cache.addAll(SHELL_URLS);
      } catch {
        /* offline / blocked — install still proceeds */
      }
      self.skipWaiting();
    })(),
  );
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    (async () => {
      const keys = await caches.keys();
      await Promise.all(keys.filter((k) => k !== CACHE_NAME).map((k) => caches.delete(k)));
      await self.clients.claim();
    })(),
  );
});

self.addEventListener('message', (event) => {
  if (event.data && event.data.type === 'SKIP_WAITING') {
    self.skipWaiting();
  }
});

function isStaticAsset(url) {
  if (STATIC_ASSET_RE.test(url.pathname)) return true;
  return STATIC_PATHS.some((p) => url.pathname.startsWith(p));
}

function isUncacheable(url) {
  if (url.pathname.startsWith('/api/')) return true;
  if (url.pathname.startsWith('/app/projects/')) return true;
  if (url.pathname.startsWith('/_next/data/')) return true;
  if (url.pathname.includes('/stream')) return true;
  if (url.pathname.includes('/chat')) return true;
  return false;
}

self.addEventListener('fetch', (event) => {
  const req = event.request;
  if (req.method !== 'GET') return;

  const url = new URL(req.url);
  if (url.origin !== self.location.origin) return;
  if (isUncacheable(url)) return;

  // Cache-first for static assets.
  if (isStaticAsset(url)) {
    event.respondWith(
      (async () => {
        const cache = await caches.open(CACHE_NAME);
        const cached = await cache.match(req);
        if (cached) return cached;
        try {
          const res = await fetch(req);
          if (res && res.ok) cache.put(req, res.clone()).catch(() => {});
          return res;
        } catch (err) {
          return cached || Response.error();
        }
      })(),
    );
    return;
  }

  // Network-first for HTML navigations and everything else cacheable.
  const acceptsHtml = req.headers.get('accept')?.includes('text/html');
  if (acceptsHtml || req.mode === 'navigate') {
    event.respondWith(
      (async () => {
        const cache = await caches.open(CACHE_NAME);
        try {
          const res = await fetch(req);
          if (res && res.ok) cache.put(req, res.clone()).catch(() => {});
          return res;
        } catch (err) {
          const cached = await cache.match(req);
          if (cached) return cached;
          const shell = await cache.match('/');
          return shell || Response.error();
        }
      })(),
    );
  }
});

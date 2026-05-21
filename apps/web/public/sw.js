// Ironflyer minimal service worker — app-shell caching for take-away on mobile.
// Skips API/streaming routes; never caches /api/* so live data stays live.

const SHELL = 'ironflyer-shell-v2';
const SHELL_URLS = ['/', '/app', '/manifest.webmanifest'];

self.addEventListener('install', (e) => {
  e.waitUntil((async () => {
    const cache = await caches.open(SHELL);
    try { await cache.addAll(SHELL_URLS); } catch {}
    self.skipWaiting();
  })());
});

self.addEventListener('activate', (e) => {
  e.waitUntil((async () => {
    const keys = await caches.keys();
    await Promise.all(keys.filter((k) => k !== SHELL).map((k) => caches.delete(k)));
    self.clients.claim();
  })());
});

self.addEventListener('fetch', (e) => {
  const url = new URL(e.request.url);
  // Never touch API / SSE / WS / Next data.
  if (url.pathname.startsWith('/api/') ||
      url.pathname.startsWith('/_next/data/') ||
      url.pathname.includes('/stream') ||
      url.pathname.includes('/chat')) {
    return;
  }
  // Stale-while-revalidate for app shell.
  e.respondWith((async () => {
    const cache = await caches.open(SHELL);
    const cached = await cache.match(e.request);
    const fetchPromise = fetch(e.request).then((res) => {
      if (res && res.ok && e.request.method === 'GET' && url.origin === self.location.origin) {
        cache.put(e.request, res.clone()).catch(() => {});
      }
      return res;
    }).catch(() => cached);
    return cached || fetchPromise;
  })());
});

// Kill-switch service worker. The old PWA build registered a SW at
// /sw.js; serving 404 here would leave stale SWs hanging around in
// users' browsers. This empty SW unregisters itself on activate and
// hard-reloads open clients so they pick up the no-SW state.
self.addEventListener("install", () => self.skipWaiting());
self.addEventListener("activate", async () => {
  await self.registration.unregister();
  const clients = await self.clients.matchAll({ type: "window" });
  for (const client of clients) client.navigate(client.url);
});

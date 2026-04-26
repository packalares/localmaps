// Self-unregistering service worker (kill switch).
//
// An earlier version of LocalMaps shipped a PWA service worker that
// cached /api/tiles/* responses as full-file 200 OK entries. That
// breaks pmtiles HTTP Range requests (cached 200 with full body is
// served back for a bytes=0-16383 request, and pmtiles errors out).
//
// This no-op replacement unregisters itself and clears all caches so
// previously-installed clients recover on their next reload.
self.addEventListener("install", (event) => {
  event.waitUntil(self.skipWaiting());
});
self.addEventListener("activate", (event) => {
  event.waitUntil(
    (async () => {
      try {
        const names = await caches.keys();
        await Promise.all(names.map((n) => caches.delete(n)));
      } catch (_) {}
      try {
        await self.registration.unregister();
      } catch (_) {}
      const clients = await self.clients.matchAll({ type: "window" });
      clients.forEach((c) => c.navigate(c.url));
    })(),
  );
});
self.addEventListener("fetch", () => {});

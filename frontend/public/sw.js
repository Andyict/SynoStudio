// Minimal service worker: makes the app installable and caches the app shell.
// Photos and API calls always go to the network (they are private and change often).
const CACHE = 'sa-shell-v1'
self.addEventListener('install', (e) => {
  e.waitUntil(caches.open(CACHE).then((c) => c.addAll(['/'])).then(() => self.skipWaiting()))
})
self.addEventListener('activate', (e) => {
  e.waitUntil(caches.keys().then((ks) => Promise.all(ks.filter((k) => k !== CACHE).map((k) => caches.delete(k)))).then(() => self.clients.claim()))
})
self.addEventListener('fetch', (e) => {
  const u = new URL(e.request.url)
  if (e.request.method !== 'GET' || u.origin !== location.origin) return
  if (u.pathname.startsWith('/assets/')) {
    e.respondWith(caches.match(e.request).then((r) => r || fetch(e.request).then((resp) => {
      const copy = resp.clone()
      caches.open(CACHE).then((c) => c.put(e.request, copy))
      return resp
    })))
  } else if (e.request.mode === 'navigate' && !u.pathname.startsWith('/studio') && !u.pathname.startsWith('/console')) {
    e.respondWith(fetch(e.request).catch(() => caches.match('/')))
  }
})

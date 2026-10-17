// Fledger Force PWA — Service Worker (Offline Support & Asset Caching)
const CACHE_NAME = 'fledger-force-v2.9';
const STATIC_ASSETS = [
  './',
  './index.html',
  './style.css',
  './app.js',
  './force-client.js',
  './manifest.json',
  './icon.svg',
  './icon-192.png',
  './icon-512.png'
];

// Install: Cache core app shell
self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => {
      return cache.addAll(STATIC_ASSETS).catch((err) => {
        console.warn('[PWA SW] Pre-caching partial failure:', err);
      });
    }).then(() => self.skipWaiting())
  );
});

// Activate: Clean up old caches & take immediate control
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) => {
      return Promise.all(
        keys.map((key) => {
          if (key !== CACHE_NAME) {
            return caches.delete(key);
          }
        })
      );
    }).then(() => self.clients.claim())
  );
});

// Fetch: Strategy depending on request type
self.addEventListener('fetch', (event) => {
  const req = event.request;
  const url = new URL(req.url);

  // 1. API Requests (/v1/*) or Non-GET: Network-First with Offline JSON response
  if (url.pathname.includes('/v1/') || req.method !== 'GET') {
    event.respondWith(
      fetch(req).catch(() => {
        return new Response(
          JSON.stringify({
            status: 'offline',
            error: 'OFFLINE_MODE',
            message: 'Perangkat sedang berada di luar jangkauan sinyal internet (Mode Offline PWA).'
          }),
          {
            status: 503,
            headers: { 'Content-Type': 'application/json' }
          }
        );
      })
    );
    return;
  }

  // 2. Navigation & HTML: Network-First to guarantee immediate UI updates
  const isHtml = req.mode === 'navigate' || (req.headers.get('accept') && req.headers.get('accept').includes('text/html'));
  if (isHtml) {
    event.respondWith(
      fetch(req).then((networkRes) => {
        if (networkRes && networkRes.status === 200) {
          const clone = networkRes.clone();
          caches.open(CACHE_NAME).then((cache) => cache.put(req, clone));
        }
        return networkRes;
      }).catch(() => {
        return caches.match('./index.html');
      })
    );
    return;
  }

  // 3. Static Assets (CSS, JS, Icons): Stale-While-Revalidate
  event.respondWith(
    caches.match(req).then((cached) => {
      const fetchPromise = fetch(req).then((networkRes) => {
        if (networkRes && networkRes.status === 200) {
          const clone = networkRes.clone();
          caches.open(CACHE_NAME).then((cache) => cache.put(req, clone));
        }
        return networkRes;
      }).catch(() => null);

      return cached || fetchPromise;
    })
  );
});

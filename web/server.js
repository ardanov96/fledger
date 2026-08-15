// web/server.js
// =============================================================================
// FMCG Wallet Web Dashboard — Sprint 20 (Portfolio Sprint 3 / Fase 8 frontend)
//
// Lightweight Node static file server (zero dependencies, no framework).
// Serves the SPA from web/public/ and proxies /v1/* requests to the backend API.
//
// Why no framework?
// - We need a portfolio-quality demo, not a production-grade SPA framework
// - Vanilla JS keeps the demo understandable + minimal attack surface
// - For production-grade SPA, swap this with Next.js / Nuxt / SvelteKit
//
// Run:
//   node web/server.js
//   PORT=3000 API_BASE_URL=https://fmcg-wallet-demo.fly.dev node web/server.js
// =============================================================================

const http = require('http');
const fs = require('fs');
const path = require('path');
const url = require('url');

const PORT = parseInt(process.env.PORT || '3000', 10);
const API_BASE_URL = process.env.API_BASE_URL || 'http://localhost:8080';
const PUBLIC_DIR = path.join(__dirname, 'public');

const MIME_TYPES = {
  '.html': 'text/html; charset=utf-8',
  '.css':  'text/css; charset=utf-8',
  '.js':   'application/javascript; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.svg':  'image/svg+xml',
  '.png':  'image/png',
  '.ico':  'image/x-icon',
};

// =============================================================================
// Reverse proxy → backend API (/v1/*)
// =============================================================================
function proxyToApi(req, res) {
  const targetUrl = API_BASE_URL + req.url;
  console.log(`[proxy] ${req.method} ${req.url} → ${targetUrl}`);

  // Manual proxy (no external deps)
  const parsed = url.parse(targetUrl);
  const lib = parsed.protocol === 'https:' ? require('https') : require('http');

  const proxyReq = lib.request({
    hostname: parsed.hostname,
    port: parsed.port || (parsed.protocol === 'https:' ? 443 : 80),
    path: parsed.path,
    method: req.method,
    headers: { ...req.headers, host: parsed.host },
  }, (proxyRes) => {
    res.writeHead(proxyRes.statusCode, proxyRes.headers);
    proxyRes.pipe(res);
  });

  proxyReq.on('error', (err) => {
    console.error(`[proxy error] ${err.message}`);
    res.writeHead(502, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ error: { code: 'BAD_GATEWAY', message: err.message } }));
  });

  req.pipe(proxyReq);
}

// =============================================================================
// Static file server (web/public/*)
// =============================================================================
function serveStatic(req, res) {
  let pathname = url.parse(req.url).pathname;
  if (pathname === '/' || pathname === '') pathname = '/index.html';

  // Prevent directory traversal
  const safePath = path.normalize(pathname).replace(/^(\.\.[\/\\])+/, '');
  const filePath = path.join(PUBLIC_DIR, safePath);

  if (!filePath.startsWith(PUBLIC_DIR)) {
    res.writeHead(403, { 'Content-Type': 'text/plain' });
    res.end('403 Forbidden');
    return;
  }

  fs.stat(filePath, (err, stats) => {
    if (err || !stats.isFile()) {
      // SPA fallback — serve index.html for client-side routing
      fs.readFile(path.join(PUBLIC_DIR, 'index.html'), (err2, data) => {
        if (err2) {
          res.writeHead(404, { 'Content-Type': 'text/plain' });
          res.end('404 Not Found');
          return;
        }
        res.writeHead(200, { 'Content-Type': MIME_TYPES['.html'] });
        res.end(data);
      });
      return;
    }
    const ext = path.extname(filePath).toLowerCase();
    res.writeHead(200, {
      'Content-Type': MIME_TYPES[ext] || 'application/octet-stream',
      'Cache-Control': 'no-cache',
    });
    fs.createReadStream(filePath).pipe(res);
  });
}

// =============================================================================
// Request router
// =============================================================================
const server = http.createServer((req, res) => {
  console.log(`[req] ${req.method} ${req.url}`);
  if (req.url.startsWith('/v1/') || req.url === '/healthz' || req.url === '/readyz') {
    proxyToApi(req, res);
  } else {
    serveStatic(req, res);
  }
});

server.listen(PORT, () => {
  console.log(`🚀 FMCG Wallet Web Dashboard`);
  console.log(`   Listening: http://localhost:${PORT}`);
  console.log(`   Proxy /v1/* → ${API_BASE_URL}`);
  console.log(`   SPA files:  ${PUBLIC_DIR}`);
});

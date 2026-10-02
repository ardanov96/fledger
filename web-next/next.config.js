/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // Sprint 47: Backend at cmd/api listens on :8080. Next.js dev server
  // proxies /v1/* there. Production: same — API and frontend share origin
  // via reverse proxy (same pattern as the current web/ server.js).
  async rewrites() {
    return [
      { source: '/v1/:path*', destination: 'http://localhost:8080/v1/:path*' },
    ];
  },
  // Enable strict security headers.
  async headers() {
    return [
      {
        source: '/(.*)',
        headers: [
          { key: 'X-Frame-Options', value: 'DENY' },
          { key: 'X-Content-Type-Options', value: 'nosniff' },
          { key: 'Referrer-Policy', value: 'strict-origin-when-cross-origin' },
        ],
      },
    ];
  },
};

module.exports = nextConfig;
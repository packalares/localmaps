/**
 * Next.js config for the LocalMaps UI.
 *
 * In development, we rewrite gateway-bound paths to NEXT_PUBLIC_GATEWAY_URL
 * (default http://localhost:8080) so the UI dev server can talk to the Go
 * gateway without CORS friction. In production the UI is served by the
 * gateway itself, so rewrites are inert.
 */

const gateway =
  process.env.NEXT_PUBLIC_GATEWAY_URL?.replace(/\/$/, "") ||
  "http://localhost:8080";

/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  poweredByHeader: false,
  async rewrites() {
    return [
      { source: "/api/:path*", destination: `${gateway}/api/:path*` },
      { source: "/og/:path*", destination: `${gateway}/og/:path*` },
      { source: "/embed", destination: `${gateway}/embed` },
    ];
  },
  async headers() {
    // The service worker must be served from /sw.js with a root scope and
    // a short cache-control so updates propagate. Browsers refuse to
    // register a SW whose scope exceeds its served path unless
    // Service-Worker-Allowed permits it.
    return [
      {
        source: "/sw.js",
        headers: [
          { key: "Service-Worker-Allowed", value: "/" },
          { key: "Cache-Control", value: "public, max-age=0, must-revalidate" },
          { key: "Content-Type", value: "application/javascript; charset=utf-8" },
        ],
      },
      {
        source: "/manifest.webmanifest",
        headers: [
          { key: "Cache-Control", value: "public, max-age=3600" },
        ],
      },
    ];
  },
};

export default nextConfig;

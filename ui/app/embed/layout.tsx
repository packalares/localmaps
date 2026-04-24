import type { Metadata } from "next";

/**
 * Embed-mode layout. Intentionally minimal — we do NOT mount the global
 * `ReactQueryProvider` / `ThemeProvider` / `Toaster` stack from
 * `app/layout.tsx` because:
 *
 *   - third-party iframes must not rely on any in-app auth session or
 *     cookie-bearing fetches (see docs/08-security.md §Embedding safety);
 *   - the embed view is deliberately read-only, so there's nothing for
 *     TanStack Query to manage;
 *   - the root layout already renders <html>/<body>, so this layout only
 *     needs to pass children through verbatim.
 *
 * The Next.js App Router will still compose this on top of `app/layout.tsx`;
 * the root layout is the one place we can't avoid inheriting from, but any
 * heavy providers live as client components so they no-op without store
 * subscribers.
 */
export const metadata: Metadata = {
  title: "LocalMaps embed",
  description: "Embedded LocalMaps map view.",
  // Discourage search engines from indexing embed URLs directly; the host
  // page is the canonical index target.
  robots: { index: false, follow: false },
};

export default function EmbedLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  // Plain fragment — the embed page is self-contained and sizes to the
  // iframe's bounding box via its own root element.
  return <>{children}</>;
}

import type { MetadataRoute } from "next";

/**
 * Next.js dynamic webmanifest route.
 *
 * Served at /manifest.webmanifest. The brand color default matches
 * `ui.brandColor` in `docs/07-config-schema.md` (#0ea5e9, sky-500). A
 * later polish can swap this for a loader that reads the user's current
 * settings — doing so now would add a runtime gateway dependency to a
 * route the browser hits before auth is resolved.
 */
const BRAND_COLOR = "#0ea5e9";
const BACKGROUND_COLOR = "#0b1220";

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "LocalMaps",
    short_name: "LocalMaps",
    description:
      "Deliver a production-quality self-hosted maps platform that replaces the most-used Google Maps features using only open data + open-source engines.",
    start_url: "/",
    scope: "/",
    display: "standalone",
    orientation: "portrait",
    theme_color: BRAND_COLOR,
    background_color: BACKGROUND_COLOR,
    lang: "en",
    dir: "ltr",
    categories: ["navigation", "travel", "maps"],
    icons: [
      {
        src: "/icons/icon-192.png",
        sizes: "192x192",
        type: "image/png",
        purpose: "any",
      },
      {
        src: "/icons/icon-512.png",
        sizes: "512x512",
        type: "image/png",
        purpose: "any",
      },
      {
        src: "/icons/icon-maskable-192.png",
        sizes: "192x192",
        type: "image/png",
        purpose: "maskable",
      },
      {
        src: "/icons/icon-maskable-512.png",
        sizes: "512x512",
        type: "image/png",
        purpose: "maskable",
      },
    ],
  };
}

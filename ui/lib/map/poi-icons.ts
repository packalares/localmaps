"use client";

import type maplibregl from "maplibre-gl";
import { CATEGORY_DESCRIPTORS } from "@/components/chrome/category-descriptors";

const ICON_SIZE = 32;
const ICON_PREFIX = "lm-poi-";

/** Mirrors mapstyle.go's `"lm-" + c.id` (c.id = "poi-<key>"). */
export function poiIconNameFor(key: string): string {
  return ICON_PREFIX + key;
}

const descriptorByKey: ReadonlyMap<string, (typeof CATEGORY_DESCRIPTORS)[number]> =
  new Map(CATEGORY_DESCRIPTORS.map((d) => [d.key, d]));

/**
 * Register the placeholder + bulk-rasterise all category icons + wire
 * the `styleimagemissing` safety net. Re-callable: every step
 * checks `hasImage()` before touching the atlas because MapLibre
 * wipes images on `setStyle()`.
 *
 * Why three steps?
 *
 *   1. Placeholder: MapLibre evaluates `icon-image: "lm-poi-food"`
 *      the moment a POI tile starts to render, BEFORE our async
 *      SVG rasterisation finishes. Without a placeholder MapLibre
 *      logs a "could not be loaded" warning for every category on
 *      every first paint. A 1px transparent SDF mask reserves the
 *      name so the warning never fires; the real icon swaps in via
 *      `updateImage()` once rasterisation completes.
 *
 *   2. Bulk rasterise: walk every descriptor, kick off the canvas
 *      decode, swap in via `updateImage()`. Same code path as before,
 *      just with the placeholder pre-seeded.
 *
 *   3. `styleimagemissing` fallback: catches anything the style
 *      references but we forgot to register (future-proofing). The
 *      handler is idempotent — it bails when the descriptor is
 *      unknown or the icon is already on the map.
 */
export function registerPoiIcons(map: maplibregl.Map): void {
  for (const desc of CATEGORY_DESCRIPTORS) {
    const name = poiIconNameFor(desc.key);
    if (!map.hasImage(name)) {
      // 1×1 transparent SDF placeholder, swapped out below.
      const blank: maplibregl.StyleImageInterface = {
        width: 1,
        height: 1,
        data: new Uint8Array([0]),
        // The cast is to keep the StyleImageInterface optional sdf
        // field happy when downstream lib types are mismatched.
      } as maplibregl.StyleImageInterface;
      map.addImage(name, blank, { sdf: true, pixelRatio: 2 });
    }
    void loadPoiIcon(map, name, desc.iconPath);
  }

  // Idempotent listener registration: maplibre's `.on` permits the
  // same handler to be added twice, so dedupe via a typed marker on
  // the map instance.
  const marker = "__lmPoiMissingWired";
  if (!(map as unknown as Record<string, boolean>)[marker]) {
    (map as unknown as Record<string, boolean>)[marker] = true;
    map.on("styleimagemissing", (event: { id: string }) => {
      if (!event.id.startsWith(ICON_PREFIX)) return;
      const key = event.id.slice(ICON_PREFIX.length);
      const desc = descriptorByKey.get(key as never);
      if (!desc) return;
      if (map.hasImage(event.id)) return;
      void loadPoiIcon(map, event.id, desc.iconPath);
    });
  }
}

async function loadPoiIcon(
  map: maplibregl.Map,
  name: string,
  pathHtml: string,
): Promise<void> {
  const data = await rasterizeSvgPath(pathHtml, ICON_SIZE);
  if (!data) return;
  // Use updateImage when a placeholder is already in place; addImage
  // would throw on duplicate. updateImage is a no-op rather than a
  // throw when the name isn't there yet, so it's safe in both modes.
  if (map.hasImage(name)) {
    map.updateImage(name, data);
  } else {
    map.addImage(name, data, { sdf: true, pixelRatio: 2 });
  }
}

async function rasterizeSvgPath(
  pathHtml: string,
  size: number,
): Promise<ImageData | null> {
  if (typeof document === "undefined") return null;
  const canvas = document.createElement("canvas");
  canvas.width = size;
  canvas.height = size;
  const ctx = canvas.getContext("2d");
  if (!ctx) return null;
  const svg =
    `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" viewBox="0 0 24 24" ` +
    `fill="none" stroke="white" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round">${pathHtml}</svg>`;
  const blob = new Blob([svg], { type: "image/svg+xml" });
  const url = URL.createObjectURL(blob);
  try {
    const img = await loadImage(url);
    ctx.clearRect(0, 0, size, size);
    ctx.drawImage(img, 0, 0, size, size);
    return ctx.getImageData(0, 0, size, size);
  } finally {
    URL.revokeObjectURL(url);
  }
}

function loadImage(src: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const img = new Image();
    img.onload = () => resolve(img);
    img.onerror = (err) => reject(err);
    img.src = src;
  });
}

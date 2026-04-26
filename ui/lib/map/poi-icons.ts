"use client";

import type maplibregl from "maplibre-gl";
import { CATEGORY_DESCRIPTORS } from "@/components/chrome/category-descriptors";

const ICON_SIZE = 32;
const ICON_PREFIX = "lm-poi-";

/** Mirrors mapstyle.go's `"lm-" + c.id` (c.id = "poi-<key>"). */
export function poiIconNameFor(key: string): string {
  return ICON_PREFIX + key;
}

/**
 * Rasterise each category's inline SVG into an SDF mask and register it
 * on the live map under `lm-poi-<key>`. The basemap's POI symbol layers
 * reference these names via `icon-image`, and `icon-color: #ffffff` paints
 * them white on top of the coloured circle backdrop.
 *
 * Re-callable: hasImage() guards re-registration after a `setStyle()`
 * because MapLibre wipes the image atlas alongside the style.
 */
export function registerPoiIcons(map: maplibregl.Map): void {
  for (const desc of CATEGORY_DESCRIPTORS) {
    const name = poiIconNameFor(desc.key);
    if (map.hasImage(name)) continue;
    void loadPoiIcon(map, name, desc.iconPath);
  }
}

async function loadPoiIcon(
  map: maplibregl.Map,
  name: string,
  pathHtml: string,
): Promise<void> {
  const data = await rasterizeSvgPath(pathHtml, ICON_SIZE);
  if (!data) return;
  if (map.hasImage(name)) return;
  map.addImage(name, data, { sdf: true, pixelRatio: 2 });
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

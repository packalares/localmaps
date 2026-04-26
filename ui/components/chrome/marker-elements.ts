/**
 * DOM-element factories for the two custom MapLibre markers we ship:
 *
 *   - `makeDroppedPinElement()` — small grey circular badge with a tiny
 *     white pin glyph, used for plain-point clicks (replaces the default
 *     red teardrop).
 *   - `makeChipMarkerElement({ iconPath, label })` — red Google-style
 *     teardrop pin with a category glyph (white path) on top of a
 *     27×27 circle, plus an optional white name-pill positioned to
 *     the right of the marker. Used for chip-search per-result pins.
 *
 * Both functions return a fresh `HTMLDivElement` so the caller can hand
 * it to `new maplibregl.Marker({ element })`. Owners are responsible for
 * `marker.remove()` on cleanup; this module only builds the DOM.
 */

/**
 * Build the dropped-pin marker element. The visual is a 28px dark-grey
 * circle with a 2px white ring and a small white pin SVG centered
 * inside. A subtle drop shadow gives it depth so it reads against any
 * basemap.
 *
 * The element is positioned with the bottom-center pixel as its
 * anchor — pass `anchor: "bottom"` when constructing the Marker so the
 * tip of the visual sits on the click coordinate.
 */
export function makeDroppedPinElement(): HTMLDivElement {
  const wrap = document.createElement("div");
  wrap.className = "localmaps-dropped-pin";
  wrap.style.cssText = [
    "width: 28px",
    "height: 28px",
    "border-radius: 9999px",
    "background: #3c4043",
    "border: 2px solid #ffffff",
    "box-shadow: 0 1px 3px rgba(0,0,0,0.35), 0 0 0 0.5px rgba(0,0,0,0.15)",
    "display: flex",
    "align-items: center",
    "justify-content: center",
    "color: #ffffff",
    "cursor: pointer",
    "pointer-events: auto",
  ].join(";");
  wrap.innerHTML = [
    '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" ',
    'viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" ',
    'stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">',
    '<path d="M12 21s-7-7.58-7-12a7 7 0 1 1 14 0c0 4.42-7 12-7 12z"/>',
    '<circle cx="12" cy="9" r="2.5"/>',
    "</svg>",
  ].join("");
  return wrap;
}

/**
 * Options for the chip-marker factory below. `iconPath` is the SVG
 * `<path d>` for the white category glyph. `label` is the place name
 * shown in a white pill to the right of the marker; pass an empty
 * string to skip the label. `color` defaults to Google's red-600
 * (`#dc2626`) and is exposed so callers can tint the bubble per
 * descriptor when we ever drop the unified-red look.
 */
export interface ChipMarkerOptions {
  /** SVG path string for the inline category glyph (24×24 viewBox). */
  iconPath: string;
  /** Place name shown in the white pill. Empty string hides the pill. */
  label?: string;
  /** Marker fill colour. Defaults to `#dc2626` (Google red-600). */
  color?: string;
}

/**
 * Build a chip-search per-result marker. The visual is a red teardrop
 * 24px wide × 28px tall, with a small white category glyph centered on
 * the circle of the bubble and a 1.5px white outline so it pops on
 * green / grey basemaps. Anchor at `bottom` so the tip of the pin sits
 * on the POI coordinate.
 *
 * When `label` is non-empty, a white pill (max 140px, truncated) is
 * rendered immediately to the right of the marker — same pattern as
 * Google Maps' restaurant chip results in `3.png`. The pill uses the
 * same `pointer-events: none` so a click goes through to the marker
 * SVG underneath.
 */
export function makeChipMarkerElement(
  opts: ChipMarkerOptions,
): HTMLDivElement {
  const color = opts.color ?? "#dc2626";
  const wrap = document.createElement("div");
  wrap.className = "localmaps-chip-marker";
  wrap.style.cssText = [
    "display: flex",
    "align-items: flex-end",
    "gap: 4px",
    "cursor: pointer",
    "pointer-events: auto",
  ].join(";");

  // Marker SVG. The teardrop body is a single path; the inner circle
  // is a separate <circle> and the glyph rides on top with `fill:
  // white`. `filter: drop-shadow(...)` gives the subtle depth shadow
  // we want without introducing a wrapper div.
  const svg = [
    '<svg xmlns="http://www.w3.org/2000/svg" width="24" height="28" ',
    'viewBox="0 0 24 28" aria-hidden="true" ',
    'style="filter: drop-shadow(0 1px 2px rgba(0,0,0,0.35)); flex-shrink: 0;">',
    // Teardrop bubble: circle on top with a triangular tail at bottom-center.
    `<path d="M12 27.5 L4.2 17 A9 9 0 1 1 19.8 17 Z" fill="${color}" stroke="#ffffff" stroke-width="1.5" stroke-linejoin="round"/>`,
    // Inner glyph viewBox is 24×24 native; we shift it to (4,2) and
    // scale down to ~14×14 so it nests inside the 18px circle.
    '<g transform="translate(5 3) scale(0.58)" fill="#ffffff" stroke="#ffffff" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">',
    `<g fill="none">${opts.iconPath}</g>`,
    "</g>",
    "</svg>",
  ].join("");
  const marker = document.createElement("span");
  marker.style.cssText = [
    "display: inline-block",
    "width: 24px",
    "height: 28px",
    "line-height: 0",
  ].join(";");
  marker.innerHTML = svg;
  wrap.appendChild(marker);

  // Optional name pill. Mirrors `3.png`'s right-side label.
  if (opts.label) {
    const pill = document.createElement("span");
    pill.className = "localmaps-chip-marker__label";
    pill.style.cssText = [
      "display: inline-block",
      "max-width: 140px",
      "padding: 2px 6px",
      "border-radius: 4px",
      "background: #ffffff",
      "color: #1f2937", // neutral-800
      "font: 500 11px/1.2 ui-sans-serif, system-ui, -apple-system, sans-serif",
      "white-space: nowrap",
      "overflow: hidden",
      "text-overflow: ellipsis",
      "box-shadow: 0 1px 2px rgba(0,0,0,0.18)",
      "pointer-events: none",
      "margin-bottom: 4px",
    ].join(";");
    pill.textContent = opts.label;
    wrap.appendChild(pill);
  }

  return wrap;
}

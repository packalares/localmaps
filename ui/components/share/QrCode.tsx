"use client";

import { QRCodeSVG } from "qrcode.react";

/**
 * Renders an SVG QR code for the supplied URL.
 *
 * Size defaults to 256 px — the same value as the documented
 * `share.qrCodeSizePx` setting (see docs/07-config-schema.md). Callers
 * should pass the live value read from settings once Phase 6 ships a
 * client-side settings hook; until then the prop default matches the
 * documented default, so operators see no drift.
 *
 * We deliberately render as SVG (not canvas) so:
 *  - the markup is a DOM element testable with @testing-library (the
 *    vitest suite asserts `getByRole("img")` on the container);
 *  - users can copy/save the QR losslessly at any size;
 *  - printing/embed scenarios keep vector crispness.
 */
export interface QrCodeProps {
  /** The payload the QR encodes. Typically a short URL. */
  value: string;
  /** Pixel size (both dimensions). Default 256 from share.qrCodeSizePx. */
  size?: number;
  /** Optional class applied to the wrapper — lets callers frame/pad. */
  className?: string;
  /** ARIA label for the whole figure. */
  ariaLabel?: string;
}

export function QrCode({
  value,
  size = 256,
  className,
  ariaLabel = "QR code",
}: QrCodeProps) {
  // An empty value produces an empty SVG. We still render the wrapper
  // so the layout doesn't jump when callers flip from long-URL → short-
  // URL; a placeholder message keeps the block accessible.
  if (!value) {
    return (
      <div
        role="img"
        aria-label="QR code placeholder"
        className={className}
        style={{ width: size, height: size }}
        data-testid="qr-empty"
      />
    );
  }
  return (
    <div
      role="img"
      aria-label={ariaLabel}
      className={className}
      data-testid="qr-code"
    >
      <QRCodeSVG
        value={value}
        size={size}
        level="M"
        includeMargin
        // Keep contrasts above WCAG thresholds in either light or dark
        // theme — the dialog background is `bg-background` so we rely
        // on the default black-on-white which meets AAA in both.
        bgColor="#ffffff"
        fgColor="#000000"
      />
    </div>
  );
}

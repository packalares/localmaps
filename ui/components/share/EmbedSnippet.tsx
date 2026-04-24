"use client";

import { useState } from "react";
import { Copy, Check } from "lucide-react";
import { Button } from "@/components/ui/button";

/**
 * Builds the canonical `<iframe>` snippet for embedding a LocalMaps view
 * on an external site and offers a one-click copy affordance.
 *
 * The snippet's src is the `/embed` path served by the gateway (see
 * contracts/openapi.yaml — tag: share). We prefer the short URL when
 * available because it is substantially more copy-friendly and the
 * gateway resolves either form to the same underlying view state.
 */
export interface EmbedSnippetProps {
  /** The `/embed?…` URL (or short URL) to wrap in the iframe. */
  src: string;
  /** Width attribute on the iframe. Default 600 — Google Maps parity. */
  width?: number;
  /** Height attribute. Default 450 — Google Maps parity. */
  height?: number;
}

/** Pure helper: build the exact snippet string a user copies. */
export function buildEmbedSnippet(
  src: string,
  width = 600,
  height = 450,
): string {
  // Keep the string on a single line so pasting into editors that
  // strip newlines (Slack, WhatsApp, older CMS fields) still yields
  // valid HTML.
  return `<iframe src="${src}" width="${width}" height="${height}" frameborder="0" loading="lazy" referrerpolicy="no-referrer"></iframe>`;
}

export function EmbedSnippet({
  src,
  width = 600,
  height = 450,
}: EmbedSnippetProps) {
  const [copied, setCopied] = useState(false);
  const snippet = buildEmbedSnippet(src, width, height);

  async function doCopy() {
    try {
      await navigator.clipboard.writeText(snippet);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      // Clipboard permissions may be denied (iframe, Firefox-headless,
      // etc). Fall back to the old-school selection trick.
      const ta = document.createElement("textarea");
      ta.value = snippet;
      document.body.appendChild(ta);
      ta.select();
      try {
        document.execCommand("copy");
        setCopied(true);
        window.setTimeout(() => setCopied(false), 1500);
      } finally {
        ta.remove();
      }
    }
  }

  return (
    <div className="flex flex-col gap-2">
      <label
        htmlFor="embed-snippet"
        className="text-xs font-medium text-muted-foreground"
      >
        Paste this into your site:
      </label>
      <textarea
        id="embed-snippet"
        readOnly
        value={snippet}
        rows={3}
        className="w-full resize-none rounded-md border border-border bg-muted px-3 py-2 font-mono text-xs"
        onFocus={(e) => e.currentTarget.select()}
      />
      <div className="flex items-center justify-between">
        <span className="text-xs text-muted-foreground">
          {width}×{height} px
        </span>
        <Button
          type="button"
          size="sm"
          variant="secondary"
          onClick={doCopy}
          aria-label={copied ? "Snippet copied to clipboard" : "Copy snippet"}
        >
          {copied ? (
            <>
              <Check className="mr-2 h-4 w-4" aria-hidden="true" />
              Copied!
            </>
          ) : (
            <>
              <Copy className="mr-2 h-4 w-4" aria-hidden="true" />
              Copy
            </>
          )}
        </Button>
      </div>
    </div>
  );
}

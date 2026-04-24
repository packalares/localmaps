"use client";

import { useState } from "react";
import { Check, Copy } from "lucide-react";
import { Button } from "@/components/ui/button";

/**
 * Body of the "Link" tab inside <ShareDialog />. Shows the current
 * long URL + a copy button, plus a "Make short link" action that POSTs
 * to `/api/links` and surfaces the resulting short URL for copying.
 *
 * Copy status is announced via an aria-live region so screen-reader
 * users are notified without stealing focus — matches the a11y
 * expectations in docs/09-testing.md.
 */
export interface LinkPanelProps {
  longUrl: string;
  shortUrl: string | null;
  onCreateShort: () => void;
  creating: boolean;
  errored: boolean;
}

export function LinkPanel(props: LinkPanelProps) {
  const [copiedWhich, setCopiedWhich] = useState<"long" | "short" | null>(null);

  async function copyText(text: string, which: "long" | "short") {
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      // Fallback for envs that block `navigator.clipboard` (older
      // Safari, restricted iframes). Keeps the UX identical.
      const ta = document.createElement("textarea");
      ta.value = text;
      document.body.appendChild(ta);
      ta.select();
      try {
        document.execCommand("copy");
      } finally {
        ta.remove();
      }
    }
    setCopiedWhich(which);
    window.setTimeout(
      () => setCopiedWhich((c) => (c === which ? null : c)),
      1500,
    );
  }

  return (
    <div className="flex flex-col gap-3">
      <span role="status" aria-live="polite" className="sr-only">
        {copiedWhich ? "Copied to clipboard" : ""}
      </span>

      <CopyRow
        label="Link"
        value={props.longUrl}
        copied={copiedWhich === "long"}
        onCopy={() => copyText(props.longUrl, "long")}
      />

      {props.shortUrl ? (
        <CopyRow
          label="Short link"
          value={props.shortUrl}
          copied={copiedWhich === "short"}
          onCopy={() => copyText(props.shortUrl!, "short")}
        />
      ) : (
        <Button
          type="button"
          variant="secondary"
          onClick={props.onCreateShort}
          disabled={props.creating || !props.longUrl}
        >
          {props.creating ? "Creating…" : "Make short link"}
        </Button>
      )}

      {props.errored && (
        <p className="text-xs text-destructive" role="alert">
          Couldn&apos;t create a short link. Please retry.
        </p>
      )}
    </div>
  );
}

function CopyRow(props: {
  label: string;
  value: string;
  copied: boolean;
  onCopy: () => void;
}) {
  return (
    <div className="flex flex-col gap-1">
      <label className="text-xs font-medium text-muted-foreground">
        {props.label}
      </label>
      <div className="flex gap-2">
        <input
          type="text"
          readOnly
          value={props.value}
          onFocus={(e) => e.currentTarget.select()}
          className="flex-1 rounded-md border border-border bg-muted px-3 py-2 text-sm"
          aria-label={props.label}
        />
        <Button
          type="button"
          size="sm"
          variant="secondary"
          onClick={props.onCopy}
          aria-label={
            props.copied
              ? `${props.label} copied to clipboard`
              : `Copy ${props.label.toLowerCase()}`
          }
        >
          {props.copied ? (
            <>
              <Check className="mr-1 h-4 w-4" aria-hidden="true" /> Copied!
            </>
          ) : (
            <>
              <Copy className="mr-1 h-4 w-4" aria-hidden="true" /> Copy
            </>
          )}
        </Button>
      </div>
    </div>
  );
}

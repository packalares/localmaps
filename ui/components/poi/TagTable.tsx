"use client";

import { useState } from "react";
import { ChevronDown, Tag as TagIcon } from "lucide-react";
import { cn } from "@/lib/utils";

export interface TagTableProps {
  tags: Record<string, string> | undefined | null;
  className?: string;
}

/**
 * Collapsed-by-default table of raw OSM/Overture tags. Aimed at power
 * users and support debugging; hidden by default so the pane stays
 * compact.
 */
export function TagTable({ tags, className }: TagTableProps) {
  const [open, setOpen] = useState(false);

  const entries = Object.entries(tags ?? {}).sort(([a], [b]) =>
    a.localeCompare(b),
  );
  if (entries.length === 0) return null;

  return (
    <section className={cn("px-4 py-3", className)} aria-label="Raw tags">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        aria-controls="poi-tag-table-body"
        className="flex w-full items-center justify-between gap-2 rounded-md py-1 text-left text-sm text-muted-foreground hover:bg-muted focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <span className="flex items-center gap-2">
          <TagIcon className="h-4 w-4 shrink-0" aria-hidden="true" />
          <span>Raw tags ({entries.length})</span>
        </span>
        <ChevronDown
          className={cn(
            "h-4 w-4 shrink-0 transition-transform",
            open && "rotate-180",
          )}
          aria-hidden="true"
        />
      </button>

      {open && (
        <div
          id="poi-tag-table-body"
          className="mt-2 overflow-hidden rounded border border-border"
        >
          <table className="w-full text-left text-xs">
            <tbody>
              {entries.map(([k, v]) => (
                <tr key={k} className="border-t border-border first:border-t-0">
                  <th
                    scope="row"
                    className="whitespace-nowrap bg-muted/40 px-2 py-1 font-mono text-muted-foreground"
                  >
                    {k}
                  </th>
                  <td className="break-all px-2 py-1 font-mono">{v}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

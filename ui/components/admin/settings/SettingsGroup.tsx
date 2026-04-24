"use client";

import { useState } from "react";
import { ChevronDown, ChevronRight } from "lucide-react";
import type { SettingsSchemaNode } from "@/lib/api/schemas";
import { labelForGroup } from "@/lib/admin/settings/format";
import { cn } from "@/lib/utils";
import { SettingField } from "./SettingField";

export interface SettingsGroupProps {
  group: string;
  nodes: SettingsSchemaNode[];
  values: Record<string, unknown>;
  errors: Record<string, string>;
  onFieldChange: (key: string, value: unknown) => void;
  defaultOpen?: boolean;
}

/**
 * Collapsible group of related fields. Opens expanded by default so the
 * first-time visitor sees every knob; collapsing is cosmetic only.
 */
export function SettingsGroup({
  group,
  nodes,
  values,
  errors,
  onFieldChange,
  defaultOpen = true,
}: SettingsGroupProps) {
  const [open, setOpen] = useState(defaultOpen);
  const dirtyCount = nodes.filter((n) => errors[n.key]).length;

  return (
    <section
      aria-label={labelForGroup(group)}
      className="rounded-lg border border-border bg-background"
    >
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        className={cn(
          "flex w-full items-center justify-between px-4 py-3 text-left",
          "focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
        )}
      >
        <span className="flex items-center gap-2 text-sm font-semibold">
          {open ? (
            <ChevronDown className="h-4 w-4" aria-hidden="true" />
          ) : (
            <ChevronRight className="h-4 w-4" aria-hidden="true" />
          )}
          {labelForGroup(group)}
          <span className="text-xs font-normal text-muted-foreground">
            ({nodes.length})
          </span>
        </span>
        {dirtyCount > 0 ? (
          <span
            className="rounded-full bg-destructive/10 px-2 py-0.5 text-xs text-destructive"
            aria-label={`${dirtyCount} invalid fields`}
          >
            {dirtyCount} invalid
          </span>
        ) : null}
      </button>
      {open ? (
        <div className="divide-y divide-border border-t border-border px-4">
          {nodes.map((n) => (
            <SettingField
              key={n.key}
              node={n}
              value={values[n.key]}
              onChange={(v) => onFieldChange(n.key, v)}
              error={errors[n.key] ?? null}
            />
          ))}
        </div>
      ) : null}
    </section>
  );
}

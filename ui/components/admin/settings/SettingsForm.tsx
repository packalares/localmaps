"use client";

import { useMemo, useState } from "react";
import { Search } from "lucide-react";
import type {
  SettingsSchemaNode,
  SettingsTree,
} from "@/lib/api/schemas";
import { groupNodes } from "@/lib/admin/settings/format";
import { useSettingsForm } from "@/lib/admin/settings/use-settings-form";
import { cn } from "@/lib/utils";
import { SettingsGroup } from "./SettingsGroup";
import { UnsavedBanner } from "./UnsavedBanner";

export interface SettingsFormProps {
  tree: SettingsTree | undefined;
  nodes: SettingsSchemaNode[];
  onSave: (diff: Record<string, unknown>) => void | Promise<void>;
  isSaving?: boolean;
  saveError?: string | null;
}

/**
 * Top-level schema-driven settings form. Owns the dirty/error state via
 * useSettingsForm and delegates rendering to SettingsGroup. A search box
 * narrows visible fields by key path + label.
 */
export function SettingsForm({
  tree,
  nodes,
  onSave,
  isSaving,
  saveError,
}: SettingsFormProps) {
  const form = useSettingsForm(tree, nodes);
  const [filter, setFilter] = useState("");

  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return nodes;
    return nodes.filter(
      (n) =>
        n.key.toLowerCase().includes(q) ||
        (n.description ?? "").toLowerCase().includes(q),
    );
  }, [nodes, filter]);

  const groups = useMemo(() => groupNodes(filtered), [filtered]);
  const dirtyCount = Object.keys(form.diff).length;

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      <UnsavedBanner
        dirtyCount={dirtyCount}
        canSave={form.isValid}
        pending={!!isSaving}
        onSave={() => onSave(form.diff)}
        onRevert={form.revert}
        errorMessage={saveError ?? null}
      />

      <div className="flex items-center gap-2 px-1">
        <Search className="h-4 w-4 text-muted-foreground" aria-hidden="true" />
        <input
          type="search"
          value={filter}
          onChange={(ev) => setFilter(ev.target.value)}
          aria-label="Filter settings"
          placeholder="Filter by key or description…"
          className={cn(
            "h-9 w-full max-w-md rounded-md border border-input bg-background px-3 text-sm",
            "focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
          )}
        />
        <span className="ml-auto text-xs text-muted-foreground">
          {filtered.length} of {nodes.length} fields
        </span>
      </div>

      <div className="flex flex-col gap-3">
        {groups.length === 0 ? (
          <p className="px-1 text-sm text-muted-foreground">
            No fields match “{filter}”.
          </p>
        ) : null}
        {groups.map(({ group, nodes: groupNodes }) => (
          <SettingsGroup
            key={group}
            group={group}
            nodes={groupNodes}
            values={form.draft}
            errors={form.errors}
            onFieldChange={form.setField}
          />
        ))}
      </div>
    </div>
  );
}

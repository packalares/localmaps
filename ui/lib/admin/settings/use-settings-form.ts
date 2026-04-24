"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import type { SettingsSchemaNode, SettingsTree } from "@/lib/api/schemas";
import { diffSettings, flattenTree } from "./diff";
import { validateAll, validateValue } from "./validate";

/**
 * Centralises the draft/dirty/error state for the settings form. Takes
 * the live tree (from useSettings) and the schema node list and returns
 * getters + setters the UI components compose against.
 */
export interface SettingsFormState {
  /** Current draft, flat. Mutations happen through setField. */
  draft: Record<string, unknown>;
  /** Initial flattened values (resets to this when user clicks Revert). */
  initial: Record<string, unknown>;
  /** Per-key validation errors (from local validate.ts). */
  errors: Record<string, string>;
  /** The diff body the save button would send. */
  diff: Record<string, unknown>;
  /** True when diff is non-empty. */
  isDirty: boolean;
  /** True when every dirty field passes local validation. */
  isValid: boolean;
  setField: (key: string, value: unknown) => void;
  revert: () => void;
  resetBaseline: (tree: SettingsTree) => void;
}

export function useSettingsForm(
  tree: SettingsTree | undefined,
  nodes: SettingsSchemaNode[],
): SettingsFormState {
  const initial = useMemo(
    () => (tree ? flattenTree(tree) : {}),
    [tree],
  );
  const [draft, setDraft] = useState<Record<string, unknown>>(initial);

  // Reset the draft when a new tree arrives (first load, post-save, etc.).
  useEffect(() => {
    setDraft(initial);
  }, [initial]);

  const errors = useMemo(() => validateAll(nodes, draft), [nodes, draft]);
  const diff = useMemo(() => diffSettings(initial, draft), [initial, draft]);
  const isDirty = Object.keys(diff).length > 0;
  const isValid = Object.keys(errors).length === 0;

  const setField = useCallback((key: string, value: unknown) => {
    setDraft((prev) => ({ ...prev, [key]: value }));
  }, []);

  const revert = useCallback(() => setDraft(initial), [initial]);

  const resetBaseline = useCallback((tree: SettingsTree) => {
    const next = flattenTree(tree);
    setDraft(next);
  }, []);

  return {
    draft,
    initial,
    errors,
    diff,
    isDirty,
    isValid,
    setField,
    revert,
    resetBaseline,
  };
}

/** Best-effort inline error for a single field, composing schema-level
 *  validation with draft state. */
export function fieldError(
  node: SettingsSchemaNode,
  value: unknown,
): string | null {
  return validateValue(node, value);
}

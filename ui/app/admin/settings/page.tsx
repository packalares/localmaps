"use client";

import { useState } from "react";
import { ApiError } from "@/lib/api/client";
import {
  useSaveSettings,
  useSettings,
  useSettingsSchema,
} from "@/lib/api/hooks";
import { SettingsForm } from "@/components/admin/settings/SettingsForm";

/**
 * /admin/settings — schema-driven settings panel. The schema (anonymous)
 * arrives via GET /api/settings/schema; the current tree (admin only)
 * via GET /api/settings. PATCH commits a diff. Errors are surfaced in
 * the sticky banner with the offending key.
 */
export default function AdminSettingsPage() {
  const schemaQ = useSettingsSchema();
  const treeQ = useSettings();
  const saveMutation = useSaveSettings();

  const [saveError, setSaveError] = useState<string | null>(null);

  const unauth = treeQ.error instanceof ApiError && treeQ.error.status === 401;

  const onSave = async (diff: Record<string, unknown>) => {
    setSaveError(null);
    try {
      await saveMutation.mutateAsync(diff);
    } catch (err) {
      if (err instanceof ApiError) {
        setSaveError(err.message);
      } else if (err instanceof Error) {
        setSaveError(err.message);
      } else {
        setSaveError("Unknown error");
      }
    }
  };

  return (
    <div className="flex h-full min-h-0 flex-col gap-4 p-6">
      <header>
        <h1 className="text-2xl font-semibold">Settings</h1>
        <p className="text-sm text-muted-foreground">
          Every runtime knob is editable. Changes are validated server-side
          and persisted to the settings database.
        </p>
      </header>

      {unauth ? (
        <div
          role="alert"
          className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm"
        >
          You are not signed in. Settings require a signed-in admin
          session.{" "}
          <a
            className="font-medium underline underline-offset-2"
            href="/login"
          >
            Sign in
          </a>
          .
        </div>
      ) : null}

      {schemaQ.isLoading ? (
        <p className="text-sm text-muted-foreground">Loading schema…</p>
      ) : null}
      {schemaQ.error ? (
        <p className="text-sm text-destructive" role="alert">
          Failed to load settings schema:{" "}
          {schemaQ.error instanceof Error ? schemaQ.error.message : "unknown"}
        </p>
      ) : null}

      {schemaQ.data && !unauth ? (
        <SettingsForm
          tree={treeQ.data}
          nodes={schemaQ.data.nodes}
          onSave={onSave}
          isSaving={saveMutation.isPending}
          saveError={saveError}
        />
      ) : null}
    </div>
  );
}

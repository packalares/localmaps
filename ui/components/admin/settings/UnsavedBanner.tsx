"use client";

import { Button } from "@/components/ui/button";
import { useMessages } from "@/lib/i18n/provider";

export interface UnsavedBannerProps {
  dirtyCount: number;
  canSave: boolean;
  pending: boolean;
  onSave: () => void;
  onRevert: () => void;
  errorMessage?: string | null;
}

/**
 * Sticky strip shown when the user has unsaved edits. Offers Save / Revert
 * plus surfaces the last server error. Disabled while a mutation is in
 * flight.
 */
export function UnsavedBanner({
  dirtyCount,
  canSave,
  pending,
  onSave,
  onRevert,
  errorMessage,
}: UnsavedBannerProps) {
  const { t } = useMessages();
  if (dirtyCount === 0 && !errorMessage) return null;
  return (
    <div
      className="sticky top-0 z-20 flex flex-col gap-2 border-b border-border bg-background px-4 py-2 shadow"
      role="region"
      aria-label="Unsaved settings"
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="text-sm">
          {dirtyCount > 0
            ? t(
                dirtyCount === 1
                  ? "settings.unsaved.countOne"
                  : "settings.unsaved.count",
                { count: dirtyCount },
              )
            : t("settings.unsaved.error")}
        </span>
        <div className="flex gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={pending || dirtyCount === 0}
            onClick={onRevert}
          >
            {t("common.revert")}
          </Button>
          <Button
            type="button"
            size="sm"
            disabled={pending || !canSave || dirtyCount === 0}
            onClick={onSave}
          >
            {pending ? t("common.saving") : t("common.saveAll")}
          </Button>
        </div>
      </div>
      {errorMessage ? (
        <p className="text-sm text-destructive" role="alert">
          {errorMessage}
        </p>
      ) : null}
    </div>
  );
}

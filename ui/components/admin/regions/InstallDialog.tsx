"use client";

import { Download } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type { RegionCatalogEntry } from "@/lib/api/schemas";
import { formatBytes } from "@/lib/admin/regions/format-bytes";
import {
  estimateInstall,
  formatDurationShort,
} from "@/lib/admin/regions/estimates";

/**
 * Confirm-install modal. Shown when the user clicks Install on a
 * catalogue leaf. Computes a rough disk / duration estimate from the
 * pbf size reported by Geofabrik, then POSTs through the provided
 * `onConfirm` when the user commits.
 */
export interface InstallDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  entry: RegionCatalogEntry | null;
  onConfirm: (entry: RegionCatalogEntry) => void;
  pending?: boolean;
  errorMessage?: string | null;
}

export function InstallDialog({
  open,
  onOpenChange,
  entry,
  onConfirm,
  pending,
  errorMessage,
}: InstallDialogProps) {
  if (!entry) return null;
  const estimate = estimateInstall(
    entry.estimatedBuildBytes ?? entry.sourcePbfBytes ?? null,
  );

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Install {entry.displayName}</DialogTitle>
          <DialogDescription>
            Downloads the OpenStreetMap extract from Geofabrik and
            rebuilds tiles, routing graph, geocoder index, and POIs.
          </DialogDescription>
        </DialogHeader>
        <dl className="grid grid-cols-2 gap-x-4 gap-y-2 pt-2 text-sm">
          <dt className="text-muted-foreground">Source name</dt>
          <dd className="font-mono text-xs">{entry.name}</dd>

          <dt className="text-muted-foreground">Source size</dt>
          <dd>{formatBytes(entry.sourcePbfBytes ?? null)}</dd>

          <dt className="text-muted-foreground">Estimated disk</dt>
          <dd>{formatBytes(estimate.diskBytes)}</dd>

          <dt className="text-muted-foreground">Estimated duration</dt>
          <dd>{formatDurationShort(estimate.durationSeconds)}</dd>
        </dl>
        {errorMessage ? (
          <p className="text-sm text-destructive" role="alert">
            {errorMessage}
          </p>
        ) : null}
        <div className="flex justify-end gap-2 pt-2">
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={pending}
          >
            Cancel
          </Button>
          <Button
            type="button"
            onClick={() => onConfirm(entry)}
            disabled={pending}
            aria-label={`Install ${entry.displayName}`}
          >
            <Download className="mr-2 h-4 w-4" aria-hidden="true" />
            {pending ? "Queuing…" : "Install"}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}

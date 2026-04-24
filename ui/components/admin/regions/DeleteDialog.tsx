"use client";

import { useEffect, useState } from "react";
import { Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { cn } from "@/lib/utils";

/**
 * Typed-confirmation dialog for DELETE /api/regions/{name}. The confirm
 * button stays disabled until the user types the region's canonical
 * name exactly. Matches Google-style "type PROJECT_NAME to confirm"
 * flows.
 */
export interface DeleteDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  regionName: string;
  displayName: string;
  onConfirm: () => void;
  pending?: boolean;
  errorMessage?: string | null;
}

export function DeleteDialog({
  open,
  onOpenChange,
  regionName,
  displayName,
  onConfirm,
  pending,
  errorMessage,
}: DeleteDialogProps) {
  const [typed, setTyped] = useState("");

  // Reset the input every time the dialog opens on a different region.
  useEffect(() => {
    if (open) setTyped("");
  }, [open, regionName]);

  const matches = typed.trim() === regionName;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Delete {displayName}?</DialogTitle>
          <DialogDescription>
            This removes the region&apos;s tiles, routing graph, geocoder
            index, and POI data from disk. The source pbf is kept only if
            <code className="mx-1 rounded bg-muted px-1 py-0.5 text-xs">
              regions.keepSourcePbf
            </code>
            is enabled. This action cannot be undone.
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-2 pt-2">
          <label htmlFor="delete-confirm" className="text-sm text-muted-foreground">
            Type{" "}
            <code className="rounded bg-muted px-1 py-0.5 text-xs">
              {regionName}
            </code>{" "}
            to confirm.
          </label>
          <input
            id="delete-confirm"
            type="text"
            autoComplete="off"
            autoFocus
            value={typed}
            onChange={(ev) => setTyped(ev.target.value)}
            aria-label="Confirm region name"
            className={cn(
              "h-9 w-full rounded-md border border-input bg-background px-3 text-sm",
              "focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
            )}
          />
          {errorMessage ? (
            <p className="text-sm text-destructive" role="alert">
              {errorMessage}
            </p>
          ) : null}
        </div>
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
            variant="destructive"
            disabled={!matches || pending}
            onClick={onConfirm}
            aria-label={`Delete ${displayName}`}
          >
            <Trash2 className="mr-2 h-4 w-4" aria-hidden="true" />
            {pending ? "Deleting…" : "Delete"}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}

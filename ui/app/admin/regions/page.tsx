"use client";

import { useState } from "react";
import { ApiError } from "@/lib/api/client";
import {
  useDeleteRegion,
  useInstallRegion,
  useSetRegionSchedule,
  useUpdateRegionNow,
} from "@/lib/api/hooks";
import type { Region, RegionCatalogEntry } from "@/lib/api/schemas";
import { ActiveJobsBadge } from "@/components/admin/regions/ActiveJobsBadge";
import { CatalogTree } from "@/components/admin/regions/CatalogTree";
import { DeleteDialog } from "@/components/admin/regions/DeleteDialog";
import { InstallDialog } from "@/components/admin/regions/InstallDialog";
import { InstalledTable } from "@/components/admin/regions/InstalledTable";
import { useRegionsAdmin } from "@/lib/admin/regions/use-regions-admin";
import { useRegionStream } from "@/lib/admin/regions/use-region-stream";

/**
 * /admin/regions — Google-Material-admin-style Regions page. Split
 * layout: catalogue on the left, installed table on the right. The
 * WebSocket stream feeds progress into the cache; mutations optimistic-
 * invalidate the installed list.
 */
export default function AdminRegionsPage() {
  const data = useRegionsAdmin();
  useRegionStream();

  const install = useInstallRegion();
  const deleteMutation = useDeleteRegion();
  const updateNow = useUpdateRegionNow();
  const setSchedule = useSetRegionSchedule();

  const [installTarget, setInstallTarget] =
    useState<RegionCatalogEntry | null>(null);
  const [installOpen, setInstallOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Region | null>(null);
  const [deleteOpen, setDeleteOpen] = useState(false);
  // On mobile the catalogue tree is collapsible; default collapsed so
  // the installed list (where most admin actions land) leads the page.
  const [catalogOpen, setCatalogOpen] = useState(false);

  const pendingByName: Record<string, "update" | "schedule" | "delete" | null> =
    {};
  if (updateNow.isPending && updateNow.variables)
    pendingByName[updateNow.variables.name] = "update";
  if (deleteMutation.isPending && deleteMutation.variables)
    pendingByName[deleteMutation.variables.name] = "delete";
  if (setSchedule.isPending && setSchedule.variables)
    pendingByName[setSchedule.variables.name] = "schedule";

  const openInstall = (entry: RegionCatalogEntry) => {
    setInstallTarget(entry);
    setInstallOpen(true);
    install.reset();
  };
  const openDelete = (region: Region) => {
    setDeleteTarget(region);
    setDeleteOpen(true);
    deleteMutation.reset();
  };

  const confirmInstall = (entry: RegionCatalogEntry) => {
    install.mutate(
      { name: entry.name },
      {
        onSuccess: () => {
          setInstallOpen(false);
          setInstallTarget(null);
        },
      },
    );
  };
  const confirmDelete = () => {
    if (!deleteTarget) return;
    deleteMutation.mutate(
      { name: deleteTarget.name },
      {
        onSuccess: () => {
          setDeleteOpen(false);
          setDeleteTarget(null);
        },
      },
    );
  };

  const authError = data.error instanceof ApiError && data.error.status === 401;

  return (
    <div className="flex h-full min-h-0 flex-col gap-4 p-4 md:p-6">
      <header className="flex items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold">Regions</h1>
          <p className="text-sm text-muted-foreground">
            Install OpenStreetMap regions, monitor progress, and schedule
            automatic updates.
          </p>
        </div>
        <ActiveJobsBadge count={data.activeCount} />
      </header>

      {authError ? (
        <div
          role="alert"
          className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm"
        >
          You are not signed in. Region management requires a signed-in
          admin session.{" "}
          <a
            className="font-medium underline underline-offset-2"
            href="/login"
          >
            Sign in
          </a>
          .
        </div>
      ) : null}

      <div className="flex min-h-0 flex-1 flex-col gap-4 lg:grid lg:grid-cols-[minmax(280px,22rem)_1fr]">
        <section
          aria-label="Geofabrik catalogue"
          className="flex min-h-0 flex-col rounded-lg border border-border bg-background"
        >
          <header className="flex items-center justify-between px-3 py-2 text-sm font-medium">
            <span>Catalogue</span>
            <div className="flex items-center gap-2">
              {data.isLoading ? (
                <span className="text-xs text-muted-foreground">Loading…</span>
              ) : null}
              <button
                type="button"
                onClick={() => setCatalogOpen((o) => !o)}
                aria-expanded={catalogOpen}
                aria-controls="admin-regions-catalog"
                className="rounded border border-border px-2 py-0.5 text-xs text-muted-foreground hover:bg-muted focus:outline-none focus-visible:ring-2 focus-visible:ring-ring md:hidden"
              >
                {catalogOpen ? "Hide" : "Show"}
              </button>
            </div>
          </header>
          <div
            id="admin-regions-catalog"
            className={catalogOpen ? "contents md:contents" : "hidden md:contents"}
          >
            <CatalogTree
              entries={data.catalog}
              installedByName={data.installedByName}
              onInstall={openInstall}
              className="min-h-0 flex-1"
            />
          </div>
        </section>

        <section
          aria-label="Installed regions"
          className="flex min-h-0 flex-col rounded-lg border border-border bg-background"
        >
          <header className="flex items-center justify-between border-b border-border px-3 py-2 text-sm font-medium">
            <span>Installed</span>
            <span className="text-xs text-muted-foreground">
              {data.regions.length} total · {data.activeCount} active
            </span>
          </header>
          <InstalledTable
            regions={data.regions}
            pendingByName={pendingByName}
            onUpdateNow={(r) => updateNow.mutate({ name: r.name })}
            onScheduleChange={(r, next) =>
              setSchedule.mutate({ name: r.name, schedule: next })
            }
            onDelete={openDelete}
          />
        </section>
      </div>

      <InstallDialog
        open={installOpen}
        onOpenChange={setInstallOpen}
        entry={installTarget}
        onConfirm={confirmInstall}
        pending={install.isPending}
        errorMessage={install.error instanceof Error ? install.error.message : null}
      />

      <DeleteDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        regionName={deleteTarget?.name ?? ""}
        displayName={deleteTarget?.displayName ?? ""}
        onConfirm={confirmDelete}
        pending={deleteMutation.isPending}
        errorMessage={
          deleteMutation.error instanceof Error
            ? deleteMutation.error.message
            : null
        }
      />
    </div>
  );
}

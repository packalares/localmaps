"use client";

import { Download, Link2 } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type { Route } from "@/lib/api/schemas";
import { downloadBlob } from "@/lib/directions/download-blob";
import { routeToGpx } from "@/lib/directions/gpx";
import { routeToKml } from "@/lib/directions/kml";
import type { Waypoint } from "@/lib/state/directions";

export interface ExportMenuProps {
  route: Route;
  waypoints: Waypoint[];
  /** Optional click handler to copy a shareable link. Wired by primary. */
  onCopyLink?: () => void;
}

function appVersion(): string {
  return process.env.NEXT_PUBLIC_APP_VERSION ?? "dev";
}

function baseFilename(routeId: string): string {
  const safe = routeId.replace(/[^a-zA-Z0-9_-]+/g, "-").slice(0, 40) || "route";
  return `localmaps-${safe}`;
}

function waypointExports(waypoints: Waypoint[]) {
  return waypoints
    .filter((w) => w.lngLat)
    .map((w) => ({
      lng: w.lngLat!.lng,
      lat: w.lngLat!.lat,
      name: w.label || "",
    }));
}

export function ExportMenu({ route, waypoints, onCopyLink }: ExportMenuProps) {
  const handleGpx = () => {
    const exports = waypointExports(waypoints);
    const xml = routeToGpx({
      routeId: route.id,
      polylines: route.legs.map((l) => l.geometry.polyline),
      waypoints: exports.map((w) => ({
        lat: w.lat,
        lon: w.lng,
        name: w.name,
      })),
      appVersion: appVersion(),
    });
    downloadBlob({
      filename: `${baseFilename(route.id)}.gpx`,
      content: xml,
      mimeType: "application/gpx+xml",
    });
  };

  const handleKml = () => {
    const exports = waypointExports(waypoints);
    const xml = routeToKml({
      routeId: route.id,
      polylines: route.legs.map((l) => l.geometry.polyline),
      waypoints: exports.map((w) => ({
        lat: w.lat,
        lon: w.lng,
        name: w.name,
      })),
      appVersion: appVersion(),
    });
    downloadBlob({
      filename: `${baseFilename(route.id)}.kml`,
      content: xml,
      mimeType: "application/vnd.google-earth.kml+xml",
    });
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label="Export route"
          className="inline-flex items-center gap-1 rounded-full border border-border bg-background px-3 py-1 text-xs font-medium text-foreground hover:bg-muted focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <Download className="h-3.5 w-3.5" aria-hidden={true} />
          Export
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onSelect={handleGpx}>
          <Download className="h-4 w-4" aria-hidden={true} />
          Download GPX
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={handleKml}>
          <Download className="h-4 w-4" aria-hidden={true} />
          Download KML
        </DropdownMenuItem>
        {onCopyLink && (
          <DropdownMenuItem onSelect={onCopyLink}>
            <Link2 className="h-4 w-4" aria-hidden={true} />
            Copy link with route
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

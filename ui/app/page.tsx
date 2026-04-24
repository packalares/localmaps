import { MainMap } from "@/components/map/MainMap";
import { Attribution } from "@/components/chrome/Attribution";
import { FabStack } from "@/components/chrome/FabStack";
import { LeftRail } from "@/components/chrome/LeftRail";
import { ContextMenuHost } from "@/components/chrome/ContextMenuHost";
import { MeasureTool } from "@/components/tools/MeasureTool";
import { MeasureOverlay } from "@/components/tools/MeasureOverlay";
import { IsochroneTool } from "@/components/tools/IsochroneTool";
import { IsochronePanel } from "@/components/tools/IsochronePanel";

/**
 * Main map page. Layout is modelled on Google Maps desktop:
 *
 * - The MapLibre canvas fills the viewport.
 * - The left rail (search bar, tabs, results) floats over the map.
 * - The right rail floats FABs (zoom / compass / locate).
 * - A discreet attribution strip sits bottom-center.
 *
 * Everything is positioned absolutely over the map container so the map
 * stays the canvas of record.
 */
export default function HomePage() {
  return (
    <main className="relative h-dvh w-screen overflow-hidden">
      <MainMap />

      {/* Left rail — search + panel. */}
      <div className="pointer-events-none absolute inset-y-0 left-0 flex h-full py-4">
        <LeftRail />
      </div>

      {/* Right rail — floating action buttons. */}
      <div className="pointer-events-none absolute right-4 top-4 flex flex-col gap-3">
        <FabStack />
      </div>

      {/* Bottom attribution. */}
      <div className="pointer-events-none absolute inset-x-0 bottom-2 flex justify-center">
        <Attribution />
      </div>

      {/* Right-click context menu (renders on demand). */}
      <ContextMenuHost />

      {/* Phase-7 tools: measure + isochrone. The drivers are invisible;
          their UI companions render conditionally on active tool. */}
      <MeasureTool />
      <IsochroneTool />
      <MeasureOverlay />
      <IsochronePanel />
    </main>
  );
}

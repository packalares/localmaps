"use client";

import { Compass } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ToolsFab } from "@/components/tools/ToolsFab";
import { RecenterButton } from "./RecenterButton";
import { ZoomControls } from "./ZoomControls";

export interface FabStackProps {
  onZoomIn?: () => void;
  onZoomOut?: () => void;
  onResetBearing?: () => void;
  onLocate?: () => void;
  /** Toggle for the Phase-7 tool suite FAB (measure + isochrone). */
  showTools?: boolean;
}

/**
 * Right-rail floating action buttons. Ordered top-down the same way
 * Google Maps does it: zoom controls, a compass that resets bearing,
 * then the "my location" button at the bottom. The Phase-7 tools
 * (measure + isochrone) share a single entrypoint via `ToolsFab` sitting
 * above the zoom cluster.
 */
export function FabStack({
  onZoomIn,
  onZoomOut,
  onResetBearing,
  onLocate,
  showTools = true,
}: FabStackProps) {
  return (
    <div className="pointer-events-auto flex flex-col items-end gap-3">
      {showTools && <ToolsFab />}
      <ZoomControls onZoomIn={onZoomIn} onZoomOut={onZoomOut} />
      <Button
        variant="chrome"
        onClick={onResetBearing}
        aria-label="Reset bearing to north"
        title="Reset bearing to north"
      >
        <Compass className="h-5 w-5" aria-hidden="true" />
      </Button>
      <RecenterButton onLocate={onLocate} />
    </div>
  );
}

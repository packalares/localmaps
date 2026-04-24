"use client";

import { Minus, Plus } from "lucide-react";
import { Button } from "@/components/ui/button";

export interface ZoomControlsProps {
  onZoomIn?: () => void;
  onZoomOut?: () => void;
}

/**
 * Vertically stacked plus/minus buttons used on the right rail. Values
 * are driven by the parent so MapView can own the MapLibre instance.
 */
export function ZoomControls({ onZoomIn, onZoomOut }: ZoomControlsProps) {
  return (
    <div className="flex flex-col gap-1">
      <Button
        variant="chrome"
        onClick={onZoomIn}
        aria-label="Zoom in"
        title="Zoom in"
      >
        <Plus className="h-5 w-5" aria-hidden="true" />
      </Button>
      <Button
        variant="chrome"
        onClick={onZoomOut}
        aria-label="Zoom out"
        title="Zoom out"
      >
        <Minus className="h-5 w-5" aria-hidden="true" />
      </Button>
    </div>
  );
}

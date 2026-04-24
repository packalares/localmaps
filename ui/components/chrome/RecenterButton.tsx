"use client";

import { LocateFixed } from "lucide-react";
import { Button } from "@/components/ui/button";

export interface RecenterButtonProps {
  onLocate?: () => void;
}

/**
 * Google-Maps-style "my location" button. Parent provides the actual
 * geolocation handler; we only render the affordance.
 */
export function RecenterButton({ onLocate }: RecenterButtonProps) {
  return (
    <Button
      variant="chrome"
      onClick={onLocate}
      aria-label="Show my location"
      title="Show my location"
    >
      <LocateFixed className="h-5 w-5" aria-hidden="true" />
    </Button>
  );
}

"use client";

import { useEffect, useRef, useState } from "react";
import { useDirectionsStore } from "@/lib/state/directions";
import { useMapStore } from "@/lib/state/map";
import { cn } from "@/lib/utils";

export interface ContextMenuPosition {
  /** Page coordinates in CSS pixels. */
  x: number;
  y: number;
  /** Geographic coordinate under the cursor when the menu opened. */
  lat: number;
  lon: number;
}

export interface ContextMenuProps {
  position: ContextMenuPosition | null;
  onClose: () => void;
  onWhatsHere?: (at: ContextMenuPosition) => void;
  onDirectionsFromHere?: (at: ContextMenuPosition) => void;
  onDirectionsToHere?: (at: ContextMenuPosition) => void;
}

/**
 * Right-click context menu mirroring Google Maps: three actions pinned
 * to the click location. Dismissed on Escape or outside click.
 *
 * The "Directions from/to here" actions have sane defaults wired to
 * the directions store + left-rail tab switch; callers can still
 * override via the optional callbacks for custom flows.
 */
export function ContextMenu({
  position,
  onClose,
  onWhatsHere,
  onDirectionsFromHere,
  onDirectionsToHere,
}: ContextMenuProps) {
  const ref = useRef<HTMLDivElement | null>(null);
  const [focused, setFocused] = useState(false);
  const setStartFromPoint = useDirectionsStore((s) => s.setStartFromPoint);
  const setEndFromPoint = useDirectionsStore((s) => s.setEndFromPoint);
  const openLeftRail = useMapStore((s) => s.openLeftRail);

  useEffect(() => {
    if (!position) return;
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        onClose();
      }
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("mousedown", onDown);
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("mousedown", onDown);
      window.removeEventListener("keydown", onKey);
    };
  }, [position, onClose]);

  useEffect(() => {
    if (position && ref.current && !focused) {
      ref.current.focus();
      setFocused(true);
    }
    if (!position) setFocused(false);
  }, [position, focused]);

  if (!position) return null;

  const items: Array<{ label: string; onClick: () => void }> = [
    {
      label: "What's here?",
      onClick: () => {
        onWhatsHere?.(position);
        onClose();
      },
    },
    {
      label: "Directions from here",
      onClick: () => {
        if (onDirectionsFromHere) {
          onDirectionsFromHere(position);
        } else {
          setStartFromPoint({ lng: position.lon, lat: position.lat });
          openLeftRail("directions");
        }
        onClose();
      },
    },
    {
      label: "Directions to here",
      onClick: () => {
        if (onDirectionsToHere) {
          onDirectionsToHere(position);
        } else {
          setEndFromPoint({ lng: position.lon, lat: position.lat });
          openLeftRail("directions");
        }
        onClose();
      },
    },
  ];

  return (
    <div
      ref={ref}
      role="menu"
      tabIndex={-1}
      className={cn(
        "pointer-events-auto chrome-card fixed z-50 min-w-[220px] py-1 text-sm",
      )}
      style={{ left: position.x, top: position.y }}
    >
      {items.map((item) => (
        <button
          key={item.label}
          type="button"
          role="menuitem"
          onClick={item.onClick}
          className="block w-full px-3 py-1.5 text-left hover:bg-muted focus:bg-muted focus:outline-none"
        >
          {item.label}
        </button>
      ))}
      <div className="mt-1 border-t border-border px-3 pb-1 pt-1 font-mono text-xs text-muted-foreground">
        {position.lat.toFixed(5)}, {position.lon.toFixed(5)}
      </div>
    </div>
  );
}

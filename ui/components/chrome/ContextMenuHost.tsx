"use client";

import { useEffect, useState } from "react";
import { ContextMenu, type ContextMenuPosition } from "./ContextMenu";
import { useMapStore } from "@/lib/state/map";

/**
 * Host component that bridges the store's `pendingContextmenu` event to
 * the presentational ContextMenu. Keeps ContextMenu decoupled from the
 * store so it remains trivially testable.
 */
export function ContextMenuHost() {
  const pending = useMapStore((s) => s.pendingContextmenu);
  const clear = useMapStore((s) => s.clearPendingContextmenu);
  const [pos, setPos] = useState<ContextMenuPosition | null>(null);

  useEffect(() => {
    if (!pending) return;
    setPos({
      x: pending.point.x,
      y: pending.point.y,
      lat: pending.lngLat.lat,
      lon: pending.lngLat.lng,
    });
    clear();
  }, [pending, clear]);

  return <ContextMenu position={pos} onClose={() => setPos(null)} />;
}

"use client";

import {
  useCallback,
  useEffect,
  useId,
  useRef,
  useState,
  type PointerEvent as ReactPointerEvent,
  type ReactNode,
} from "react";
import { cn } from "@/lib/utils";

/**
 * Three-point bottom sheet modelled on the Google Maps mobile drawer.
 *
 *   - `peek`  : sheet shows only the handle + header strip (~25% height).
 *   - `half`  : sheet fills the lower ~60% of the screen.
 *   - `full`  : sheet expands to ~95%, a backdrop appears, and focus is
 *               trapped — same semantics as a modal.
 *
 * Gestures: drag the handle (or anywhere above the scrollable body) to
 * move between snap points. On release we snap to the nearest point.
 * Keyboard: Escape collapses — `full` → `half` → `peek` — and dismisses
 * when already at `peek` via the `onRequestClose` callback. The handle
 * bar is focusable with Arrow Up/Down to change snap points for users
 * without pointer devices.
 *
 * We deliberately avoid `@radix-ui/react-dialog` here: the sheet is
 * *always mounted* under the map so its state can be animated; Dialog
 * unmounts on close and would fight our drag. Instead the `full` snap
 * opts in to Dialog-like focus trapping + aria-modal.
 */
export type SheetSnap = "peek" | "half" | "full";

export interface BottomSheetProps {
  snap: SheetSnap;
  onSnapChange: (next: SheetSnap) => void;
  /** Header row rendered above the scrollable body — typically the tab chip. */
  header?: ReactNode;
  children: ReactNode;
  /** Fires when Escape is pressed at `peek` (user dismisses the sheet entirely). */
  onRequestClose?: () => void;
  /** aria-label for the sheet region. Defaults to 'Map panel'. */
  label?: string;
  className?: string;
}

const SNAP_HEIGHTS: Record<SheetSnap, number> = {
  peek: 25,
  half: 60,
  full: 95,
};

const SNAP_ORDER: SheetSnap[] = ["peek", "half", "full"];

function nearestSnap(heightPct: number): SheetSnap {
  let best: SheetSnap = "peek";
  let bestDist = Infinity;
  for (const s of SNAP_ORDER) {
    const d = Math.abs(SNAP_HEIGHTS[s] - heightPct);
    if (d < bestDist) {
      bestDist = d;
      best = s;
    }
  }
  return best;
}

export function BottomSheet({
  snap,
  onSnapChange,
  header,
  children,
  onRequestClose,
  label = "Map panel",
  className,
}: BottomSheetProps) {
  const sheetRef = useRef<HTMLDivElement | null>(null);
  const titleId = useId();
  const [dragOffsetPct, setDragOffsetPct] = useState<number | null>(null);
  const dragStartRef = useRef<{ y: number; startPct: number } | null>(null);

  const heightPct =
    dragOffsetPct !== null ? dragOffsetPct : SNAP_HEIGHTS[snap];

  const onPointerDown = useCallback(
    (e: ReactPointerEvent<HTMLDivElement>) => {
      if (e.pointerType === "mouse" && e.button !== 0) return;
      (e.target as HTMLElement).setPointerCapture?.(e.pointerId);
      dragStartRef.current = { y: e.clientY, startPct: SNAP_HEIGHTS[snap] };
    },
    [snap],
  );

  const onPointerMove = useCallback((e: ReactPointerEvent<HTMLDivElement>) => {
    const start = dragStartRef.current;
    if (!start) return;
    const vh = window.innerHeight || 1;
    const deltaPct = ((start.y - e.clientY) / vh) * 100;
    const next = Math.max(10, Math.min(98, start.startPct + deltaPct));
    setDragOffsetPct(next);
  }, []);

  const onPointerUp = useCallback(() => {
    const start = dragStartRef.current;
    if (!start) return;
    dragStartRef.current = null;
    if (dragOffsetPct !== null) {
      const target = nearestSnap(dragOffsetPct);
      onSnapChange(target);
    }
    setDragOffsetPct(null);
  }, [dragOffsetPct, onSnapChange]);

  // Escape collapses one snap; at peek it dismisses.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      if (snap === "full") onSnapChange("half");
      else if (snap === "half") onSnapChange("peek");
      else onRequestClose?.();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [snap, onSnapChange, onRequestClose]);

  // Focus trap + restore when the sheet is `full` (modal semantics).
  useEffect(() => {
    if (snap !== "full") return;
    const prev = document.activeElement as HTMLElement | null;
    const root = sheetRef.current;
    const first = root?.querySelector<HTMLElement>(
      "[data-sheet-focus-initial]",
    );
    first?.focus();
    const onTrap = (e: KeyboardEvent) => {
      if (e.key !== "Tab" || !root) return;
      const focusable = root.querySelectorAll<HTMLElement>(
        'a,button,input,select,textarea,[tabindex]:not([tabindex="-1"])',
      );
      if (focusable.length === 0) return;
      const firstEl = focusable[0];
      const lastEl = focusable[focusable.length - 1];
      if (e.shiftKey && document.activeElement === firstEl) {
        e.preventDefault();
        lastEl.focus();
      } else if (!e.shiftKey && document.activeElement === lastEl) {
        e.preventDefault();
        firstEl.focus();
      }
    };
    document.addEventListener("keydown", onTrap);
    return () => {
      document.removeEventListener("keydown", onTrap);
      prev?.focus?.();
    };
  }, [snap]);

  const isFull = snap === "full";
  const showBackdrop = isFull;

  return (
    <>
      {showBackdrop ? (
        <div
          aria-hidden="true"
          className="pointer-events-auto fixed inset-0 z-40 bg-black/40 transition-opacity"
          onClick={() => onSnapChange("half")}
        />
      ) : null}
      <aside
        ref={sheetRef}
        role="dialog"
        aria-modal={isFull ? "true" : undefined}
        aria-labelledby={titleId}
        aria-label={label}
        className={cn(
          "fixed inset-x-0 bottom-0 z-50 flex flex-col rounded-t-2xl bg-background text-foreground shadow-chrome ring-1 ring-border",
          dragOffsetPct === null ? "transition-[height] duration-200 ease-out" : "",
          className,
        )}
        style={{ height: `${heightPct}vh`, touchAction: "none" }}
      >
        <div
          role="button"
          tabIndex={0}
          aria-label="Drag handle — move up or down to resize the panel"
          data-sheet-focus-initial
          onPointerDown={onPointerDown}
          onPointerMove={onPointerMove}
          onPointerUp={onPointerUp}
          onPointerCancel={onPointerUp}
          onKeyDown={(e) => {
            const idx = SNAP_ORDER.indexOf(snap);
            if (e.key === "ArrowUp" && idx < SNAP_ORDER.length - 1) {
              e.preventDefault();
              onSnapChange(SNAP_ORDER[idx + 1]);
            } else if (e.key === "ArrowDown" && idx > 0) {
              e.preventDefault();
              onSnapChange(SNAP_ORDER[idx - 1]);
            }
          }}
          className="flex cursor-grab touch-none select-none flex-col items-center py-2 focus:outline-none focus-visible:ring-2 focus-visible:ring-ring active:cursor-grabbing"
        >
          <span
            aria-hidden="true"
            className="h-1 w-10 rounded-full bg-muted-foreground/40"
          />
        </div>
        <div id={titleId} className="sr-only">
          {label}
        </div>
        {header ? (
          <div className="flex items-center gap-2 border-b border-border px-3 pb-2">
            {header}
          </div>
        ) : null}
        <div className="min-h-0 flex-1 overflow-y-auto">{children}</div>
      </aside>
    </>
  );
}

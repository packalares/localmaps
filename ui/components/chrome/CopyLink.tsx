"use client";

import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type MouseEvent,
} from "react";
import { Link2, Check } from "lucide-react";
import { useMapStore } from "@/lib/state/map";
import { usePlaceStore } from "@/lib/state/place";
import { useDirectionsStore } from "@/lib/state/directions";
import {
  buildShareUrl,
  encodeState,
  type ShareableState,
} from "@/lib/url-state/index";
import { Toast, ToastDescription, ToastTitle } from "@/components/ui/toast";
import { cn } from "@/lib/utils";

/**
 * Snapshot-at-click share button. On every click we rebuild a
 * ShareableState from the stores, encode it, then hand the absolute URL
 * to `navigator.clipboard.writeText`. On overflow we notify the user the
 * link is too long and (per spec) advise switching to the short-link
 * dialog Agent R provides later; when clipboard is unavailable we fall
 * back to a window.prompt so the URL is still retrievable.
 *
 * A11y:
 * - `aria-label="Copy link"` for screen readers.
 * - Live region toast announces "Link copied" on success / errors.
 */
export interface CopyLinkProps {
  /** When provided, used instead of window.location for origin + pathname. */
  baseUrl?: { origin: string; pathname: string };
  /** Tailwind override for the trigger. */
  className?: string;
}

type ToastKind = "ok" | "err" | "long";

interface ToastState {
  kind: ToastKind;
  message: string;
  at: number;
}

export function CopyLink({ baseUrl, className }: CopyLinkProps) {
  const [toast, setToast] = useState<ToastState | null>(null);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const buildState = useCallback((): ShareableState => {
    const m = useMapStore.getState();
    const d = useDirectionsStore.getState();
    const p = usePlaceStore.getState();
    const hasWaypoint = d.waypoints.some((wp) => wp.lngLat !== null);
    // The canonical "selected place" is `usePlaceStore.selectedFeature`.
    // We only round-trip a POI id (kind === "poi"); plain points carry
    // no stable id, so the share link relies on the viewport hash for
    // them.
    const selectedPoiId =
      p.selectedFeature?.kind === "poi" && p.selectedFeature.id
        ? p.selectedFeature.id
        : null;
    return {
      viewport: m.viewport,
      activeRegion: m.activeRegion,
      leftRailTab: m.leftRailTab,
      selectedPoiId,
      route: hasWaypoint
        ? {
            mode: d.mode,
            waypoints: d.waypoints
              .filter((wp) => wp.lngLat !== null)
              .map((wp) => ({ lng: wp.lngLat!.lng, lat: wp.lngLat!.lat })),
            options: d.options,
          }
        : null,
    };
  }, []);

  const flash = useCallback((kind: ToastKind, message: string) => {
    setToast({ kind, message, at: Date.now() });
    if (timerRef.current) clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => setToast(null), 2500);
  }, []);

  const onClick = useCallback(
    async (e: MouseEvent<HTMLButtonElement>) => {
      e.preventDefault();
      if (typeof window === "undefined") return;
      const origin = baseUrl?.origin ?? window.location.origin;
      const pathname = baseUrl?.pathname ?? window.location.pathname;
      const encoded = encodeState(buildState());
      const url = buildShareUrl(origin, pathname, encoded);

      if (encoded.overBudget) {
        flash(
          "long",
          "Link is too long for direct copy — use the short-link dialog instead.",
        );
        return;
      }

      try {
        if (
          typeof navigator !== "undefined" &&
          navigator.clipboard &&
          typeof navigator.clipboard.writeText === "function"
        ) {
          await navigator.clipboard.writeText(url);
          flash("ok", "Link copied to clipboard");
          return;
        }
      } catch (err) {
        console.warn("CopyLink: clipboard write failed", err);
      }
      // Fallback: put the URL in a prompt so the user can copy manually.
      try {
        window.prompt("Copy this link", url);
        flash("ok", "Link ready to copy");
      } catch {
        flash("err", "Could not copy link");
      }
    },
    [baseUrl, buildState, flash],
  );

  // Keyboard shortcut: Alt+Shift+C copies the link. We avoid Cmd/Ctrl-L
  // because that reliably collides with the browser address bar.
  useEffect(() => {
    const handler = (e: globalThis.KeyboardEvent) => {
      if (e.altKey && e.shiftKey && (e.key === "c" || e.key === "C")) {
        e.preventDefault();
        onClick({
          preventDefault: () => {},
        } as unknown as MouseEvent<HTMLButtonElement>);
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [onClick]);

  useEffect(() => {
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, []);

  const copied = toast?.kind === "ok";

  return (
    <>
      <button
        type="button"
        aria-label="Copy link"
        title="Copy link (Alt+Shift+C)"
        data-testid="copy-link"
        onClick={onClick}
        className={cn(
          "chrome-card inline-flex h-10 w-10 items-center justify-center rounded-md text-foreground hover:bg-muted focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
          className,
        )}
      >
        {copied ? (
          <Check className="h-4 w-4" aria-hidden="true" />
        ) : (
          <Link2 className="h-4 w-4" aria-hidden="true" />
        )}
      </button>
      {toast ? (
        <Toast
          key={toast.at}
          open
          onOpenChange={(open) => {
            if (!open) setToast(null);
          }}
          variant={toast.kind === "err" ? "destructive" : "default"}
          role="status"
        >
          <div className="flex flex-col gap-1">
            <ToastTitle>
              {toast.kind === "ok"
                ? "Copied"
                : toast.kind === "long"
                ? "Link too long"
                : "Copy failed"}
            </ToastTitle>
            <ToastDescription>{toast.message}</ToastDescription>
          </div>
        </Toast>
      ) : null}
    </>
  );
}

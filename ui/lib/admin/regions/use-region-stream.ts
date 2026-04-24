"use client";

import { useEffect, useRef } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { apiUrl } from "@/lib/env";
import {
  JobSchema,
  RegionSchema,
  WsEventSchema,
  type Job,
  type Region,
  type WsEvent,
} from "@/lib/api/schemas";

/**
 * Subscribes to the gateway's WebSocket and funnels `region.*` + `job.*`
 * events into the TanStack Query cache so table rows and progress cells
 * re-render without polling. If the socket cannot be opened or drops,
 * the hook falls back to the query's own `refetchInterval` (set by
 * the consumer); nothing else is done to retry — the server's own
 * backoff + a fresh page load bring us back in sync.
 *
 * See `contracts/openapi.yaml` `/api/ws` for the event shapes.
 */

export type RegionStreamStatus =
  | "idle"
  | "connecting"
  | "open"
  | "closed"
  | "error";

export interface RegionStreamOptions {
  /** Enable/disable the connection (e.g. when tab is hidden). Default true. */
  enabled?: boolean;
  /**
   * Optional callback for non-cache side effects — e.g. firing a toast
   * when a region becomes ready or fails.
   */
  onEvent?: (event: WsEvent) => void;
  /**
   * Factory override, primarily for tests: given a URL, return
   * something WebSocket-shaped. The default uses the global
   * `WebSocket`.
   */
  socketFactory?: (url: string) => WebSocket;
}

function wsUrlFor(path: string): string {
  const http = apiUrl(path);
  if (http.startsWith("ws://") || http.startsWith("wss://")) return http;
  if (http.startsWith("https://")) return "wss://" + http.slice("https://".length);
  if (http.startsWith("http://")) return "ws://" + http.slice("http://".length);
  if (typeof window !== "undefined") {
    const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
    return `${proto}//${window.location.host}${http.startsWith("/") ? http : `/${http}`}`;
  }
  return http;
}

/**
 * Apply a server event to the query cache. Exported for unit testing;
 * the hook body uses it internally.
 */
export function applyEventToCache(
  event: WsEvent,
  qc: ReturnType<typeof useQueryClient>,
): void {
  if (event.type.startsWith("region.")) {
    const region = event.data as Region;
    qc.setQueryData(["regions", "byName", region.name], region);
    qc.setQueryData<{ regions: Region[] } | undefined>(
      ["regions", "list"],
      (prev) => {
        if (!prev) return prev;
        const existing = prev.regions.findIndex((r) => r.name === region.name);
        if (existing === -1) return { regions: [...prev.regions, region] };
        const next = prev.regions.slice();
        next[existing] = region;
        return { regions: next };
      },
    );
    return;
  }
  if (event.type.startsWith("job.")) {
    const job = event.data as Job;
    qc.setQueryData(["jobs", "byId", job.id], job);
  }
}

export function useRegionStream(
  options: RegionStreamOptions = {},
): { statusRef: React.MutableRefObject<RegionStreamStatus> } {
  const qc = useQueryClient();
  const statusRef = useRef<RegionStreamStatus>("idle");
  const enabled = options.enabled ?? true;
  const onEvent = options.onEvent;
  const factory = options.socketFactory;

  useEffect(() => {
    if (!enabled) return;
    if (typeof window === "undefined") return;
    const WSImpl = factory
      ? null
      : (typeof WebSocket !== "undefined" ? WebSocket : null);
    if (!factory && !WSImpl) return;

    statusRef.current = "connecting";
    const url = wsUrlFor("/api/ws");
    let ws: WebSocket;
    try {
      ws = factory ? factory(url) : new (WSImpl as typeof WebSocket)(url);
    } catch {
      statusRef.current = "error";
      return;
    }

    ws.onopen = () => {
      statusRef.current = "open";
    };
    ws.onmessage = (ev: MessageEvent) => {
      let payload: unknown;
      try {
        payload = typeof ev.data === "string" ? JSON.parse(ev.data) : ev.data;
      } catch {
        return;
      }
      const parsed = WsEventSchema.safeParse(payload);
      if (!parsed.success) return;
      // Extra schema insurance on the inner data.
      if (parsed.data.type.startsWith("region.")) {
        const r = RegionSchema.safeParse(parsed.data.data);
        if (!r.success) return;
      } else {
        const j = JobSchema.safeParse(parsed.data.data);
        if (!j.success) return;
      }
      applyEventToCache(parsed.data, qc);
      onEvent?.(parsed.data);
    };
    ws.onerror = () => {
      statusRef.current = "error";
    };
    ws.onclose = () => {
      statusRef.current = "closed";
    };

    return () => {
      try {
        ws.close();
      } catch {
        // socket never reached OPEN; nothing to close.
      }
    };
  }, [enabled, factory, onEvent, qc]);

  return { statusRef };
}

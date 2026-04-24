"use client";

import { useEffect, useState } from "react";
import {
  Toast,
  ToastDescription,
  ToastTitle,
} from "@/components/ui/toast";
import { Button } from "@/components/ui/button";

/**
 * Registers `/sw.js` once on mount. When the browser reports a
 * `waiting` SW (i.e. a new version is ready but held behind the current
 * page), we surface a toast with a "Reload" button that posts
 * `skipWaiting` and refreshes so the new SW takes over cleanly.
 *
 * Gracefully no-ops in:
 *  - browsers without the serviceWorker API
 *  - development (Next sometimes serves a different /sw.js than prod)
 *  - SSR (guarded by typeof window)
 */
export function PwaRegister() {
  const [pending, setPending] = useState<ServiceWorker | null>(null);

  useEffect(() => {
    if (typeof window === "undefined") return;
    if (!("serviceWorker" in navigator)) return;
    // In dev, Next injects HMR that doesn't play nicely with a SW taking
    // over /_next/*. We still register so developers can exercise the
    // code path manually, but the SW is no-op in dev because Next serves
    // different hashed URLs on every reload — the cache never hits.
    let cancelled = false;

    const register = async () => {
      try {
        const reg = await navigator.serviceWorker.register("/sw.js", {
          scope: "/",
        });
        if (cancelled) return;
        if (reg.waiting) setPending(reg.waiting);
        reg.addEventListener("updatefound", () => {
          const installing = reg.installing;
          if (!installing) return;
          installing.addEventListener("statechange", () => {
            if (
              installing.state === "installed" &&
              navigator.serviceWorker.controller
            ) {
              setPending(installing);
            }
          });
        });
      } catch {
        // Browsers may block SW (private mode, insecure origin). Silent.
      }
    };

    void register();

    // When a new SW activates, reload once so the page matches.
    let reloaded = false;
    const onControllerChange = () => {
      if (reloaded) return;
      reloaded = true;
      window.location.reload();
    };
    navigator.serviceWorker.addEventListener(
      "controllerchange",
      onControllerChange,
    );

    return () => {
      cancelled = true;
      navigator.serviceWorker.removeEventListener(
        "controllerchange",
        onControllerChange,
      );
    };
  }, []);

  if (!pending) return null;

  return (
    <Toast
      open
      onOpenChange={(open) => {
        if (!open) setPending(null);
      }}
      role="status"
    >
      <div className="flex items-center gap-3">
        <div className="flex flex-col gap-1">
          <ToastTitle>Update ready</ToastTitle>
          <ToastDescription>
            A new version of LocalMaps is available.
          </ToastDescription>
        </div>
        <Button
          size="sm"
          onClick={() => {
            pending.postMessage({ type: "skipWaiting" });
          }}
        >
          Reload
        </Button>
      </div>
    </Toast>
  );
}

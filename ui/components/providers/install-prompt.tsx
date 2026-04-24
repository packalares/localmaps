"use client";

import { useCallback, useEffect, useState } from "react";
import { Download } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

/**
 * The install prompt. Listens for `beforeinstallprompt`, stashes the
 * event, and exposes a small button that triggers the browser install
 * flow on click.
 *
 * If the event never fires (Safari, installed PWA, Firefox desktop) the
 * component renders nothing. We never persist a dismissal — if the user
 * hides the button once it's back on the next visit, which matches how
 * Chrome itself behaves.
 */

interface BeforeInstallPromptEvent extends Event {
  readonly platforms: string[];
  prompt(): Promise<void>;
  readonly userChoice: Promise<{
    outcome: "accepted" | "dismissed";
    platform: string;
  }>;
}

export function InstallPrompt({ className }: { className?: string }) {
  const [evt, setEvt] = useState<BeforeInstallPromptEvent | null>(null);
  const [installed, setInstalled] = useState(false);

  useEffect(() => {
    if (typeof window === "undefined") return;

    const onPrompt = (e: Event) => {
      e.preventDefault();
      setEvt(e as BeforeInstallPromptEvent);
    };
    const onInstalled = () => {
      setInstalled(true);
      setEvt(null);
    };

    window.addEventListener("beforeinstallprompt", onPrompt);
    window.addEventListener("appinstalled", onInstalled);

    // Already running as an installed PWA? Hide the prompt.
    const mq = window.matchMedia?.("(display-mode: standalone)");
    if (mq?.matches) setInstalled(true);

    return () => {
      window.removeEventListener("beforeinstallprompt", onPrompt);
      window.removeEventListener("appinstalled", onInstalled);
    };
  }, []);

  const onInstall = useCallback(async () => {
    if (!evt) return;
    try {
      await evt.prompt();
      await evt.userChoice;
    } finally {
      // `prompt()` can only be called once — drop our reference either way.
      setEvt(null);
    }
  }, [evt]);

  if (installed || !evt) return null;

  return (
    <Button
      variant="outline"
      size="sm"
      onClick={onInstall}
      className={cn("gap-1 px-2", className)}
      aria-label="Install LocalMaps as an app"
    >
      <Download className="h-4 w-4" aria-hidden="true" />
      <span className="text-xs">Install</span>
    </Button>
  );
}

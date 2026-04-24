"use client";

import { useEffect, useMemo, useState } from "react";
import { Link as LinkIcon, QrCode as QrIcon, Code2 } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { cn } from "@/lib/utils";
import { useCreateShortLink } from "@/lib/api/hooks";
import { QrCode } from "./QrCode";
import { EmbedSnippet } from "./EmbedSnippet";
import { LinkPanel } from "./LinkPanel";

/**
 * Three-tab share dialog (Link / QR / Embed), modelled on the equivalent
 * Google Maps affordance and the Phase-5 spec in the project charter.
 *
 * The dialog is a thin orchestrator: it computes the long URL from the
 * window location at open time, lets the user promote it to a short URL
 * via `POST /api/links`, and threads the resulting URL through the QR
 * and Embed tabs. All three rendering primitives live in sibling files
 * (QrCode / EmbedSnippet / LinkPanel) so their tests can exercise them
 * in isolation.
 */
export interface ShareDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Size (px) passed to <QrCode>. Default 256 (share.qrCodeSizePx). */
  qrSize?: number;
}

type Tab = "link" | "qr" | "embed";

export function ShareDialog({
  open,
  onOpenChange,
  qrSize = 256,
}: ShareDialogProps) {
  const [tab, setTab] = useState<Tab>("link");
  const [longUrl, setLongUrl] = useState("");
  const [shortUrl, setShortUrl] = useState<string | null>(null);
  const createLink = useCreateShortLink();

  // Capture the current URL each time the dialog opens. Reading from
  // window.location keeps this component decoupled from the map-state
  // store — the URL-state hook (ui/lib/url-state.ts) already owns the
  // bidirectional sync; we only read the side-effect here.
  //
  // NOTE: `createLink` is intentionally *not* in the dep list. TanStack
  // Query returns a new mutation-state object every render, so including
  // it would cause an infinite useState → re-render → reset loop. The
  // closure's `.reset()` is stable across renders.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => {
    if (!open) return;
    setTab("link");
    setShortUrl(null);
    createLink.reset();
    if (typeof window !== "undefined") {
      setLongUrl(window.location.href);
    }
  }, [open]);

  // The URL fed to both QR and Embed tabs: short if we have one, else
  // the long URL so users get something scannable/embeddable on first
  // open — the short link is a nice-to-have, not a prerequisite.
  const shareUrl = shortUrl ?? longUrl;

  // The embed iframe points at the server's `/embed` path (contracts/
  // openapi.yaml — tag: share). When a short URL exists we use it and
  // the server resolves the redirect chain for us.
  const embedSrc = useMemo(() => {
    if (shortUrl) return shortUrl;
    if (typeof window === "undefined") return "";
    const u = new URL(window.location.href);
    return `${u.origin}/embed${u.search}${u.hash}`;
  }, [shortUrl]);

  async function handleCreateShort() {
    if (typeof window === "undefined") return;
    // POST a relative URL — the server's same-origin validator accepts
    // either form, and relatives keep the link working if the deploy
    // moves origins.
    const u = new URL(window.location.href);
    const relative = `${u.pathname}${u.search}${u.hash}` || "/";
    try {
      const res = await createLink.mutateAsync({ url: relative });
      setShortUrl(`${u.origin}/api/links/${res.code}`);
    } catch {
      // Error state surfaces via createLink.isError in the LinkPanel.
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md gap-4">
        <DialogHeader>
          <DialogTitle>Share this view</DialogTitle>
          <DialogDescription>
            Copy a link to the current map state, scan a QR code, or
            embed this view on another site.
          </DialogDescription>
        </DialogHeader>

        <div role="tablist" aria-label="Share options" className="flex gap-1">
          <TabTrigger
            active={tab === "link"}
            onClick={() => setTab("link")}
            icon={<LinkIcon className="h-4 w-4" aria-hidden="true" />}
            label="Link"
          />
          <TabTrigger
            active={tab === "qr"}
            onClick={() => setTab("qr")}
            icon={<QrIcon className="h-4 w-4" aria-hidden="true" />}
            label="QR"
          />
          <TabTrigger
            active={tab === "embed"}
            onClick={() => setTab("embed")}
            icon={<Code2 className="h-4 w-4" aria-hidden="true" />}
            label="Embed"
          />
        </div>

        {tab === "link" && (
          <LinkPanel
            longUrl={longUrl}
            shortUrl={shortUrl}
            onCreateShort={handleCreateShort}
            creating={createLink.isPending}
            errored={createLink.isError}
          />
        )}
        {tab === "qr" && (
          <div className="flex justify-center py-2">
            <QrCode value={shareUrl} size={qrSize} />
          </div>
        )}
        {tab === "embed" && <EmbedSnippet src={embedSrc} />}
      </DialogContent>
    </Dialog>
  );
}

function TabTrigger(props: {
  active: boolean;
  onClick: () => void;
  icon: React.ReactNode;
  label: string;
}) {
  return (
    <button
      role="tab"
      type="button"
      aria-selected={props.active}
      onClick={props.onClick}
      className={cn(
        "flex flex-1 items-center justify-center gap-2 rounded-md px-3 py-2 text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
        props.active
          ? "bg-primary/10 text-primary"
          : "text-foreground hover:bg-muted",
      )}
    >
      {props.icon}
      <span>{props.label}</span>
    </button>
  );
}

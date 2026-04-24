"use client";

import { useState } from "react";
import { Share2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ShareDialog } from "./ShareDialog";

/**
 * The Share chrome button + its dialog wiring. Exposed as a single
 * component so parent chrome (LeftRail, FabStack, etc.) can drop it in
 * without juggling open-state. The dialog instance is lazy in spirit —
 * it mounts in the portal only when `open` is true, but keeping it in
 * the tree lets Radix manage focus traps + ESC without a second component.
 */
export interface ShareButtonProps {
  /** Optional className applied to the trigger button. */
  className?: string;
  /** QR code pixel size passed through to the dialog (share.qrCodeSizePx). */
  qrSize?: number;
}

export function ShareButton({ className, qrSize }: ShareButtonProps) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button
        type="button"
        variant="chrome"
        size="icon"
        aria-label="Share this view"
        title="Share this view"
        onClick={() => setOpen(true)}
        className={className}
      >
        <Share2 className="h-4 w-4" aria-hidden="true" />
      </Button>
      <ShareDialog open={open} onOpenChange={setOpen} qrSize={qrSize} />
    </>
  );
}

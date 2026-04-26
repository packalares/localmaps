"use client";

import * as React from "react";
import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { cn } from "@/lib/utils";

const TooltipProvider = TooltipPrimitive.Provider;
const Tooltip = TooltipPrimitive.Root;
const TooltipTrigger = TooltipPrimitive.Trigger;

/**
 * Tooltip body. Rendered through Radix's Portal so the tooltip lives at
 * the document root and isn't trapped inside the LeftIconRail's
 * `z-30` stacking context. The explicit `z-[60]` keeps the tooltip
 * above the chrome layers (LeftRail z-20, LeftIconRail / SearchBar
 * z-30, LayersCard z-30) without needing to fight stacking-context
 * tricks. Without the Portal, hover tooltips on the avatar buttons
 * appeared BEHIND the search-bar overlay because both share z-30 and
 * the search bar comes later in DOM order.
 */
const TooltipContent = React.forwardRef<
  React.ElementRef<typeof TooltipPrimitive.Content>,
  React.ComponentPropsWithoutRef<typeof TooltipPrimitive.Content>
>(({ className, sideOffset = 4, ...props }, ref) => (
  <TooltipPrimitive.Portal>
    <TooltipPrimitive.Content
      ref={ref}
      sideOffset={sideOffset}
      className={cn(
        "z-[60] overflow-hidden rounded-md bg-foreground px-3 py-1.5 text-xs text-background shadow",
        className,
      )}
      {...props}
    />
  </TooltipPrimitive.Portal>
));
TooltipContent.displayName = TooltipPrimitive.Content.displayName;

export { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger };

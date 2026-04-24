"use client";

import { ToastProvider, ToastViewport } from "./toast";
import type { ReactNode } from "react";

/**
 * Wraps the app in Radix's ToastProvider + the viewport. Phase-1 we only
 * mount the infrastructure; the actual `useToast()` hook that pushes
 * toasts will be added in a later phase alongside the first piece of
 * code that needs to surface an error.
 */
export function Toaster({ children }: { children?: ReactNode }) {
  return (
    <ToastProvider swipeDirection="right">
      {children}
      <ToastViewport />
    </ToastProvider>
  );
}

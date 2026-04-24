"use client";

import { cn } from "@/lib/utils";

export interface FieldBooleanProps {
  id: string;
  value: boolean;
  onChange: (v: boolean) => void;
  disabled?: boolean;
  label?: string;
}

/**
 * Minimal accessible switch. Implemented as a role="switch" button so
 * keyboard + screen-reader users get the right semantics without
 * pulling in another dependency.
 */
export function FieldBoolean({
  id,
  value,
  onChange,
  disabled,
  label,
}: FieldBooleanProps) {
  return (
    <button
      id={id}
      type="button"
      role="switch"
      aria-checked={value}
      aria-label={label}
      disabled={disabled}
      onClick={() => onChange(!value)}
      className={cn(
        "relative inline-flex h-6 w-10 shrink-0 items-center rounded-full transition-colors",
        "focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
        "disabled:cursor-not-allowed disabled:opacity-50",
        value ? "bg-primary" : "bg-muted",
      )}
    >
      <span
        className={cn(
          "inline-block h-5 w-5 transform rounded-full bg-background shadow transition-transform",
          value ? "translate-x-4" : "translate-x-0.5",
        )}
      />
    </button>
  );
}

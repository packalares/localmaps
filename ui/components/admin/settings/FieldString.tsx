"use client";

import { cn } from "@/lib/utils";

export interface FieldStringProps {
  id: string;
  value: string;
  onChange: (v: string) => void;
  disabled?: boolean;
  "aria-invalid"?: boolean;
  placeholder?: string;
}

/** Plain text input shared by TypeString and the single-line fallback. */
export function FieldString({
  id,
  value,
  onChange,
  disabled,
  placeholder,
  ...rest
}: FieldStringProps) {
  return (
    <input
      id={id}
      type="text"
      value={value}
      disabled={disabled}
      placeholder={placeholder}
      onChange={(ev) => onChange(ev.target.value)}
      aria-invalid={rest["aria-invalid"]}
      className={cn(
        "h-9 w-full rounded-md border border-input bg-background px-3 text-sm",
        "focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
        "disabled:opacity-50",
        rest["aria-invalid"] ? "border-destructive" : "",
      )}
    />
  );
}

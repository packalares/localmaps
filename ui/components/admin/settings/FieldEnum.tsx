"use client";

import { cn } from "@/lib/utils";

export interface FieldEnumProps {
  id: string;
  value: string;
  options: readonly string[];
  onChange: (v: string) => void;
  disabled?: boolean;
  "aria-invalid"?: boolean;
}

/** Native <select> keeps keyboard + accessibility behaviour simple. */
export function FieldEnum({
  id,
  value,
  options,
  onChange,
  disabled,
  ...rest
}: FieldEnumProps) {
  return (
    <select
      id={id}
      value={value}
      disabled={disabled}
      onChange={(ev) => onChange(ev.target.value)}
      aria-invalid={rest["aria-invalid"]}
      className={cn(
        "h-9 w-full rounded-md border border-input bg-background px-3 text-sm",
        "focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
        "disabled:opacity-50",
        rest["aria-invalid"] ? "border-destructive" : "",
      )}
    >
      {options.map((opt) => (
        <option key={opt} value={opt}>
          {opt}
        </option>
      ))}
    </select>
  );
}

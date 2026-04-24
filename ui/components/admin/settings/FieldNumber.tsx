"use client";

import { cn } from "@/lib/utils";

export interface FieldNumberProps {
  id: string;
  value: number | string;
  onChange: (v: number | string) => void;
  step?: number;
  min?: number;
  max?: number;
  integer?: boolean;
  disabled?: boolean;
  "aria-invalid"?: boolean;
}

/** Number/integer input. Emits `""` when the user empties the field so
 *  the parent can distinguish "unset" from "0". */
export function FieldNumber({
  id,
  value,
  onChange,
  step,
  min,
  max,
  integer,
  disabled,
  ...rest
}: FieldNumberProps) {
  return (
    <input
      id={id}
      type="number"
      value={value}
      step={step ?? (integer ? 1 : "any")}
      min={min}
      max={max}
      disabled={disabled}
      onChange={(ev) => {
        const raw = ev.target.value;
        if (raw === "") {
          onChange("");
          return;
        }
        const parsed = integer ? parseInt(raw, 10) : parseFloat(raw);
        if (Number.isNaN(parsed)) {
          onChange(raw);
          return;
        }
        onChange(parsed);
      }}
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

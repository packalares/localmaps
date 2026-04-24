"use client";

import { useEffect, useState } from "react";
import { stringifyValue, tryParseJson } from "@/lib/admin/settings/format";
import { cn } from "@/lib/utils";

export interface FieldJsonProps {
  id: string;
  value: unknown;
  onChange: (v: unknown) => void;
  disabled?: boolean;
  "aria-invalid"?: boolean;
}

/**
 * JSON textarea used for object-typed settings (e.g. map.defaultCenter)
 * and arrays of objects (e.g. auth.basicUsers). The field keeps its own
 * text buffer so the user can type through a transiently-invalid state
 * — we only propagate the parsed value back up when the JSON is valid.
 */
export function FieldJson({ id, value, onChange, disabled, ...rest }: FieldJsonProps) {
  const [text, setText] = useState(() => stringifyValue(value));
  const [localError, setLocalError] = useState<string | null>(null);

  useEffect(() => {
    setText(stringifyValue(value));
  }, [value]);

  return (
    <div className="flex flex-col gap-1">
      <textarea
        id={id}
        value={text}
        disabled={disabled}
        onChange={(ev) => {
          const raw = ev.target.value;
          setText(raw);
          const parsed = tryParseJson(raw);
          if (parsed.ok) {
            setLocalError(null);
            onChange(parsed.value);
          } else {
            setLocalError(parsed.error);
          }
        }}
        aria-invalid={rest["aria-invalid"] || !!localError}
        rows={6}
        spellCheck={false}
        className={cn(
          "w-full rounded-md border border-input bg-background p-2 font-mono text-xs",
          "focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
          "disabled:opacity-50",
          (rest["aria-invalid"] || localError) ? "border-destructive" : "",
        )}
      />
      {localError ? (
        <p className="text-xs text-destructive" role="alert">
          Invalid JSON: {localError}
        </p>
      ) : null}
    </div>
  );
}

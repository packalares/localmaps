"use client";

import { ArrowDown, ArrowUp, Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export interface FieldStringArrayProps {
  id: string;
  value: readonly string[];
  onChange: (next: string[]) => void;
  disabled?: boolean;
}

/**
 * Editable list of strings with add / remove / reorder affordances.
 * Designed for small lists (egress hosts, languages, POI categories).
 * Uses in-place editing of each row via a plain <input>.
 */
export function FieldStringArray({
  id,
  value,
  onChange,
  disabled,
}: FieldStringArrayProps) {
  const setAt = (i: number, next: string) => {
    const arr = value.slice();
    arr[i] = next;
    onChange(arr);
  };
  const remove = (i: number) => {
    const arr = value.slice();
    arr.splice(i, 1);
    onChange(arr);
  };
  const move = (i: number, delta: number) => {
    const j = i + delta;
    if (j < 0 || j >= value.length) return;
    const arr = value.slice();
    [arr[i], arr[j]] = [arr[j], arr[i]];
    onChange(arr);
  };
  const add = () => onChange([...value, ""]);

  return (
    <div id={id} className="flex flex-col gap-1.5" aria-label="String list editor">
      {value.length === 0 ? (
        <p className="text-xs italic text-muted-foreground">
          No entries. Click Add to create one.
        </p>
      ) : null}
      {value.map((item, i) => (
        <div key={i} className="flex items-center gap-1.5">
          <input
            type="text"
            value={item}
            disabled={disabled}
            onChange={(ev) => setAt(i, ev.target.value)}
            aria-label={`Item ${i + 1}`}
            className={cn(
              "h-8 flex-1 rounded-md border border-input bg-background px-2 text-sm",
              "focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
              "disabled:opacity-50",
            )}
          />
          <Button
            type="button"
            size="sm"
            variant="ghost"
            disabled={disabled || i === 0}
            onClick={() => move(i, -1)}
            aria-label={`Move item ${i + 1} up`}
            className="h-8 w-8 p-0"
          >
            <ArrowUp className="h-3.5 w-3.5" />
          </Button>
          <Button
            type="button"
            size="sm"
            variant="ghost"
            disabled={disabled || i === value.length - 1}
            onClick={() => move(i, 1)}
            aria-label={`Move item ${i + 1} down`}
            className="h-8 w-8 p-0"
          >
            <ArrowDown className="h-3.5 w-3.5" />
          </Button>
          <Button
            type="button"
            size="sm"
            variant="ghost"
            disabled={disabled}
            onClick={() => remove(i)}
            aria-label={`Remove item ${i + 1}`}
            className="h-8 w-8 p-0 text-destructive"
          >
            <Trash2 className="h-3.5 w-3.5" />
          </Button>
        </div>
      ))}
      <div>
        <Button
          type="button"
          size="sm"
          variant="outline"
          disabled={disabled}
          onClick={add}
          className="h-8"
        >
          <Plus className="mr-1 h-3.5 w-3.5" /> Add
        </Button>
      </div>
    </div>
  );
}

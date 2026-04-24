"use client";

import { useState } from "react";
import { Calendar, Check, ChevronDown } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  formatSchedule,
  isValidCron,
  parseSchedule,
  PRESET_SCHEDULES,
} from "@/lib/admin/regions/format-schedule";
import { cn } from "@/lib/utils";

/**
 * Schedule picker: Never / Daily / Weekly / Monthly + Custom (5-field cron).
 * Parent owns the value; this component renders the current choice +
 * commits the next choice through onChange. `disabled` grays out the
 * trigger while a mutation is in flight.
 */
export interface ScheduleDropdownProps {
  value: string | null | undefined;
  onChange: (next: string) => void;
  disabled?: boolean;
}

export function ScheduleDropdown({
  value,
  onChange,
  disabled,
}: ScheduleDropdownProps) {
  const parsed = parseSchedule(value);
  const [open, setOpen] = useState(false);
  const [cron, setCron] = useState(parsed.kind === "custom" ? parsed.cron ?? "" : "");
  const [cronError, setCronError] = useState<string | null>(null);

  const commitCustom = () => {
    if (!isValidCron(cron)) {
      setCronError("Enter a 5-field cron expression");
      return;
    }
    setCronError(null);
    onChange(cron.trim());
    setOpen(false);
  };

  return (
    <DropdownMenu open={open} onOpenChange={setOpen}>
      <DropdownMenuTrigger asChild>
        <Button
          variant="outline"
          size="sm"
          disabled={disabled}
          aria-label="Change update schedule"
          className="justify-between gap-2"
        >
          <Calendar className="h-3.5 w-3.5" aria-hidden="true" />
          <span className="min-w-0 truncate">{formatSchedule(value)}</span>
          <ChevronDown className="h-3.5 w-3.5 opacity-70" aria-hidden="true" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-[14rem] p-1">
        {PRESET_SCHEDULES.map((preset) => (
          <DropdownMenuItem
            key={preset}
            onSelect={(ev) => {
              ev.preventDefault();
              onChange(preset);
              setOpen(false);
            }}
            className="capitalize"
          >
            <span className="flex w-5 items-center justify-center">
              {parsed.kind === preset ? (
                <Check className="h-4 w-4" aria-hidden="true" />
              ) : null}
            </span>
            <span>{preset}</span>
          </DropdownMenuItem>
        ))}
        <DropdownMenuSeparator />
        <div className="px-2 py-2">
          <label className="mb-1 block text-xs font-medium text-muted-foreground">
            Custom cron (5 fields)
          </label>
          <input
            type="text"
            placeholder="0 4 * * 0"
            aria-label="Custom cron expression"
            value={cron}
            onChange={(ev) => {
              setCron(ev.target.value);
              setCronError(null);
            }}
            onKeyDown={(ev) => {
              if (ev.key === "Enter") {
                ev.preventDefault();
                commitCustom();
              }
            }}
            className={cn(
              "h-8 w-full rounded-md border border-input bg-background px-2 text-sm",
              "focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
              cronError && "border-destructive",
            )}
          />
          {cronError ? (
            <p className="mt-1 text-xs text-destructive" role="alert">
              {cronError}
            </p>
          ) : null}
          <Button
            size="sm"
            className="mt-2 w-full"
            type="button"
            onClick={commitCustom}
          >
            Apply custom
          </Button>
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

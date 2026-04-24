"use client";

import { useMemo, useState } from "react";
import { ChevronDown, Clock } from "lucide-react";
import {
  parseOpeningHours,
  statusLabel,
  type DaySchedule,
} from "@/lib/poi/opening-hours";
import { cn } from "@/lib/utils";

export interface HoursAccordionProps {
  /** Raw OSM `opening_hours` tag. */
  raw: string | null | undefined;
  /** Testing seam — defaults to new Date(). */
  now?: Date;
  className?: string;
}

const WEEKDAY_LABELS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

function formatTime(d: Date): string {
  return `${String(d.getHours()).padStart(2, "0")}:${String(
    d.getMinutes(),
  ).padStart(2, "0")}`;
}

function formatDayRow(day: DaySchedule): string {
  if (day.closed || day.ranges.length === 0) return "Closed";
  return day.ranges
    .map((r) => {
      const s = `${String(Math.floor(r.startMin / 60)).padStart(2, "0")}:${String(
        r.startMin % 60,
      ).padStart(2, "0")}`;
      const endMin = r.endMin > 1440 ? r.endMin - 1440 : r.endMin;
      const e = `${String(Math.floor(endMin / 60)).padStart(2, "0")}:${String(
        endMin % 60,
      ).padStart(2, "0")}`;
      return `${s}–${e}`;
    })
    .join(", ");
}

/**
 * Expandable opening-hours panel. Collapsed: single summary line
 * showing today's status (e.g. "Open now · closes 22:00"). Expanded:
 * full week grid with today's row highlighted.
 *
 * Renders nothing if the raw tag is missing, so callers can drop it in
 * and let it decide.
 */
export function HoursAccordion({
  raw,
  now = new Date(),
  className,
}: HoursAccordionProps) {
  const [open, setOpen] = useState(false);
  const parsed = useMemo(() => parseOpeningHours(raw ?? ""), [raw]);

  if (!raw || !raw.trim()) return null;

  const status = statusLabel(parsed, now, formatTime);
  const today = now.getDay();

  return (
    <section
      className={cn(
        "border-b border-border px-4 py-3 text-sm",
        className,
      )}
      aria-label="Opening hours"
    >
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        aria-controls="poi-hours-body"
        className="flex w-full items-center justify-between gap-2 rounded-md py-1 text-left hover:bg-muted focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <span className="flex items-center gap-2">
          <Clock className="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden />
          <span className="font-medium">{status}</span>
        </span>
        <ChevronDown
          className={cn(
            "h-4 w-4 shrink-0 text-muted-foreground transition-transform",
            open && "rotate-180",
          )}
          aria-hidden="true"
        />
      </button>

      {open && (
        <ul
          id="poi-hours-body"
          className="mt-2 space-y-1 text-sm"
        >
          {parsed.week.map((day) => (
            <li
              key={day.weekday}
              className={cn(
                "flex justify-between gap-4 rounded px-2 py-1",
                day.weekday === today && "bg-primary/10 text-foreground",
              )}
            >
              <span className="font-medium text-muted-foreground">
                {WEEKDAY_LABELS[day.weekday]}
              </span>
              <span>{formatDayRow(day)}</span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

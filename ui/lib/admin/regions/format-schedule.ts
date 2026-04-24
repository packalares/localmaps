/**
 * Region schedule helpers.
 *
 * The openapi `RegionSchedule` is simply a string that is either a
 * preset keyword (`never` / `daily` / `weekly` / `monthly`) or a
 * 5-field cron expression. This module converts between the raw string
 * and a UI-friendly {kind, cron?} representation, and produces a
 * human-readable label for a given value.
 */

export const PRESET_SCHEDULES = [
  "never",
  "daily",
  "weekly",
  "monthly",
] as const;
export type ScheduleKind = (typeof PRESET_SCHEDULES)[number] | "custom";

export interface ScheduleValue {
  kind: ScheduleKind;
  /** Only populated when kind === "custom". 5-field cron. */
  cron?: string;
}

const CRON_REGEX =
  /^\s*(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s*$/;

/** True if the argument is a syntactically acceptable 5-field cron. */
export function isValidCron(value: string): boolean {
  if (!value) return false;
  const m = CRON_REGEX.exec(value);
  if (!m) return false;
  // Shape-level validation only; the server is authoritative for
  // semantic validity.
  const fields = m.slice(1, 6);
  const allowedChars = /^[0-9*,\-/?LW#]+$/i;
  return fields.every((f) => allowedChars.test(f));
}

/** Parse a raw schedule string into the UI-friendly discriminated form. */
export function parseSchedule(raw: string | null | undefined): ScheduleValue {
  if (!raw) return { kind: "never" };
  const trimmed = raw.trim().toLowerCase();
  if ((PRESET_SCHEDULES as readonly string[]).includes(trimmed)) {
    return { kind: trimmed as ScheduleKind };
  }
  return { kind: "custom", cron: raw.trim() };
}

/** Re-encode a parsed schedule back to the on-wire string. */
export function serializeSchedule(value: ScheduleValue): string {
  if (value.kind === "custom") return value.cron ?? "";
  return value.kind;
}

/** Human-readable label for a schedule value. */
export function formatSchedule(raw: string | null | undefined): string {
  const parsed = parseSchedule(raw);
  switch (parsed.kind) {
    case "never":
      return "Never";
    case "daily":
      return "Daily";
    case "weekly":
      return "Weekly";
    case "monthly":
      return "Monthly";
    case "custom":
      return `Custom (${parsed.cron ?? ""})`;
  }
}

/**
 * Format an ISO timestamp for the "Next update" column. Falls back to
 * em-dash when the timestamp is missing or unparsable.
 */
export function formatNextUpdate(
  iso: string | null | undefined,
  now: Date = new Date(),
): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  const sameYear = d.getFullYear() === now.getFullYear();
  // "Sun 4:00 AM" same year, "Sun 4 Jan 2026" otherwise. We lean on the
  // browser locale so operator expectations follow their OS.
  return d.toLocaleString(undefined, {
    weekday: "short",
    hour: "numeric",
    minute: "2-digit",
    ...(sameYear ? {} : { year: "numeric", month: "short", day: "numeric" }),
  });
}

/**
 * Minimal OSM `opening_hours` parser. We deliberately do NOT depend on
 * `opening_hours.js` (which is a large dependency with a Moment-era
 * API); instead we implement the subset that covers the vast majority
 * of tags in OpenStreetMap.
 *
 * Supported grammar (a subset of the full spec at
 * https://wiki.openstreetmap.org/wiki/Key:opening_hours):
 *
 *   <rule>        ::= <days> " " <times>
 *   <rule-list>   ::= <rule> (";" <rule>)*     (last rule wins for a given day)
 *   <days>        ::= <day-range> ("," <day-range>)*
 *   <day-range>   ::= <day> | <day> "-" <day>
 *   <day>         ::= Mo|Tu|We|Th|Fr|Sa|Su|PH (PH ignored)
 *   <times>       ::= "off" | "closed" | <time-range> ("," <time-range>)*
 *   <time-range>  ::= HH:MM "-" HH:MM            (HH may be 00..24)
 *   <rule>        ::= "24/7"                      (all days all hours)
 *
 * Any unrecognised token causes the rule to be skipped (not the whole
 * string) — the function returns the best-effort parse so callers can
 * display "Hours unavailable" only when we truly can't understand
 * anything.
 */

export type Weekday = 0 | 1 | 2 | 3 | 4 | 5 | 6;
// Mo = 1 in our model (aligns with JS Date.getDay() where Sunday=0).

export interface TimeRange {
  /** Minutes from 00:00 on the day the rule applies to, inclusive. */
  startMin: number;
  /** Minutes from 00:00, exclusive. May be > 1440 for overnight. */
  endMin: number;
}

export interface DaySchedule {
  weekday: Weekday;
  ranges: TimeRange[];
  /** True if the rule explicitly declares this day closed. */
  closed: boolean;
}

export interface ParsedHours {
  /** Sunday..Saturday (index 0..6); always 7 entries. */
  week: DaySchedule[];
  /** 24/7 shortcut: store and short-circuit lookups. */
  alwaysOpen: boolean;
  /** The raw input, preserved for the UI's "show raw" fallback. */
  raw: string;
}

const DAY_TOKENS: Record<string, Weekday> = {
  Su: 0,
  Mo: 1,
  Tu: 2,
  We: 3,
  Th: 4,
  Fr: 5,
  Sa: 6,
};

function emptyWeek(): DaySchedule[] {
  return [0, 1, 2, 3, 4, 5, 6].map((d) => ({
    weekday: d as Weekday,
    ranges: [],
    closed: false,
  }));
}

function parseTime(s: string): number | null {
  const m = /^(\d{1,2}):(\d{2})$/.exec(s.trim());
  if (!m) return null;
  const hh = Number(m[1]);
  const mm = Number(m[2]);
  if (hh < 0 || hh > 24 || mm < 0 || mm > 59) return null;
  return hh * 60 + mm;
}

function parseDayRange(token: string): Weekday[] {
  token = token.trim();
  if (!token || token === "PH" || token === "SH") return []; // ignored
  const dashIdx = token.indexOf("-");
  if (dashIdx < 0) {
    const d = DAY_TOKENS[token];
    return d === undefined ? [] : [d];
  }
  const from = DAY_TOKENS[token.slice(0, dashIdx)];
  const to = DAY_TOKENS[token.slice(dashIdx + 1)];
  if (from === undefined || to === undefined) return [];
  const out: Weekday[] = [];
  let i = from;
  // Allow wrap-around (e.g. Fr-Mo).
  // Limit to 7 iterations defensively.
  for (let k = 0; k < 7; k++) {
    out.push(i as Weekday);
    if (i === to) break;
    i = ((i + 1) % 7) as Weekday;
  }
  return out;
}

function parseDays(spec: string): Weekday[] {
  const parts = spec.split(",").map((p) => p.trim()).filter(Boolean);
  const seen = new Set<Weekday>();
  for (const p of parts) for (const d of parseDayRange(p)) seen.add(d);
  return [...seen].sort() as Weekday[];
}

function parseTimeRanges(spec: string): TimeRange[] | "off" | null {
  const trimmed = spec.trim();
  if (trimmed === "off" || trimmed === "closed") return "off";
  const parts = trimmed.split(",").map((p) => p.trim()).filter(Boolean);
  const out: TimeRange[] = [];
  for (const p of parts) {
    const halves = p.split("-").map((x) => x.trim());
    if (halves.length !== 2) return null;
    const [a, b] = halves;
    if (!a || !b) return null;
    const start = parseTime(a);
    const endRaw = parseTime(b);
    if (start === null || endRaw === null) return null;
    let end = endRaw;
    if (end <= start) end += 24 * 60; // overnight
    out.push({ startMin: start, endMin: end });
  }
  return out;
}

export function parseOpeningHours(raw: string | undefined | null): ParsedHours {
  const input = (raw ?? "").trim();
  if (!input) return { week: emptyWeek(), alwaysOpen: false, raw: "" };
  if (input === "24/7") {
    return {
      week: emptyWeek().map((d) => ({
        ...d,
        ranges: [{ startMin: 0, endMin: 24 * 60 }],
      })),
      alwaysOpen: true,
      raw: input,
    };
  }

  const week = emptyWeek();
  const rules = input.split(";").map((r) => r.trim()).filter(Boolean);
  for (const rule of rules) {
    // Split on the first whitespace run after the day spec.
    const match = /^([A-Za-z0-9,\- ]+?)\s+(.+)$/.exec(rule);
    let daySpec: string;
    let timeSpec: string;
    if (!match) {
      // Bare "24/7" or just a time range with implicit all days.
      if (rule === "24/7") {
        for (const d of week) d.ranges.push({ startMin: 0, endMin: 1440 });
        continue;
      }
      daySpec = "Mo-Su";
      timeSpec = rule;
    } else {
      daySpec = match[1];
      timeSpec = match[2];
    }
    const days = parseDays(daySpec);
    if (!days.length) continue;
    const ranges = parseTimeRanges(timeSpec);
    if (ranges === null) continue;
    if (ranges === "off") {
      for (const d of days) {
        week[d].ranges = [];
        week[d].closed = true;
      }
    } else {
      for (const d of days) {
        week[d].ranges = ranges;
        week[d].closed = false;
      }
    }
  }
  return { week, alwaysOpen: false, raw: input };
}

export function isOpenAt(parsed: ParsedHours, when: Date): boolean {
  if (parsed.alwaysOpen) return true;
  const wd = when.getDay() as Weekday;
  const minutes = when.getHours() * 60 + when.getMinutes();
  const day = parsed.week[wd];
  if (day.closed) return false;
  for (const r of day.ranges) {
    if (minutes >= r.startMin && minutes < r.endMin) return true;
  }
  // Handle overnight carryover from the previous day.
  const prev = parsed.week[((wd + 6) % 7) as Weekday];
  for (const r of prev.ranges) {
    if (r.endMin > 1440 && minutes < r.endMin - 1440) return true;
  }
  return false;
}

export interface Transition {
  /** Date object representing the next toggle moment. */
  at: Date;
  /** What the status becomes AT that moment. */
  opens: boolean;
}

/**
 * Returns the next open→closed or closed→open transition after `when`.
 * Scans up to 8 days forward; returns null if there is none (e.g. the
 * place is permanently closed in the parsed tree).
 */
export function nextTransition(
  parsed: ParsedHours,
  when: Date,
): Transition | null {
  if (parsed.alwaysOpen) return null;
  const cur = isOpenAt(parsed, when);

  // Probe every minute-boundary in the next 8 days. Since our grammar is
  // minute-granular, this catches every transition without overshooting.
  const maxHoriz = 8 * 24 * 60; // minutes
  const start = new Date(when);
  start.setSeconds(0, 0);
  for (let i = 1; i <= maxHoriz; i++) {
    const probe = new Date(start.getTime() + i * 60_000);
    const open = isOpenAt(parsed, probe);
    if (open !== cur) return { at: probe, opens: open };
  }
  return null;
}

/** Convenience: human-readable "Open now ⋅ closes 22:00" label. */
export function statusLabel(
  parsed: ParsedHours,
  when: Date,
  formatTime: (d: Date) => string,
): string {
  if (parsed.alwaysOpen) return "Open 24 hours";
  const open = isOpenAt(parsed, when);
  const next = nextTransition(parsed, when);
  if (open) {
    if (!next) return "Open";
    return `Open now · closes ${formatTime(next.at)}`;
  }
  if (!next) return "Closed";
  return `Closed · opens ${formatTime(next.at)}`;
}

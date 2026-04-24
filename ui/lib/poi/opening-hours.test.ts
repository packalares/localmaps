import { describe, expect, it } from "vitest";
import {
  isOpenAt,
  nextTransition,
  parseOpeningHours,
  statusLabel,
} from "./opening-hours";

// Helper: make a Date for a fixed weekday + HH:MM in local TZ. We use a
// base Sunday (2024-01-07 = Sunday) and offset by weekday.
function at(weekday: number, hhmm: string): Date {
  const [h, m] = hhmm.split(":").map(Number);
  const d = new Date(2024, 0, 7 + weekday, h, m, 0, 0);
  return d;
}

describe("parseOpeningHours", () => {
  it("returns an empty week for an empty input", () => {
    const p = parseOpeningHours("");
    expect(p.alwaysOpen).toBe(false);
    expect(p.week).toHaveLength(7);
    for (const d of p.week) expect(d.ranges).toEqual([]);
  });

  it("handles 24/7", () => {
    const p = parseOpeningHours("24/7");
    expect(p.alwaysOpen).toBe(true);
    expect(isOpenAt(p, at(0, "03:00"))).toBe(true);
    expect(isOpenAt(p, at(3, "23:59"))).toBe(true);
  });

  it("parses Mo-Fr 09:00-18:00", () => {
    const p = parseOpeningHours("Mo-Fr 09:00-18:00");
    // Monday 10:00 → open
    expect(isOpenAt(p, at(1, "10:00"))).toBe(true);
    // Monday 08:59 → closed
    expect(isOpenAt(p, at(1, "08:59"))).toBe(false);
    // Monday 18:00 → closed (exclusive end)
    expect(isOpenAt(p, at(1, "18:00"))).toBe(false);
    // Saturday 10:00 → closed
    expect(isOpenAt(p, at(6, "10:00"))).toBe(false);
  });

  it("parses Mo-Fr 09:00-18:00; Sa 10:00-14:00; Su off", () => {
    const p = parseOpeningHours("Mo-Fr 09:00-18:00; Sa 10:00-14:00; Su off");
    expect(isOpenAt(p, at(6, "11:00"))).toBe(true); // Sat
    expect(isOpenAt(p, at(6, "14:00"))).toBe(false);
    expect(isOpenAt(p, at(0, "11:00"))).toBe(false); // Sun closed
  });

  it("handles comma-separated day ranges", () => {
    const p = parseOpeningHours("Mo,We,Fr 08:00-12:00");
    expect(isOpenAt(p, at(1, "09:00"))).toBe(true);
    expect(isOpenAt(p, at(2, "09:00"))).toBe(false);
    expect(isOpenAt(p, at(3, "09:00"))).toBe(true);
  });

  it("handles overnight ranges crossing midnight", () => {
    const p = parseOpeningHours("Fr-Sa 22:00-02:00");
    expect(isOpenAt(p, at(5, "23:00"))).toBe(true); // Fri 23:00
    expect(isOpenAt(p, at(6, "01:30"))).toBe(true); // Sat 01:30 (carryover)
    expect(isOpenAt(p, at(6, "03:00"))).toBe(false);
  });

  it("handles multiple time ranges in a single rule", () => {
    const p = parseOpeningHours("Mo-Fr 09:00-12:00,13:00-18:00");
    expect(isOpenAt(p, at(1, "10:00"))).toBe(true);
    expect(isOpenAt(p, at(1, "12:30"))).toBe(false); // lunch
    expect(isOpenAt(p, at(1, "14:00"))).toBe(true);
  });

  it("treats later rules as overriding earlier ones for same day", () => {
    const p = parseOpeningHours("Mo-Su 09:00-17:00; Su off");
    expect(isOpenAt(p, at(0, "12:00"))).toBe(false);
    expect(isOpenAt(p, at(1, "12:00"))).toBe(true);
  });

  it("skips PH / SH tokens gracefully", () => {
    const p = parseOpeningHours("PH off; Mo-Fr 09:00-17:00");
    expect(isOpenAt(p, at(1, "10:00"))).toBe(true);
  });

  it("skips malformed rules without dropping valid ones", () => {
    const p = parseOpeningHours("garbage; Mo-Fr 09:00-17:00");
    expect(isOpenAt(p, at(1, "10:00"))).toBe(true);
  });

  it("supports wrap-around day ranges (Fr-Mo)", () => {
    const p = parseOpeningHours("Fr-Mo 10:00-14:00");
    expect(isOpenAt(p, at(5, "11:00"))).toBe(true); // Fri
    expect(isOpenAt(p, at(6, "11:00"))).toBe(true); // Sat
    expect(isOpenAt(p, at(0, "11:00"))).toBe(true); // Sun
    expect(isOpenAt(p, at(1, "11:00"))).toBe(true); // Mon
    expect(isOpenAt(p, at(2, "11:00"))).toBe(false); // Tue
  });

  it("implicit all-days for bare time rule", () => {
    const p = parseOpeningHours("09:00-17:00");
    expect(isOpenAt(p, at(0, "10:00"))).toBe(true);
    expect(isOpenAt(p, at(3, "10:00"))).toBe(true);
  });
});

describe("nextTransition", () => {
  it("returns null for 24/7", () => {
    const p = parseOpeningHours("24/7");
    expect(nextTransition(p, at(1, "10:00"))).toBeNull();
  });

  it("finds the closing time during an open window", () => {
    const p = parseOpeningHours("Mo-Fr 09:00-18:00");
    const t = nextTransition(p, at(1, "10:00"));
    expect(t).not.toBeNull();
    expect(t!.opens).toBe(false);
    expect(t!.at.getHours()).toBe(18);
    expect(t!.at.getMinutes()).toBe(0);
  });

  it("finds the next opening while closed", () => {
    const p = parseOpeningHours("Mo-Fr 09:00-18:00");
    // Sat 10:00 → next open is Mon 09:00
    const t = nextTransition(p, at(6, "10:00"));
    expect(t).not.toBeNull();
    expect(t!.opens).toBe(true);
    expect(t!.at.getHours()).toBe(9);
  });
});

describe("statusLabel", () => {
  const fmt = (d: Date) =>
    `${String(d.getHours()).padStart(2, "0")}:${String(
      d.getMinutes(),
    ).padStart(2, "0")}`;

  it("says 'Open 24 hours' for 24/7", () => {
    const p = parseOpeningHours("24/7");
    expect(statusLabel(p, at(1, "10:00"), fmt)).toBe("Open 24 hours");
  });

  it("says 'Open now · closes HH:MM' during a window", () => {
    const p = parseOpeningHours("Mo-Fr 09:00-18:00");
    expect(statusLabel(p, at(1, "10:00"), fmt)).toContain("Open now");
    expect(statusLabel(p, at(1, "10:00"), fmt)).toContain("18:00");
  });

  it("says 'Closed · opens HH:MM' while closed", () => {
    const p = parseOpeningHours("Mo-Fr 09:00-18:00");
    expect(statusLabel(p, at(6, "10:00"), fmt)).toContain("Closed");
    expect(statusLabel(p, at(6, "10:00"), fmt)).toContain("09:00");
  });
});

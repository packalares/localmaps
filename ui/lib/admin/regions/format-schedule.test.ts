import { describe, expect, it } from "vitest";
import {
  formatNextUpdate,
  formatSchedule,
  isValidCron,
  parseSchedule,
  serializeSchedule,
} from "./format-schedule";

describe("parseSchedule / serializeSchedule", () => {
  it("round-trips presets", () => {
    for (const k of ["never", "daily", "weekly", "monthly"] as const) {
      expect(parseSchedule(k)).toEqual({ kind: k });
      expect(serializeSchedule({ kind: k })).toBe(k);
    }
  });

  it("treats empty / null as never", () => {
    expect(parseSchedule(null)).toEqual({ kind: "never" });
    expect(parseSchedule(undefined)).toEqual({ kind: "never" });
    expect(parseSchedule("")).toEqual({ kind: "never" });
  });

  it("parses arbitrary cron as custom", () => {
    expect(parseSchedule("0 4 * * 0")).toEqual({
      kind: "custom",
      cron: "0 4 * * 0",
    });
    expect(serializeSchedule({ kind: "custom", cron: "0 4 * * 0" })).toBe(
      "0 4 * * 0",
    );
  });
});

describe("formatSchedule", () => {
  it("labels presets", () => {
    expect(formatSchedule("never")).toBe("Never");
    expect(formatSchedule("monthly")).toBe("Monthly");
  });

  it("labels custom with the raw cron", () => {
    expect(formatSchedule("0 4 * * 0")).toBe("Custom (0 4 * * 0)");
  });
});

describe("isValidCron", () => {
  it("accepts 5-field expressions", () => {
    expect(isValidCron("0 4 * * 0")).toBe(true);
    expect(isValidCron("*/10 * * * *")).toBe(true);
    expect(isValidCron("0 0 1,15 * *")).toBe(true);
  });

  it("rejects the wrong number of fields", () => {
    expect(isValidCron("0 4 * *")).toBe(false);
    expect(isValidCron("0 4 * * 0 *")).toBe(false);
  });

  it("rejects disallowed characters", () => {
    expect(isValidCron("0 4 * * !")).toBe(false);
  });

  it("rejects empty input", () => {
    expect(isValidCron("")).toBe(false);
  });
});

describe("formatNextUpdate", () => {
  it("returns em-dash for missing or invalid input", () => {
    expect(formatNextUpdate(null)).toBe("—");
    expect(formatNextUpdate("")).toBe("—");
    expect(formatNextUpdate("not-a-date")).toBe("—");
  });

  it("renders a readable weekday + time", () => {
    const s = formatNextUpdate("2026-05-10T04:00:00Z");
    expect(s).toMatch(/[A-Za-z]{3}/);
  });
});

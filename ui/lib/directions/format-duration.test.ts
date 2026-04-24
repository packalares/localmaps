import { describe, expect, it } from "vitest";
import { formatDuration } from "./format-duration";

describe("formatDuration", () => {
  it("handles sub-minute values", () => {
    expect(formatDuration(5)).toBe("<1 min");
    expect(formatDuration(59)).toBe("<1 min");
  });

  it("formats minutes", () => {
    expect(formatDuration(60)).toBe("1 min");
    expect(formatDuration(125)).toBe("2 min"); // 2 min 5 s → rounds down to 2
    expect(formatDuration(1799)).toBe("30 min");
  });

  it("formats hours and minutes", () => {
    expect(formatDuration(3600)).toBe("1 h");
    expect(formatDuration(3900)).toBe("1 h 5 min");
    expect(formatDuration(3 * 3600 + 20 * 60)).toBe("3 h 20 min");
  });

  it("formats days for multi-day durations", () => {
    expect(formatDuration(24 * 3600)).toBe("1 d");
    expect(formatDuration(26 * 3600)).toBe("1 d 2 h");
  });

  it("returns placeholder for invalid inputs", () => {
    expect(formatDuration(-10)).toBe("—");
    expect(formatDuration(Number.NaN)).toBe("—");
  });
});

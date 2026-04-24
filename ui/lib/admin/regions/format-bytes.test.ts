import { describe, expect, it } from "vitest";
import { formatBytes } from "./format-bytes";

describe("formatBytes", () => {
  it("renders em-dash for nullish or invalid", () => {
    expect(formatBytes(null)).toBe("—");
    expect(formatBytes(undefined)).toBe("—");
    expect(formatBytes(Number.NaN)).toBe("—");
    expect(formatBytes(-1)).toBe("—");
  });

  it("renders 0 as '0 B'", () => {
    expect(formatBytes(0)).toBe("0 B");
  });

  it("uses whole-number bytes below 1 KB", () => {
    expect(formatBytes(512)).toBe("512 B");
    expect(formatBytes(1023)).toBe("1023 B");
  });

  it("switches to KB / MB / GB as needed", () => {
    expect(formatBytes(1024)).toBe("1.0 KB");
    expect(formatBytes(1500)).toBe("1.5 KB");
    expect(formatBytes(10 * 1024)).toBe("10 KB");
    expect(formatBytes(1024 * 1024)).toBe("1.0 MB");
    expect(formatBytes(5 * 1024 * 1024 * 1024)).toBe("5.0 GB");
    expect(formatBytes(100 * 1024 * 1024 * 1024)).toBe("100 GB");
  });

  it("rolls into TB on huge volumes", () => {
    expect(formatBytes(2 * 1024 ** 4)).toBe("2.0 TB");
  });
});

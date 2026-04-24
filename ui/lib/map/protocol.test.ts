import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  isPmtilesProtocolRegistered,
  registerPmtilesProtocol,
  resetPmtilesProtocolForTests,
} from "./protocol";

describe("registerPmtilesProtocol", () => {
  beforeEach(() => {
    resetPmtilesProtocolForTests();
  });

  it("registers the pmtiles protocol on first call", () => {
    const addProtocol = vi.fn();
    const tile = vi.fn();
    const maplibre = { addProtocol };
    const pmtilesModule = {
      Protocol: class {
        tile = tile;
      },
    };

    const first = registerPmtilesProtocol({ maplibre, pmtilesModule });
    expect(first).toBe(true);
    expect(addProtocol).toHaveBeenCalledTimes(1);
    expect(addProtocol).toHaveBeenCalledWith("pmtiles", tile);
    expect(isPmtilesProtocolRegistered()).toBe(true);
  });

  it("is idempotent on repeated calls", () => {
    const addProtocol = vi.fn();
    const maplibre = { addProtocol };
    const pmtilesModule = {
      Protocol: class {
        tile = vi.fn();
      },
    };

    const first = registerPmtilesProtocol({ maplibre, pmtilesModule });
    const second = registerPmtilesProtocol({ maplibre, pmtilesModule });
    const third = registerPmtilesProtocol({ maplibre, pmtilesModule });

    expect(first).toBe(true);
    expect(second).toBe(false);
    expect(third).toBe(false);
    expect(addProtocol).toHaveBeenCalledTimes(1);
  });
});

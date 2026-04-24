import { beforeEach, describe, expect, it } from "vitest";
import { useMeasureStore } from "./measure-state";

function reset() {
  useMeasureStore.getState().clear();
}

describe("measure-state", () => {
  beforeEach(reset);

  it("starts inactive with an empty points list", () => {
    const s = useMeasureStore.getState();
    expect(s.isActive).toBe(false);
    expect(s.points).toEqual([]);
    expect(s.mode).toBe("distance");
  });

  it("setActive(true) clears points + finalised flag", () => {
    useMeasureStore.setState({
      points: [{ lng: 1, lat: 1 }],
      isFinalised: true,
    });
    useMeasureStore.getState().setActive(true);
    const s = useMeasureStore.getState();
    expect(s.isActive).toBe(true);
    expect(s.points).toEqual([]);
    expect(s.isFinalised).toBe(false);
  });

  it("addPoint only appends while active and not finalised", () => {
    useMeasureStore.getState().addPoint({ lng: 1, lat: 1 });
    expect(useMeasureStore.getState().points).toHaveLength(0);

    useMeasureStore.getState().setActive(true);
    useMeasureStore.getState().addPoint({ lng: 1, lat: 1 });
    useMeasureStore.getState().addPoint({ lng: 2, lat: 2 });
    expect(useMeasureStore.getState().points).toHaveLength(2);

    useMeasureStore.getState().finalise();
    useMeasureStore.getState().addPoint({ lng: 3, lat: 3 });
    expect(useMeasureStore.getState().points).toHaveLength(2);
  });

  it("removeLastPoint pops from the tail", () => {
    useMeasureStore.getState().setActive(true);
    useMeasureStore.getState().addPoint({ lng: 1, lat: 1 });
    useMeasureStore.getState().addPoint({ lng: 2, lat: 2 });
    useMeasureStore.getState().removeLastPoint();
    expect(useMeasureStore.getState().points).toEqual([{ lng: 1, lat: 1 }]);
  });

  it("finalise without any points stays non-finalised (nothing to show)", () => {
    useMeasureStore.getState().setActive(true);
    useMeasureStore.getState().finalise();
    expect(useMeasureStore.getState().isFinalised).toBe(false);
    expect(useMeasureStore.getState().isActive).toBe(false);
  });

  it("clear wipes everything", () => {
    useMeasureStore.setState({
      mode: "area",
      points: [{ lng: 1, lat: 1 }],
      isActive: true,
      isFinalised: true,
    });
    useMeasureStore.getState().clear();
    const s = useMeasureStore.getState();
    expect(s).toMatchObject({
      mode: "distance",
      points: [],
      isActive: false,
      isFinalised: false,
    });
  });

  it("setMode does not alter active/points state", () => {
    useMeasureStore.getState().setActive(true);
    useMeasureStore.getState().addPoint({ lng: 1, lat: 1 });
    useMeasureStore.getState().setMode("area");
    const s = useMeasureStore.getState();
    expect(s.mode).toBe("area");
    expect(s.points).toHaveLength(1);
    expect(s.isActive).toBe(true);
  });
});

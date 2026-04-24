import { beforeEach, describe, expect, it } from "vitest";
import { useIsochroneStore } from "./isochrone-state";

function reset() {
  useIsochroneStore.getState().clear();
}

describe("isochrone-state", () => {
  beforeEach(reset);

  it("defaults to auto mode with all three bands selected", () => {
    const s = useIsochroneStore.getState();
    expect(s.mode).toBe("auto");
    expect(s.minutes).toEqual([10, 15, 30]);
    expect(s.origin).toBeNull();
  });

  it("setActive(true) clears origin + result", () => {
    useIsochroneStore.setState({
      origin: { lng: 1, lat: 2 },
      result: { type: "FeatureCollection", features: [] },
    });
    useIsochroneStore.getState().setActive(true);
    const s = useIsochroneStore.getState();
    expect(s.origin).toBeNull();
    expect(s.result).toBeNull();
    expect(s.isActive).toBe(true);
  });

  it("toggleMinutes adds and removes a band, keeping order ascending", () => {
    useIsochroneStore.setState({ minutes: [15] });
    useIsochroneStore.getState().toggleMinutes(10);
    expect(useIsochroneStore.getState().minutes).toEqual([10, 15]);
    useIsochroneStore.getState().toggleMinutes(15);
    expect(useIsochroneStore.getState().minutes).toEqual([10]);
  });

  it("toggleMinutes refuses to empty the list", () => {
    useIsochroneStore.setState({ minutes: [10] });
    useIsochroneStore.getState().toggleMinutes(10);
    // No-op: we keep at least one band selected.
    expect(useIsochroneStore.getState().minutes).toEqual([10]);
  });

  it("setOrigin + setMode update the respective fields", () => {
    useIsochroneStore.getState().setOrigin({ lng: 26, lat: 44 });
    useIsochroneStore.getState().setMode("bicycle");
    const s = useIsochroneStore.getState();
    expect(s.origin).toEqual({ lng: 26, lat: 44 });
    expect(s.mode).toBe("bicycle");
  });

  it("clear resets everything", () => {
    useIsochroneStore.setState({
      origin: { lng: 1, lat: 2 },
      mode: "pedestrian",
      minutes: [10],
      isActive: true,
      result: { type: "FeatureCollection", features: [] },
      isLoading: true,
    });
    useIsochroneStore.getState().clear();
    const s = useIsochroneStore.getState();
    expect(s).toMatchObject({
      origin: null,
      mode: "auto",
      minutes: [10, 15, 30],
      isActive: false,
      isLoading: false,
      result: null,
    });
  });
});

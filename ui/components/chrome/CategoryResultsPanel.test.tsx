import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useMapStore } from "@/lib/state/map";
import { usePlaceStore } from "@/lib/state/place";
import { useDirectionsStore } from "@/lib/state/directions";
import { CategoryResultsPanel } from "./CategoryResultsPanel";

/**
 * The chip-results side panel renders only while a chip is active. It
 * mirrors the `Directions` panel layout:
 *   header (label + close X) → "Directions from your location" CTA →
 *   list of `ResultCard`s. Selecting a row publishes the POI to
 *   `selectedFeature` (so the bottom info card appears) and centers
 *   the map. Closing clears the chip + the panel.
 */

const samplePoi = {
  id: "p1",
  label: "Caru' cu Bere",
  category: "food",
  center: { lat: 44.43, lon: 26.1 },
};

describe("<CategoryResultsPanel />", () => {
  beforeEach(() => {
    useMapStore.getState().clear();
    usePlaceStore.getState().clearSelectedFeature();
    useDirectionsStore.getState().reset();
  });
  afterEach(() => {
    useMapStore.getState().clear();
    usePlaceStore.getState().clearSelectedFeature();
    useDirectionsStore.getState().reset();
  });

  it("renders nothing when no chip is active", () => {
    const { container } = render(<CategoryResultsPanel />);
    expect(container).toBeEmptyDOMElement();
  });

  it("renders the chip label, the location CTA, and result cards", () => {
    act(() => {
      const s = useMapStore.getState();
      s.setActiveCategoryChip("food");
      s.setCategorySearchResults([samplePoi]);
    });

    render(<CategoryResultsPanel />);
    expect(screen.getByRole("heading", { name: /food & drink/i }))
      .toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /directions from your location/i }),
    ).toBeInTheDocument();
    expect(screen.getByText("Caru' cu Bere")).toBeInTheDocument();
  });

  it("clicking a result publishes the POI as selectedFeature", async () => {
    const user = userEvent.setup();
    act(() => {
      const s = useMapStore.getState();
      s.setActiveCategoryChip("food");
      s.setCategorySearchResults([samplePoi]);
    });

    render(<CategoryResultsPanel />);
    await user.click(screen.getByText("Caru' cu Bere"));

    const f = usePlaceStore.getState().selectedFeature;
    expect(f).not.toBeNull();
    expect(f!.kind).toBe("poi");
    expect(f!.id).toBe("p1");
    expect(f!.lat).toBeCloseTo(44.43);
    expect(f!.lon).toBeCloseTo(26.1);
  });

  it("close X clears the active chip and closes the panel", async () => {
    const user = userEvent.setup();
    act(() => {
      const s = useMapStore.getState();
      s.runCategorySearch("food");
      s.setCategorySearchResults([samplePoi]);
    });
    expect(useMapStore.getState().activeCategoryChip).toBe("food");
    expect(useMapStore.getState().leftRailTab).toBe("categoryResults");

    render(<CategoryResultsPanel />);
    await user.click(screen.getByRole("button", { name: /close results/i }));

    expect(useMapStore.getState().activeCategoryChip).toBeNull();
    expect(useMapStore.getState().leftRailTab).toBe("search");
  });

  it("Directions-from-your-location flips the rail to the directions tab", async () => {
    const user = userEvent.setup();
    // Stub geolocation so the success callback fires synchronously.
    const getCurrentPosition = vi.fn(
      (success: PositionCallback) => {
        success({
          coords: {
            latitude: 44.4,
            longitude: 26.1,
            accuracy: 1,
            altitude: null,
            altitudeAccuracy: null,
            heading: null,
            speed: null,
          },
          timestamp: Date.now(),
        } as GeolocationPosition);
      },
    );
    Object.defineProperty(globalThis.navigator, "geolocation", {
      configurable: true,
      value: { getCurrentPosition },
    });

    act(() => {
      useMapStore.getState().setActiveCategoryChip("food");
    });

    render(<CategoryResultsPanel />);
    await user.click(
      screen.getByRole("button", { name: /directions from your location/i }),
    );

    expect(getCurrentPosition).toHaveBeenCalledTimes(1);
    expect(useMapStore.getState().leftRailTab).toBe("directions");
    const wp = useDirectionsStore.getState().waypoints;
    expect(wp[0].label).toBe("Your location");
    expect(wp[0].lngLat).toEqual({ lng: 26.1, lat: 44.4 });
  });
});

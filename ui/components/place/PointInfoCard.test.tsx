import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import maplibregl, { type Map as MapLibreMap } from "maplibre-gl";
import {
  PointInfoCard,
  PointInfoCardHost,
} from "./PointInfoCard";
import { usePlaceStore, type SelectedFeature } from "@/lib/state/place";
import { useMapStore } from "@/lib/state/map";
import { useDirectionsStore } from "@/lib/state/directions";

/**
 * Build a fresh QueryClient per test so the reverse-geocode / poi
 * hooks never share cached state.
 */
function wrap(ui: React.ReactElement) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={qc}>{ui}</QueryClientProvider>,
  );
}

describe("<PointInfoCard />", () => {
  beforeEach(() => {
    usePlaceStore.getState().clearSelectedFeature();
    useMapStore.getState().clear();
    useDirectionsStore.getState().reset();
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("returns null when no feature is selected", () => {
    const { container } = wrap(
      <PointInfoCard feature={null} onClose={() => {}} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("renders a point-kind card with coords + close button", () => {
    const feature: SelectedFeature = {
      kind: "point",
      lat: 44.47992,
      lon: 26.16213,
      name: "Şoseaua Bucuresti",
    };
    wrap(<PointInfoCard feature={feature} onClose={() => {}} />);
    expect(
      screen.getByRole("heading", { level: 2, name: /şoseaua bucuresti/i }),
    ).toBeInTheDocument();
    expect(screen.getByText(/44\.47992, 26\.16213/)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /close info card/i }),
    ).toBeInTheDocument();
  });

  it("renders a POI-kind card with hours, phone, and website", () => {
    const feature: SelectedFeature = {
      kind: "poi",
      lat: 52.5,
      lon: 13.4,
      id: "p1",
      name: "Cafe Lume",
      hours: "Mo-Fr 09:00-18:00",
      phone: "+49 30 1234567",
      website: "https://example.com",
      categoryIcon: "cafe",
    };
    wrap(<PointInfoCard feature={feature} onClose={() => {}} />);
    expect(screen.getByText(/mo-fr 09:00-18:00/i)).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /\+49 30 1234567/ }),
    ).toHaveAttribute("href", "tel:+49301234567");
    const site = screen.getByRole("link", { name: /example\.com/ });
    expect(site).toHaveAttribute("href", "https://example.com");
    expect(site).toHaveAttribute("target", "_blank");
  });

  it("close button invokes the onClose callback", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    const feature: SelectedFeature = {
      kind: "point",
      lat: 1,
      lon: 2,
    };
    wrap(<PointInfoCard feature={feature} onClose={onClose} />);
    await user.click(screen.getByRole("button", { name: /close info card/i }));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("directions button invokes onDirections with the feature", async () => {
    const user = userEvent.setup();
    const onDirections = vi.fn();
    const feature: SelectedFeature = {
      kind: "point",
      lat: 10,
      lon: 20,
      name: "Somewhere",
    };
    wrap(
      <PointInfoCard
        feature={feature}
        onClose={() => {}}
        onDirections={onDirections}
      />,
    );
    await user.click(
      screen.getByRole("button", { name: /directions to somewhere/i }),
    );
    expect(onDirections).toHaveBeenCalledTimes(1);
    expect(onDirections.mock.calls[0][0]).toMatchObject({
      kind: "point",
      lat: 10,
      lon: 20,
    });
  });

  it("share button invokes onShare when provided", async () => {
    const user = userEvent.setup();
    const onShare = vi.fn();
    const feature: SelectedFeature = {
      kind: "point",
      lat: 10,
      lon: 20,
    };
    wrap(
      <PointInfoCard
        feature={feature}
        onClose={() => {}}
        onShare={onShare}
      />,
    );
    await user.click(
      screen.getByRole("button", { name: /share this location/i }),
    );
    expect(onShare).toHaveBeenCalledTimes(1);
  });

  it(
    "share button replaces the URL with a deep-link and opens the share " +
      "dialog when no onShare handler is provided",
    async () => {
      const user = userEvent.setup();
      const replaceState = vi.spyOn(window.history, "replaceState");

      const feature: SelectedFeature = {
        kind: "point",
        lat: 44.47992,
        lon: 26.16213,
      };
      wrap(<PointInfoCard feature={feature} onClose={() => {}} />);

      await user.click(
        screen.getByRole("button", { name: /share this location/i }),
      );

      // The card flips the address bar to the deep-link before opening
      // the rich Share dialog (Link / QR / Embed). Verify the path
      // pushed to history, not a clipboard fallback.
      expect(replaceState).toHaveBeenCalled();
      const path = replaceState.mock.calls[replaceState.mock.calls.length - 1][2] as string;
      expect(path).toMatch(/lat=44\.47992/);
      expect(path).toMatch(/lon=26\.16213/);
      expect(path).toMatch(/zoom=15/);

      replaceState.mockRestore();
    },
  );

  it("share button appends place=<id> for POI features", async () => {
    const user = userEvent.setup();
    const replaceState = vi.spyOn(window.history, "replaceState");

    const feature: SelectedFeature = {
      kind: "poi",
      lat: 52.5,
      lon: 13.4,
      id: "node/123",
      name: "Cafe Lume",
    };
    wrap(<PointInfoCard feature={feature} onClose={() => {}} />);

    await user.click(
      screen.getByRole("button", { name: /share this location/i }),
    );

    expect(replaceState).toHaveBeenCalled();
    const path = replaceState.mock.calls[replaceState.mock.calls.length - 1][2] as string;
    expect(path).toMatch(/place=node%2F123/);

    replaceState.mockRestore();
  });
});

describe("<PointInfoCardHost />", () => {
  beforeEach(() => {
    usePlaceStore.getState().clearSelectedFeature();
    useMapStore.getState().clear();
    useDirectionsStore.getState().reset();
  });

  it("renders nothing when the store has no selectedFeature", () => {
    const { container } = wrap(<PointInfoCardHost />);
    expect(container).toBeEmptyDOMElement();
  });

  it("renders when a feature is set and the close button clears it", async () => {
    const user = userEvent.setup();
    usePlaceStore.getState().setSelectedFeature({
      kind: "point",
      lat: 44.5,
      lon: 26.1,
      name: "Pinned",
    });
    wrap(<PointInfoCardHost />);
    expect(
      screen.getByRole("heading", { level: 2, name: /pinned/i }),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /close info card/i }));
    expect(usePlaceStore.getState().selectedFeature).toBeNull();
  });

  it("directions button writes the destination into the directions store and opens the panel", async () => {
    const user = userEvent.setup();
    usePlaceStore.getState().setSelectedFeature({
      kind: "point",
      lat: 12,
      lon: 34,
      name: "Dropped pin",
    });
    wrap(<PointInfoCardHost />);

    await user.click(
      screen.getByRole("button", { name: /directions to dropped pin/i }),
    );

    const waypoints = useDirectionsStore.getState().waypoints;
    const last = waypoints[waypoints.length - 1];
    expect(last.lngLat).toEqual({ lng: 34, lat: 12 });
    expect(last.label).toBe("Dropped pin");
    expect(useMapStore.getState().leftRailTab).toBe("directions");
  });

  it("drops the custom dropped-pin marker on open and removes it when selection clears", () => {
    const addTo = vi.fn().mockReturnThis();
    const setLngLat = vi.fn().mockReturnThis();
    const remove = vi.fn();
    const markerSpy = vi.spyOn(maplibregl, "Marker").mockImplementation(
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (..._args: unknown[]) => {
        return { setLngLat, addTo, remove } as unknown as maplibregl.Marker;
      },
    );
    setLngLat.mockReturnValue({ setLngLat, addTo, remove });
    addTo.mockReturnValue({ setLngLat, addTo, remove });

    // Pretend we have a map. The marker code only checks for truthiness.
    useMapStore.getState().setMap({} as unknown as MapLibreMap);

    wrap(<PointInfoCardHost />);
    // No feature yet — nothing should have been constructed.
    expect(markerSpy).not.toHaveBeenCalled();

    act(() => {
      usePlaceStore.getState().setSelectedFeature({
        kind: "point",
        lat: 10,
        lon: 20,
      });
    });

    expect(markerSpy).toHaveBeenCalledTimes(1);
    // The marker now uses a custom HTMLElement (the small grey badge
    // with the white pin glyph) anchored at its bottom.
    const ctorArg = markerSpy.mock.calls[0][0] as {
      element?: HTMLElement;
      anchor?: string;
    };
    expect(ctorArg.anchor).toBe("bottom");
    expect(ctorArg.element).toBeInstanceOf(HTMLElement);
    expect(ctorArg.element?.classList.contains("localmaps-dropped-pin")).toBe(
      true,
    );
    expect(setLngLat).toHaveBeenCalledWith([20, 10]);
    expect(addTo).toHaveBeenCalledTimes(1);
    expect(remove).not.toHaveBeenCalled();

    // Clearing the selected feature should remove the marker.
    act(() => {
      usePlaceStore.getState().clearSelectedFeature();
    });
    expect(remove).toHaveBeenCalledTimes(1);

    markerSpy.mockRestore();
  });

  it("does not attempt to mount a marker when the map is not ready", () => {
    const markerSpy = vi.spyOn(maplibregl, "Marker");
    // No map on the store.
    usePlaceStore.getState().setSelectedFeature({
      kind: "point",
      lat: 1,
      lon: 2,
    });
    wrap(<PointInfoCardHost />);
    expect(markerSpy).not.toHaveBeenCalled();
    markerSpy.mockRestore();
  });
});

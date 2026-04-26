import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useDirectionsStore } from "@/lib/state/directions";
import { useMapStore } from "@/lib/state/map";

// Stub out map-rendering and route-sync hooks: they touch MapLibre and
// the real fetch layer respectively.
vi.mock("@/lib/directions/use-route-render", () => ({
  useRouteRender: () => {},
}));
vi.mock("@/lib/directions/use-route-sync", () => ({
  useRouteSync: () => ({ isError: false, isPending: false }),
}));

// Mock the autocomplete hook with a tiny deterministic dataset.
vi.mock("@/lib/api/hooks", async () => {
  const actual = await vi.importActual<
    typeof import("@/lib/api/hooks")
  >("@/lib/api/hooks");
  return {
    ...actual,
    useGeocodeAutocomplete: (args: { q: string; enabled?: boolean }) => ({
      data:
        args.enabled !== false && args.q.length >= 3
          ? {
              traceId: "t",
              results: [
                {
                  id: "r1",
                  label: "Bucharest, Romania",
                  center: { lat: 44.43, lon: 26.1 },
                  confidence: 0.9,
                },
                {
                  id: "r2",
                  label: "Budapest, Hungary",
                  center: { lat: 47.5, lon: 19.04 },
                  confidence: 0.8,
                },
              ],
            }
          : undefined,
      isFetching: false,
      isError: false,
    }),
  };
});

import { DirectionsPanel } from "./DirectionsPanel";

function renderWithQuery(ui: React.ReactElement) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>{ui}</QueryClientProvider>,
  );
}

describe("<DirectionsPanel />", () => {
  beforeEach(() => {
    useDirectionsStore.getState().reset();
    try {
      window.localStorage.removeItem("localmaps.search.history.v1");
    } catch {
      /* ignore */
    }
  });
  afterEach(() => {
    useDirectionsStore.getState().reset();
  });

  it("renders mode toggles, From and To inputs, and a close button", () => {
    renderWithQuery(<DirectionsPanel />);
    expect(
      screen.getByRole("tab", { name: /driving/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("tab", { name: /cycling/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("tab", { name: /walking/i }),
    ).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: /^from$/i })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: /^to$/i })).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /close directions/i }),
    ).toBeInTheDocument();
  });

  it("marks the active mode with aria-selected=true", () => {
    useDirectionsStore.setState((s) => ({ ...s, mode: "bicycle" }));
    renderWithQuery(<DirectionsPanel />);
    expect(
      screen.getByRole("tab", { name: /cycling/i }),
    ).toHaveAttribute("aria-selected", "true");
    expect(
      screen.getByRole("tab", { name: /driving/i }),
    ).toHaveAttribute("aria-selected", "false");
  });

  it("clicking a mode toggle updates the Zustand mode", async () => {
    const user = userEvent.setup();
    renderWithQuery(<DirectionsPanel />);
    await user.click(screen.getByRole("tab", { name: /walking/i }));
    expect(useDirectionsStore.getState().mode).toBe("pedestrian");
  });

  it("unsupported modes (transit/flights) are disabled", () => {
    renderWithQuery(<DirectionsPanel />);
    const transit = screen.getByRole("tab", { name: /transit/i });
    const flights = screen.getByRole("tab", { name: /flights/i });
    expect(transit).toBeDisabled();
    expect(flights).toBeDisabled();
  });

  it("autocomplete fills the destination waypoint", async () => {
    const user = userEvent.setup();
    renderWithQuery(<DirectionsPanel />);

    const inputTo = screen.getByRole("textbox", { name: /^to$/i });
    await user.type(inputTo, "buc");

    const option = await screen.findByRole("option", {
      name: /bucharest/i,
    });
    await user.click(option);

    expect((inputTo as HTMLInputElement).value).toMatch(/bucharest/i);
    const stored = useDirectionsStore.getState().waypoints.at(-1);
    expect(stored?.lngLat).toEqual({ lng: 26.1, lat: 44.43 });
  });

  it("swap button flips From and To", async () => {
    const user = userEvent.setup();
    useDirectionsStore.setState((s) => ({
      ...s,
      waypoints: [
        { id: "x", label: "Alpha", lngLat: { lng: 1, lat: 1 } },
        { id: "y", label: "Omega", lngLat: { lng: 2, lat: 2 } },
      ],
    }));
    renderWithQuery(<DirectionsPanel />);
    await user.click(
      screen.getByRole("button", { name: /swap start and end/i }),
    );
    const labels = useDirectionsStore
      .getState()
      .waypoints.map((w) => w.label);
    expect(labels).toEqual(["Omega", "Alpha"]);
  });

  it("close button returns to the search tab", async () => {
    const user = userEvent.setup();
    useMapStore.setState((s) => ({ ...s, leftRailTab: "directions" }));
    renderWithQuery(<DirectionsPanel />);
    await user.click(screen.getByRole("button", { name: /close directions/i }));
    expect(useMapStore.getState().leftRailTab).toBe("search");
  });

  it("'Your location' button uses the geolocation API to fill From", async () => {
    const user = userEvent.setup();
    const getCurrentPosition = vi.fn(
      (success: (pos: GeolocationPosition) => void) => {
        success({
          coords: {
            latitude: 12.34,
            longitude: 56.78,
            accuracy: 10,
            altitude: null,
            altitudeAccuracy: null,
            heading: null,
            speed: null,
          },
          timestamp: Date.now(),
        } as GeolocationPosition);
      },
    );
    Object.defineProperty(window.navigator, "geolocation", {
      configurable: true,
      value: { getCurrentPosition },
    });
    renderWithQuery(<DirectionsPanel />);
    await user.click(screen.getByRole("button", { name: /your location/i }));
    const from = useDirectionsStore.getState().waypoints[0];
    expect(from.lngLat).toEqual({ lng: 56.78, lat: 12.34 });
  });

  it("renders recent places from localStorage and fills the destination on click", async () => {
    const user = userEvent.setup();
    const entry = {
      id: "r-history-1",
      label: "Prajitureselle",
      center: { lat: 44.5, lon: 26.1 },
      confidence: 0.9,
      address: { street: "Strada X 35", city: "Voluntari" },
    };
    window.localStorage.setItem(
      "localmaps.search.history.v1",
      JSON.stringify([entry]),
    );

    renderWithQuery(<DirectionsPanel />);
    const row = await screen.findByRole("button", {
      name: /prajitureselle/i,
    });
    await user.click(row);

    const stored = useDirectionsStore.getState().waypoints.at(-1);
    expect(stored?.label).toMatch(/prajitureselle/i);
    expect(stored?.lngLat).toEqual({ lng: 26.1, lat: 44.5 });
  });
});

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useDirectionsStore } from "@/lib/state/directions";

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
  });
  afterEach(() => {
    useDirectionsStore.getState().reset();
  });

  it("renders two waypoint rows by default", () => {
    renderWithQuery(<DirectionsPanel />);
    const list = screen.getByRole("list", { name: /route waypoints/i });
    const items = within(list).getAllByRole("listitem");
    expect(items).toHaveLength(2);
    expect(screen.getByRole("textbox", { name: /waypoint a/i })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: /waypoint b/i })).toBeInTheDocument();
  });

  it("shows autocomplete results and selecting fills the waypoint label", async () => {
    const user = userEvent.setup();
    renderWithQuery(<DirectionsPanel />);

    const inputA = screen.getByRole("textbox", { name: /waypoint a/i });
    await user.type(inputA, "buc");

    const option = await screen.findByRole("option", {
      name: /bucharest/i,
    });
    await user.click(option);

    expect(
      (inputA as HTMLInputElement).value,
    ).toMatch(/bucharest/i);
    const stored = useDirectionsStore.getState().waypoints[0];
    expect(stored.lngLat).toEqual({ lng: 26.1, lat: 44.43 });
  });

  it("adds a waypoint row when Add stop is clicked", async () => {
    const user = userEvent.setup();
    renderWithQuery(<DirectionsPanel />);
    await user.click(screen.getByRole("button", { name: /add stop/i }));
    const items = within(
      screen.getByRole("list", { name: /route waypoints/i }),
    ).getAllByRole("listitem");
    expect(items.length).toBe(3);
  });

  it("keyboard reorder moves a waypoint down", async () => {
    const user = userEvent.setup();
    useDirectionsStore.setState((s) => ({
      ...s,
      waypoints: [
        { id: "x", label: "Alpha", lngLat: { lng: 1, lat: 1 } },
        { id: "y", label: "Mid", lngLat: { lng: 2, lat: 2 } },
        { id: "z", label: "Omega", lngLat: { lng: 3, lat: 3 } },
      ],
    }));
    renderWithQuery(<DirectionsPanel />);
    const moveDownButtons = screen.getAllByRole("button", {
      name: /move waypoint down/i,
    });
    await user.click(moveDownButtons[0]);
    const labels = useDirectionsStore
      .getState()
      .waypoints.map((w) => w.label);
    expect(labels).toEqual(["Mid", "Alpha", "Omega"]);
  });

  it("swap button flips start and end", async () => {
    const user = userEvent.setup();
    useDirectionsStore.setState((s) => ({
      ...s,
      waypoints: [
        { id: "x", label: "Alpha", lngLat: { lng: 1, lat: 1 } },
        { id: "y", label: "Omega", lngLat: { lng: 2, lat: 2 } },
      ],
    }));
    renderWithQuery(<DirectionsPanel />);
    await user.click(screen.getByRole("button", { name: /swap start and end/i }));
    const labels = useDirectionsStore
      .getState()
      .waypoints.map((w) => w.label);
    expect(labels).toEqual(["Omega", "Alpha"]);
  });
});

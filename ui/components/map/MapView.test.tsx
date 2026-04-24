import { describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MapView } from "./MapView";
import { useMapStore } from "@/lib/state/map";

// MapLibre is heavy and DOM-affecting; mock it at the module boundary so
// the test runs under jsdom. The MapCanvas component imports it eagerly.
vi.mock("maplibre-gl", () => {
  const instance = {
    addControl: vi.fn(),
    on: vi.fn(),
    once: vi.fn(),
    off: vi.fn(),
    remove: vi.fn(),
    getCenter: () => ({ lat: 0, lng: 0 }),
    getZoom: () => 2,
    getBearing: () => 0,
    getPitch: () => 0,
    getStyle: () => null,
    setStyle: vi.fn(),
  };
  const MapCtor = vi.fn(() => instance);
  return {
    default: {
      Map: MapCtor,
      NavigationControl: class {},
      GeolocateControl: class {},
      ScaleControl: class {},
      addProtocol: vi.fn(),
    },
    Map: MapCtor,
    NavigationControl: class {},
    GeolocateControl: class {},
    ScaleControl: class {},
    addProtocol: vi.fn(),
  };
});
vi.mock("pmtiles", () => ({
  Protocol: class {
    tile = vi.fn();
  },
}));

function withQueryClient(children: React.ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  );
}

describe("<MapView />", () => {
  it("renders a loading placeholder before the client bundle resolves", () => {
    vi.stubGlobal("fetch", vi.fn(() => new Promise(() => {})));
    render(withQueryClient(<MapView />));
    // The dynamic loader renders a "Loading map" fallback synchronously.
    expect(
      screen.getByRole("region", { name: /map loading/i }),
    ).toBeInTheDocument();
    vi.unstubAllGlobals();
  });

  it("eventually swaps in the live MapLibre canvas", async () => {
    vi.stubGlobal("fetch", vi.fn(() => new Promise(() => {})));
    useMapStore.getState().clear();
    render(withQueryClient(<MapView />));
    await waitFor(() => {
      expect(
        screen.getByRole("region", { name: /interactive map/i }),
      ).toBeInTheDocument();
    });
    vi.unstubAllGlobals();
  });
});

import { describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";

// MapLibre is heavy + DOM-affecting; mock at the module boundary with the
// subset the EmbedMap tree touches. Keep handler captures so we can poke
// lifecycle hooks explicitly.
type Handler = (...args: unknown[]) => void;

vi.mock("maplibre-gl", () => {
  const handlers = new Map<string, Handler>();
  const instance = {
    addControl: vi.fn(),
    on: vi.fn((type: string, h: Handler) => {
      handlers.set(type, h);
    }),
    once: vi.fn(),
    off: vi.fn((type: string) => {
      handlers.delete(type);
    }),
    remove: vi.fn(),
    setStyle: vi.fn(),
    isStyleLoaded: () => false,
    addSource: vi.fn(),
    addLayer: vi.fn(),
    removeSource: vi.fn(),
    removeLayer: vi.fn(),
    getLayer: () => undefined,
    getSource: () => undefined,
    zoomIn: vi.fn(),
    zoomOut: vi.fn(),
  };
  const MapCtor = vi.fn(() => instance);
  const PopupCtor = vi.fn(() => ({
    setLngLat: vi.fn().mockReturnThis(),
    setText: vi.fn().mockReturnThis(),
    addTo: vi.fn().mockReturnThis(),
    remove: vi.fn(),
  }));
  return {
    default: {
      Map: MapCtor,
      NavigationControl: class {},
      Popup: PopupCtor,
      addProtocol: vi.fn(),
    },
    Map: MapCtor,
    NavigationControl: class {},
    Popup: PopupCtor,
    addProtocol: vi.fn(),
    __handlers: handlers,
    __instance: instance,
  };
});

vi.mock("pmtiles", () => ({
  Protocol: class {
    tile = vi.fn();
  },
}));

import { EmbedMap } from "./EmbedMap";

describe("<EmbedMap />", () => {
  it("renders the loading placeholder synchronously", () => {
    render(
      <EmbedMap
        center={{ lat: 44.43, lon: 26.1 }}
        zoom={12}
        styleName="dark"
        region={null}
        pin={null}
      />,
    );
    expect(
      screen.getByRole("region", { name: /map loading/i }),
    ).toBeInTheDocument();
  });

  it("eventually swaps in the MapLibre canvas region", async () => {
    render(
      <EmbedMap
        center={{ lat: 10, lon: 20 }}
        zoom={5}
        styleName="light"
        region="europe-romania"
        pin={null}
      />,
    );
    await waitFor(() => {
      expect(
        screen.getByRole("region", { name: /interactive map/i }),
      ).toBeInTheDocument();
    });
  });

  it("does not issue any fetch while mounting (no auth/data calls)", async () => {
    const fetchSpy = vi.fn(() => new Promise(() => {}));
    vi.stubGlobal("fetch", fetchSpy);
    render(
      <EmbedMap
        center={{ lat: 0, lon: 0 }}
        zoom={2}
        styleName="light"
        region={null}
        pin={null}
      />,
    );
    await waitFor(() => {
      expect(
        screen.getByRole("region", { name: /interactive map/i }),
      ).toBeInTheDocument();
    });
    // The embed surface must not call `/api/regions`, `/api/settings`, etc.
    // MapLibre would fetch the style URL itself, but we mocked it out so
    // the spy should still be empty.
    expect(fetchSpy).not.toHaveBeenCalled();
    vi.unstubAllGlobals();
  });

  it("renders zoom controls that the user can click", async () => {
    render(
      <EmbedMap
        center={{ lat: 0, lon: 0 }}
        zoom={2}
        styleName="light"
        region={null}
        pin={null}
      />,
    );
    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /zoom in/i }),
      ).toBeInTheDocument();
    });
    const maplibre = (await import(
      "maplibre-gl"
    )) as unknown as {
      __instance: { zoomIn: ReturnType<typeof vi.fn> };
    };
    screen.getByRole("button", { name: /zoom in/i }).click();
    expect(maplibre.__instance.zoomIn).toHaveBeenCalled();
  });
});

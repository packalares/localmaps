import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useActiveToolStore } from "@/lib/tools/active-tool";
import { useIsochroneStore } from "@/lib/tools/isochrone-state";
import { useMapStore } from "@/lib/state/map";
import { IsochronePanel } from "./IsochronePanel";

function withQuery(ui: React.ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={qc}>{ui}</QueryClientProvider>;
}

describe("<IsochronePanel />", () => {
  beforeEach(() => {
    useActiveToolStore.setState({ active: null });
    useIsochroneStore.getState().clear();
    useMapStore.getState().clear();
  });
  afterEach(() => {
    vi.restoreAllMocks();
    useActiveToolStore.setState({ active: null });
    useIsochroneStore.getState().clear();
    useMapStore.getState().clear();
  });

  it("renders nothing when the tool is not active", () => {
    render(withQuery(<IsochronePanel />));
    expect(screen.queryByLabelText(/isochrone tool/i)).not.toBeInTheDocument();
  });

  it("Render button POSTs /api/isochrone with the contract body", async () => {
    const fetchMock = vi.fn(
      async (_url: RequestInfo | URL, _init?: RequestInit) =>
        new Response(
          JSON.stringify({ type: "FeatureCollection", features: [] }),
          { status: 200, headers: { "content-type": "application/json" } },
        ),
    );
    vi.stubGlobal("fetch", fetchMock);

    // Make active with a chosen origin; no map needed because resolveOrigin
    // short-circuits when origin is already set.
    act(() => {
      useActiveToolStore.setState({ active: "isochrone" });
      useIsochroneStore.setState({
        origin: { lng: 26.1, lat: 44.43 },
        mode: "bicycle",
        minutes: [10, 30],
        result: null,
        isLoading: false,
        isActive: true,
      });
    });

    const user = userEvent.setup();
    render(withQuery(<IsochronePanel />));
    await user.click(screen.getByRole("button", { name: /^render$/i }));

    // Wait until the mutation fires by polling until fetch was called.
    await vi.waitFor(() => {
      expect(fetchMock).toHaveBeenCalled();
    });

    const [url, init] = fetchMock.mock.calls[0]!;
    expect(String(url)).toMatch(/\/api\/isochrone$/);
    expect(init?.method).toBe("POST");
    const body = JSON.parse(init!.body as string);
    expect(body).toEqual({
      origin: { lat: 44.43, lon: 26.1 },
      mode: "bicycle",
      contoursSeconds: [600, 1800],
    });
  });

  it("Clear removes the current result", async () => {
    act(() => {
      useActiveToolStore.setState({ active: "isochrone" });
      useIsochroneStore.setState({
        origin: { lng: 1, lat: 2 },
        mode: "auto",
        minutes: [10],
        isActive: true,
        isLoading: false,
        result: { type: "FeatureCollection", features: [] },
      });
    });
    const user = userEvent.setup();
    render(withQuery(<IsochronePanel />));
    await user.click(screen.getByRole("button", { name: /clear/i }));
    expect(useIsochroneStore.getState().result).toBeNull();
  });
});

import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
  type Mock,
} from "vitest";
import {
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { SearchPanel } from "./SearchPanel";
import { useMapStore } from "@/lib/state/map";
import type { GeocodeResult } from "@/lib/api/schemas";

function okJson(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

function bucharest(): GeocodeResult {
  return {
    id: "r-bucharest",
    label: "Bucharest, Romania",
    confidence: 0.95,
    category: "locality",
    center: { lat: 44.4325, lon: 26.1039 },
  };
}

function brasov(): GeocodeResult {
  return {
    id: "r-brasov",
    label: "Brasov, Romania",
    confidence: 0.9,
    category: "locality",
    center: { lat: 45.6427, lon: 25.5887 },
  };
}

function renderPanel(props: Parameters<typeof SearchPanel>[0]) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <SearchPanel {...props} />
    </QueryClientProvider>,
  );
}

describe("<SearchPanel />", () => {
  const originalFetch = globalThis.fetch;

  beforeEach(() => {
    useMapStore.getState().clear();
    useMapStore.setState((s) => ({ ...s, installedRegions: [] }));
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
    vi.restoreAllMocks();
  });

  it("shows the 'install a region' empty state when no query + no regions", () => {
    renderPanel({ query: "" });
    expect(
      screen.getByText(/install a region from admin → regions/i),
    ).toBeInTheDocument();
  });

  it("shows the 'try searching' empty state when no query + regions exist", () => {
    useMapStore.setState((s) => ({
      ...s,
      installedRegions: [
        {
          name: "europe/romania",
          displayName: "Romania",
          sourceUrl: "https://example.test",
          state: "ready" as const,
        },
      ],
    }));
    renderPanel({ query: "" });
    expect(
      screen.getByText(/try searching for a city, a landmark, or an address/i),
    ).toBeInTheDocument();
  });

  it("typing triggers a fetch and renders results", async () => {
    const fetchMock = vi.fn<typeof fetch>(async () =>
      okJson({ results: [bucharest(), brasov()], traceId: "t1" }),
    );
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    renderPanel({ query: "bu", debounceMs: 0 });

    await waitFor(() =>
      expect(screen.getByText("Bucharest")).toBeInTheDocument(),
    );
    expect(screen.getByText("Brasov")).toBeInTheDocument();

    const calledUrl = (fetchMock.mock.calls[0]?.[0] ?? "") as string;
    expect(calledUrl).toContain("/api/geocode/autocomplete");
    expect(calledUrl).toContain("q=bu");
  });

  it("shows the 'no matches' copy when the server returns zero results", async () => {
    globalThis.fetch = (vi.fn<typeof fetch>(async () =>
      okJson({ results: [], traceId: "t2" }),
    ) as unknown) as typeof fetch;

    renderPanel({ query: "zzzz", debounceMs: 0 });

    await waitFor(() =>
      expect(
        screen.getByText(/no matches\. check your spelling/i),
      ).toBeInTheDocument(),
    );
  });

  it("selecting a result fires setSelectedResult and flyTo on the store map", async () => {
    const user = userEvent.setup();
    globalThis.fetch = (vi.fn<typeof fetch>(async () =>
      okJson({ results: [bucharest()], traceId: "t3" }),
    ) as unknown) as typeof fetch;

    const flyTo = vi.fn();
    useMapStore.setState((s) => ({
      ...s,
      map: {
        flyTo,
        getSource: () => undefined,
        getLayer: () => undefined,
        addSource: () => {},
        addLayer: () => {},
        removeLayer: () => {},
        removeSource: () => {},
      } as unknown as typeof s.map,
    }));

    renderPanel({ query: "buc", debounceMs: 0 });

    const option = await screen.findByRole("option", {
      name: /bucharest/i,
    });
    await user.click(option);

    expect(useMapStore.getState().selectedResult?.id).toBe("r-bucharest");
    expect(flyTo).toHaveBeenCalledTimes(1);
    const flyArgs = (flyTo as Mock).mock.calls[0]?.[0];
    expect(flyArgs).toMatchObject({
      center: [26.1039, 44.4325],
    });
  });

  it("keyboard Enter on a highlighted item selects it", async () => {
    globalThis.fetch = (vi.fn<typeof fetch>(async () =>
      okJson({ results: [bucharest(), brasov()], traceId: "t4" }),
    ) as unknown) as typeof fetch;

    renderPanel({ query: "b", debounceMs: 0 });
    await screen.findByText("Bucharest");

    // The panel listens for keydown events bubbling up from its section.
    const section = screen.getByRole("region", { name: /search results/i });
    // Arrow down moves highlight to the second option; Enter selects it.
    fireEvent.keyDown(section, { key: "ArrowDown" });
    fireEvent.keyDown(section, { key: "Enter" });

    expect(useMapStore.getState().selectedResult?.id).toBe("r-brasov");
  });

  it("hits /api/geocode/search when fullSearch is true", async () => {
    const fetchMock = vi.fn<typeof fetch>(async () =>
      okJson({ results: [bucharest()], traceId: "t5" }),
    );
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    renderPanel({ query: "bucharest", debounceMs: 0, fullSearch: true });
    await screen.findByText("Bucharest");
    const url = (fetchMock.mock.calls[0]?.[0] ?? "") as string;
    expect(url).toContain("/api/geocode/search");
  });
});

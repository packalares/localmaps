import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CopyLink } from "./CopyLink";
import { Toaster } from "@/components/ui/toaster";
import { useMapStore } from "@/lib/state/map";
import {
  useDirectionsStore,
  DEFAULT_OPTIONS,
  DEFAULT_WAYPOINTS,
} from "@/lib/state/directions";

function resetStores() {
  useMapStore.getState().clear();
  useDirectionsStore.setState({
    waypoints: DEFAULT_WAYPOINTS(),
    mode: "auto",
    options: DEFAULT_OPTIONS,
    route: null,
    alternatives: [],
  });
}

function setUrl(path: string) {
  window.history.replaceState(null, "", path);
}

/**
 * Minimal clipboard stub — tracks the last write.
 *
 * Must be installed AFTER `userEvent.setup()` because user-event attaches
 * its own clipboard stub in setup; we want to assert against the real
 * write the component performs, not user-event's internal data-transfer
 * clipboard.
 */
function installClipboardStub(): { last: () => string | null } {
  let last: string | null = null;
  Object.defineProperty(navigator, "clipboard", {
    configurable: true,
    value: {
      writeText: vi.fn(async (v: string) => {
        last = v;
      }),
    },
  });
  return { last: () => last };
}

function renderWithToaster() {
  return render(
    <Toaster>
      <CopyLink baseUrl={{ origin: "https://maps.example", pathname: "/" }} />
    </Toaster>,
  );
}

describe("<CopyLink />", () => {
  beforeEach(() => {
    resetStores();
    setUrl("/");
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders an accessible button with tooltip + label", () => {
    renderWithToaster();
    const btn = screen.getByRole("button", { name: /copy link/i });
    expect(btn).toBeInTheDocument();
    expect(btn).toHaveAttribute("aria-label", "Copy link");
  });

  it("writes the current encoded URL to the clipboard", async () => {
    // Populate store so the URL carries non-trivial state.
    useMapStore.setState({
      viewport: { lat: 44.43, lon: 26.1, zoom: 12, bearing: 0, pitch: 0 },
      activeRegion: "europe-romania",
      leftRailTab: "directions",
    });

    const user = userEvent.setup();
    const clip = installClipboardStub();
    renderWithToaster();
    await user.click(screen.getByRole("button", { name: /copy link/i }));

    const written = clip.last();
    expect(written).not.toBeNull();
    expect(written).toContain("https://maps.example/");
    expect(written).toContain("r=europe-romania");
    expect(written).toContain("tab=directions");
    expect(written).toContain("#12.00/44.4300/26.1000");
  });

  it("re-encodes on every click (snapshot-at-click)", async () => {
    const user = userEvent.setup();
    const clip = installClipboardStub();
    renderWithToaster();

    useMapStore.setState({ activeRegion: "europe-romania" });
    await user.click(screen.getByRole("button", { name: /copy link/i }));
    expect(clip.last()).toContain("r=europe-romania");

    useMapStore.setState({ activeRegion: "af-rwanda" });
    await user.click(screen.getByRole("button", { name: /copy link/i }));
    expect(clip.last()).toContain("r=af-rwanda");
  });

  it("falls back to prompt() when clipboard missing", async () => {
    const user = userEvent.setup();
    // Strip clipboard AFTER user-event attaches its stub.
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: undefined,
    });
    const promptSpy = vi
      .spyOn(window, "prompt")
      .mockImplementation(() => null);

    renderWithToaster();
    await user.click(screen.getByRole("button", { name: /copy link/i }));
    expect(promptSpy).toHaveBeenCalledTimes(1);
    expect(promptSpy.mock.calls[0]?.[1]).toContain("https://maps.example/");
  });

  it("shows the over-budget warning instead of copying when state is huge", async () => {
    // Install a route with way too many waypoints.
    useDirectionsStore.setState({
      waypoints: Array.from({ length: 200 }, (_, i) => ({
        id: `w${i}`,
        label: "",
        lngLat: { lng: -179 + i * 0.1234, lat: -89 + i * 0.1234 },
      })),
      mode: "auto",
      options: DEFAULT_OPTIONS,
      route: null,
      alternatives: [],
    });

    const user = userEvent.setup();
    installClipboardStub();
    renderWithToaster();
    await user.click(screen.getByRole("button", { name: /copy link/i }));

    expect(
      await screen.findByText(/link is too long/i),
    ).toBeInTheDocument();
  });

  it("Alt+Shift+C shortcut triggers a copy", async () => {
    const user = userEvent.setup();
    const clip = installClipboardStub();
    renderWithToaster();
    await user.keyboard("{Alt>}{Shift>}C{/Shift}{/Alt}");
    expect(clip.last()).not.toBeNull();
  });
});

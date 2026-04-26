import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { POI_CATEGORIES, useMapStore } from "@/lib/state/map";
import { PoiSearchChips } from "./PoiSearchChips";

/**
 * The chip row is now always visible — the previous zoom + region gates
 * have been lifted so the user always has a clear path to "Search this
 * area for X". The disabled-state interaction (no region installed) is
 * still asserted here.
 */

function wrap(children: React.ReactNode) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

function seedRegion() {
  const s = useMapStore.getState();
  s.setActiveRegion("europe-romania");
  s.setViewport({ lat: 0, lon: 0, zoom: 14, bearing: 0, pitch: 0 });
}

describe("<PoiSearchChips />", () => {
  beforeEach(() => {
    window.localStorage.clear();
    useMapStore.getState().clear();
  });
  afterEach(() => {
    window.localStorage.clear();
    useMapStore.getState().clear();
  });

  it("renders a chip for every POI category", () => {
    seedRegion();
    render(wrap(<PoiSearchChips />));
    const group = screen.getByRole("group", {
      name: /search pois by category/i,
    });
    const chips = within(group).getAllByRole("button");
    expect(chips).toHaveLength(POI_CATEGORIES.length);
    for (const cat of POI_CATEGORIES) {
      expect(group.querySelector(`[data-category="${cat}"]`)).not.toBeNull();
    }
  });

  it("stays visible even when there is no active region", () => {
    useMapStore.getState().setViewport({
      lat: 0,
      lon: 0,
      zoom: 14,
      bearing: 0,
      pitch: 0,
    });
    render(wrap(<PoiSearchChips />));
    expect(
      screen.getByRole("group", { name: /search pois by category/i }),
    ).toBeInTheDocument();
  });

  it("stays visible while zoomed out below the POI threshold", () => {
    useMapStore.getState().setActiveRegion("europe-romania");
    useMapStore.getState().setViewport({
      lat: 0,
      lon: 0,
      zoom: 8,
      bearing: 0,
      pitch: 0,
    });
    render(wrap(<PoiSearchChips />));
    expect(
      screen.getByRole("group", { name: /search pois by category/i }),
    ).toBeInTheDocument();
  });

  it("clicking a chip with no region installed does NOT activate it", async () => {
    const user = userEvent.setup();
    render(wrap(<PoiSearchChips />));

    const group = screen.getByRole("group", {
      name: /search pois by category/i,
    });
    const foodChip = group.querySelector(
      '[data-category="food"]',
    ) as HTMLButtonElement;

    await user.click(foodChip);

    // No region → soft no-op (toast event fires, but the chip stays
    // inactive so we don't kick off a /api/pois fetch with no region).
    expect(useMapStore.getState().activeCategoryChip).toBeNull();
  });

  it("clicking a chip promotes it to active without touching poiVisibility", async () => {
    const user = userEvent.setup();
    seedRegion();
    render(wrap(<PoiSearchChips />));

    const group = screen.getByRole("group", {
      name: /search pois by category/i,
    });
    const foodChip = group.querySelector(
      '[data-category="food"]',
    ) as HTMLButtonElement;
    expect(foodChip).not.toBeNull();
    expect(foodChip.dataset.active).toBe("false");

    const beforeVisibility = { ...useMapStore.getState().poiVisibility };
    await user.click(foodChip);

    expect(useMapStore.getState().activeCategoryChip).toBe("food");
    // Chip row hides once a chip is active (Change 2) — the data-active
    // attribute on the previously-rendered element no longer matters
    // because the row unmounts. The store flip is the source of truth.
    // Crucially — visibility must NOT change. Chips are now search
    // triggers, not visibility toggles.
    expect(useMapStore.getState().poiVisibility).toEqual(beforeVisibility);
  });

  it("clicking a chip flips the left rail to the categoryResults tab", async () => {
    const user = userEvent.setup();
    seedRegion();
    render(wrap(<PoiSearchChips />));

    const group = screen.getByRole("group", {
      name: /search pois by category/i,
    });
    const foodChip = group.querySelector(
      '[data-category="food"]',
    ) as HTMLButtonElement;

    expect(useMapStore.getState().leftRailTab).toBe("search");
    await user.click(foodChip);
    expect(useMapStore.getState().leftRailTab).toBe("categoryResults");
  });

  it("hides the chip row once a chip is active", async () => {
    const user = userEvent.setup();
    seedRegion();
    render(wrap(<PoiSearchChips />));

    const group = screen.getByRole("group", {
      name: /search pois by category/i,
    });
    const foodChip = group.querySelector(
      '[data-category="food"]',
    ) as HTMLButtonElement;

    await user.click(foodChip);
    expect(useMapStore.getState().activeCategoryChip).toBe("food");
    expect(useMapStore.getState().leftRailTab).toBe("categoryResults");
    // Row hides — the side panel owns the close affordance now.
    expect(
      screen.queryByRole("group", { name: /search pois by category/i }),
    ).not.toBeInTheDocument();
  });

  it("hides the chip row when the recents panel is open", () => {
    seedRegion();
    useMapStore.getState().openLeftRail("recents");
    render(wrap(<PoiSearchChips />));
    expect(
      screen.queryByRole("group", { name: /search pois by category/i }),
    ).not.toBeInTheDocument();
  });

  it("re-renders the chip row after closeCategoryResults runs", async () => {
    const user = userEvent.setup();
    seedRegion();
    render(wrap(<PoiSearchChips />));

    const group = screen.getByRole("group", {
      name: /search pois by category/i,
    });
    const foodChip = group.querySelector(
      '[data-category="food"]',
    ) as HTMLButtonElement;
    await user.click(foodChip);
    expect(useMapStore.getState().activeCategoryChip).toBe("food");

    // Simulate the panel's X button (full reset).
    act(() => {
      useMapStore.getState().closeCategoryResults();
    });

    expect(useMapStore.getState().activeCategoryChip).toBeNull();
    expect(
      screen.getByRole("group", { name: /search pois by category/i }),
    ).toBeInTheDocument();
  });
});

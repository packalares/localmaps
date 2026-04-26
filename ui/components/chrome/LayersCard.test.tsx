import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TooltipProvider } from "@/components/ui/tooltip";
import { ThemeProvider } from "@/components/providers/theme";
import {
  POI_CATEGORIES,
  useMapStore,
  defaultPoiVisibility,
} from "@/lib/state/map";
import { LayersCard } from "./LayersCard";

function wrap(children: React.ReactNode) {
  return (
    <ThemeProvider>
      <TooltipProvider delayDuration={0}>{children}</TooltipProvider>
    </ThemeProvider>
  );
}

describe("<LayersCard />", () => {
  beforeEach(() => {
    useMapStore.getState().clear();
    window.localStorage.removeItem("localmaps-theme");
  });
  afterEach(() => {
    useMapStore.getState().clear();
    window.localStorage.removeItem("localmaps-theme");
  });

  it("opens a popover with the POI controls", async () => {
    const user = userEvent.setup();
    render(wrap(<LayersCard />));

    await user.click(screen.getByRole("button", { name: /^layers$/i }));

    // Theme controls now live in the LeftIconRail's standalone Theme
    // button — the Layers popover only houses POI visibility.
    expect(screen.queryByRole("radio", { name: /light/i })).toBeNull();
    expect(screen.queryByRole("radio", { name: /dark/i })).toBeNull();
    expect(await screen.findByRole("switch")).toBeInTheDocument();
  });

  it("toggling 'Show POIs' off hides every POI category", async () => {
    useMapStore.getState().replacePoiVisibility(defaultPoiVisibility());
    const user = userEvent.setup();
    render(wrap(<LayersCard />));

    await user.click(screen.getByRole("button", { name: /^layers$/i }));
    const toggle = await screen.findByRole("switch");
    expect(toggle).toHaveAttribute("aria-checked", "true");

    await user.click(toggle);

    const next = useMapStore.getState().poiVisibility;
    for (const cat of POI_CATEGORIES) {
      expect(next[cat]).toBe(false);
    }
  });

  it("renders a checkbox for every POI category", async () => {
    useMapStore.getState().replacePoiVisibility(defaultPoiVisibility());
    const user = userEvent.setup();
    render(wrap(<LayersCard />));

    await user.click(screen.getByRole("button", { name: /^layers$/i }));

    const list = await screen.findByRole("group", {
      name: /poi categories/i,
    });
    const checkboxes = within(list).getAllByRole("checkbox");
    expect(checkboxes).toHaveLength(POI_CATEGORIES.length);
  });

  it("unchecking a category flips only that category's visibility", async () => {
    useMapStore.getState().replacePoiVisibility(defaultPoiVisibility());
    const user = userEvent.setup();
    render(wrap(<LayersCard />));

    await user.click(screen.getByRole("button", { name: /^layers$/i }));
    const list = await screen.findByRole("group", {
      name: /poi categories/i,
    });
    const foodCheckbox = list.querySelector(
      '[data-category="food"]',
    ) as HTMLInputElement;
    expect(foodCheckbox).not.toBeNull();
    expect(foodCheckbox.checked).toBe(true);

    await user.click(foodCheckbox);

    const next = useMapStore.getState().poiVisibility;
    expect(next.food).toBe(false);
    expect(next.shopping).toBe(true);
  });

  it("disables the category checkboxes while 'Show POIs' is off", async () => {
    // Seed all-off so the master switch reads as off.
    const allOff = {} as Record<(typeof POI_CATEGORIES)[number], boolean>;
    for (const c of POI_CATEGORIES) allOff[c] = false;
    useMapStore.getState().replacePoiVisibility(allOff);

    const user = userEvent.setup();
    render(wrap(<LayersCard />));
    await user.click(screen.getByRole("button", { name: /^layers$/i }));

    const list = await screen.findByRole("group", {
      name: /poi categories/i,
    });
    const checkboxes = within(list).getAllByRole("checkbox");
    for (const cb of checkboxes) {
      expect(cb).toBeDisabled();
    }
  });
});

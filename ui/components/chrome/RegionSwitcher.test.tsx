import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TooltipProvider } from "@/components/ui/tooltip";
import { useMapStore } from "@/lib/state/map";
import type { Region } from "@/lib/api/schemas";
import { RegionSwitcher } from "./RegionSwitcher";

function wrap(children: React.ReactNode) {
  return <TooltipProvider delayDuration={0}>{children}</TooltipProvider>;
}

function seedRegions(regions: Region[]) {
  useMapStore.getState().setInstalledRegions(regions);
}

const romania: Region = {
  name: "europe/romania",
  displayName: "Romania",
  sourceUrl: "https://download.geofabrik.de/europe/romania.osm.pbf",
  state: "ready",
};
const germany: Region = {
  name: "europe/germany",
  displayName: "Germany",
  sourceUrl: "https://download.geofabrik.de/europe/germany.osm.pbf",
  state: "ready",
};
const downloading: Region = {
  name: "europe/france",
  displayName: "France",
  sourceUrl: "https://download.geofabrik.de/europe/france.osm.pbf",
  state: "downloading",
};

describe("<RegionSwitcher />", () => {
  beforeEach(() => {
    useMapStore.getState().clear();
  });
  afterEach(() => {
    useMapStore.getState().clear();
  });

  it("shows the disabled 'No regions installed' affordance when empty", () => {
    render(wrap(<RegionSwitcher />));
    const btn = screen.getByRole("button", { name: /no regions installed/i });
    expect(btn).toBeDisabled();
  });

  it("treats downloading-only regions as empty", () => {
    seedRegions([downloading]);
    render(wrap(<RegionSwitcher />));
    expect(
      screen.getByRole("button", { name: /no regions installed/i }),
    ).toBeDisabled();
  });

  it("renders the list of ready regions and selects on click", async () => {
    seedRegions([romania, germany, downloading]);
    const user = userEvent.setup();
    render(wrap(<RegionSwitcher />));

    const trigger = screen.getByRole("button", {
      name: /choose active region/i,
    });
    await user.click(trigger);

    // All 2 ready regions + "All regions" entry in the menu
    expect(
      await screen.findByRole("menuitem", { name: /switch to romania/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("menuitem", { name: /switch to germany/i }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("menuitem", { name: /france/i }),
    ).not.toBeInTheDocument();

    await user.click(
      screen.getByRole("menuitem", { name: /switch to romania/i }),
    );
    expect(useMapStore.getState().activeRegion).toBe("europe-romania");
  });

  it("'All regions' resets the selection to null", async () => {
    seedRegions([romania]);
    useMapStore.getState().setActiveRegion("europe-romania");
    const user = userEvent.setup();
    render(wrap(<RegionSwitcher />));
    await user.click(
      screen.getByRole("button", { name: /choose active region/i }),
    );
    await user.click(
      await screen.findByRole("menuitem", {
        name: /show all installed regions/i,
      }),
    );
    expect(useMapStore.getState().activeRegion).toBeNull();
  });
});

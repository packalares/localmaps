import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TooltipProvider } from "@/components/ui/tooltip";
import { ThemeProvider } from "@/components/providers/theme";
import { LocaleProvider } from "@/lib/i18n/provider";
import { useMapStore } from "@/lib/state/map";
import type { GeocodeResult, Region } from "@/lib/api/schemas";
import { LOCALE_STORAGE_KEY } from "@/lib/i18n/types";
import { LeftIconRail } from "./LeftIconRail";

function wrap(children: React.ReactNode) {
  return (
    <ThemeProvider>
      <TooltipProvider delayDuration={0}>
        <LocaleProvider initialLocale="en">{children}</LocaleProvider>
      </TooltipProvider>
    </ThemeProvider>
  );
}

const bucharest: GeocodeResult = {
  id: "city/bucharest",
  label: "Bucharest, Romania",
  center: { lat: 44.4268, lon: 26.1025 },
  confidence: 0.95,
};
const brasov: GeocodeResult = {
  id: "city/brasov",
  label: "Brașov, Romania",
  center: { lat: 45.6579, lon: 25.6012 },
  confidence: 0.93,
};
const kids: GeocodeResult = {
  id: "city/kids",
  label: "Kids, Romania",
  center: { lat: 44.5, lon: 26.2 },
  confidence: 0.9,
};
const kos: GeocodeResult = {
  id: "city/kos",
  label: "Kos, Greece",
  center: { lat: 36.88, lon: 27.29 },
  confidence: 0.92,
};

const romania: Region = {
  name: "europe/romania",
  displayName: "Romania",
  sourceUrl: "https://example.test/romania.osm.pbf",
  state: "ready",
};

describe("<LeftIconRail />", () => {
  beforeEach(() => {
    useMapStore.getState().clear();
    window.localStorage.removeItem("localmaps.search.history.v1");
    window.localStorage.removeItem(LOCALE_STORAGE_KEY);
  });
  afterEach(() => {
    useMapStore.getState().clear();
    window.localStorage.removeItem("localmaps.search.history.v1");
    window.localStorage.removeItem(LOCALE_STORAGE_KEY);
  });

  it("renders a full-height rail wrapper (w-14, inset-y-0, fixed)", () => {
    render(wrap(<LeftIconRail />));
    const nav = screen.getByRole("navigation");
    expect(nav.className).toContain("fixed");
    expect(nav.className).toContain("inset-y-0");
    expect(nav.className).toContain("left-0");
    expect(nav.className).toContain("w-14");
  });

  it("Saved / Recents triggers flip the left-rail tab", async () => {
    render(wrap(<LeftIconRail />));
    const user = userEvent.setup();

    await user.click(screen.getByRole("button", { name: /saved places/i }));
    expect(useMapStore.getState().leftRailTab).toBe("saved");

    await user.click(screen.getByRole("button", { name: /recent searches/i }));
    expect(useMapStore.getState().leftRailTab).toBe("recents");
  });

  it("shows avatars for up to four recent history entries", () => {
    window.localStorage.setItem(
      "localmaps.search.history.v1",
      JSON.stringify([bucharest, brasov, kids, kos, { ...kos, id: "fifth" }]),
    );
    render(wrap(<LeftIconRail />));

    for (const entry of [bucharest, brasov, kids, kos]) {
      expect(
        screen.getByRole("button", {
          name: new RegExp(`recenter map on ${entry.label}`, "i"),
        }),
      ).toBeInTheDocument();
    }
    // A fifth entry should be suppressed by MAX_AVATARS.
    expect(
      screen.queryAllByRole("button", { name: /recenter map on kos/i }),
    ).toHaveLength(1);
  });

  it("clicking an avatar recenters the map on that entry", async () => {
    window.localStorage.setItem(
      "localmaps.search.history.v1",
      JSON.stringify([bucharest]),
    );
    // Seed a user-set zoom so we can verify the avatar click preserves
    // the existing zoom level (pan-only behaviour) instead of forcing
    // the camera back to a hard-coded value.
    useMapStore.getState().setViewport({
      lat: 0,
      lon: 0,
      zoom: 11,
      bearing: 0,
      pitch: 0,
    });
    const user = userEvent.setup();
    render(wrap(<LeftIconRail />));

    const avatar = await screen.findByRole("button", {
      name: /recenter map on bucharest/i,
    });
    await user.click(avatar);

    const vp = useMapStore.getState().viewport;
    expect(vp.lat).toBeCloseTo(44.4268, 3);
    expect(vp.lon).toBeCloseTo(26.1025, 3);
    expect(vp.zoom).toBe(11);
  });

  it("region trigger opens a menu with installed regions", async () => {
    useMapStore.getState().setInstalledRegions([romania]);
    const user = userEvent.setup();
    render(wrap(<LeftIconRail />));

    await user.click(
      screen.getByRole("button", { name: /choose active region/i }),
    );
    expect(
      await screen.findByRole("menuitem", { name: /switch to romania/i }),
    ).toBeInTheDocument();
  });

  it("theme trigger flips the document's `dark` class on click", async () => {
    const user = userEvent.setup();
    document.documentElement.classList.remove("dark");
    render(wrap(<LeftIconRail />));

    // Initial state: light → button shows "Switch to dark theme".
    const trigger = await screen.findByRole("button", {
      name: /switch to dark theme/i,
    });
    await user.click(trigger);

    expect(document.documentElement.classList.contains("dark")).toBe(true);
    // After the toggle, the button now offers the inverse switch.
    expect(
      screen.getByRole("button", { name: /switch to light theme/i }),
    ).toBeInTheDocument();
  });

  it("language trigger opens a popover with supported locales", async () => {
    const user = userEvent.setup();
    render(wrap(<LeftIconRail />));

    await user.click(
      screen.getByRole("button", { name: /choose interface language/i }),
    );
    expect(
      await screen.findByRole("option", { name: /switch to english/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("option", { name: /switch to română/i }),
    ).toBeInTheDocument();
  });
});

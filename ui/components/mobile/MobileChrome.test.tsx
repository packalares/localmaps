import { beforeEach, describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { TooltipProvider } from "@/components/ui/tooltip";
import { useMapStore } from "@/lib/state/map";
import { MobileChrome } from "./MobileChrome";

function wrap(ui: React.ReactElement) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={qc}>
      <TooltipProvider delayDuration={0}>{ui}</TooltipProvider>
    </QueryClientProvider>,
  );
}

describe("<MobileChrome />", () => {
  beforeEach(() => {
    useMapStore.setState((s) => ({
      ...s,
      leftRailTab: "search",
      selectedPoi: null,
      selectedResult: null,
      pendingClick: null,
    }));
  });

  it("renders the search bar pinned to the top", () => {
    wrap(<MobileChrome />);
    expect(
      screen.getByRole("searchbox", { name: /search maps/i }),
    ).toBeInTheDocument();
  });

  it("renders the bottom sheet as a dialog", () => {
    wrap(<MobileChrome />);
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("renders the bottom nav with Search / Directions / Saved tabs by default", () => {
    wrap(<MobileChrome />);
    expect(screen.getByRole("button", { name: "Search" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Directions" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Saved" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Place" })).toBeNull();
  });

  it("shows the Place tab once a POI is selected", () => {
    useMapStore.setState((s) => ({
      ...s,
      selectedPoi: { id: "x", label: "Test", lat: 0, lon: 0 },
    }));
    wrap(<MobileChrome />);
    expect(screen.getByRole("button", { name: "Place" })).toBeInTheDocument();
  });

  it("tapping a nav tab switches the store", async () => {
    const user = userEvent.setup();
    wrap(<MobileChrome />);
    await user.click(screen.getByRole("button", { name: "Directions" }));
    expect(useMapStore.getState().leftRailTab).toBe("directions");
  });
});

import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useMapStore } from "@/lib/state/map";
import { BottomNav } from "./BottomNav";

describe("<BottomNav />", () => {
  beforeEach(() => {
    useMapStore.setState((s) => ({
      ...s,
      leftRailTab: "search",
      selectedPoi: null,
    }));
  });

  it("hides the Place tab when hasPlace is false", () => {
    render(<BottomNav snap="peek" onSnapChange={() => {}} hasPlace={false} />);
    expect(screen.queryByRole("button", { name: "Place" })).toBeNull();
    expect(screen.getByRole("button", { name: "Search" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Directions" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Saved" })).toBeInTheDocument();
  });

  it("shows the Place tab when hasPlace is true", () => {
    render(<BottomNav snap="peek" onSnapChange={() => {}} hasPlace={true} />);
    expect(screen.getByRole("button", { name: "Place" })).toBeInTheDocument();
  });

  it("marks the active tab via aria-pressed", () => {
    useMapStore.setState((s) => ({ ...s, leftRailTab: "directions" }));
    render(<BottomNav snap="peek" onSnapChange={() => {}} hasPlace={false} />);
    expect(screen.getByRole("button", { name: "Search" })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
    expect(screen.getByRole("button", { name: "Directions" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });

  it("tapping a tab writes it into the store", async () => {
    const user = userEvent.setup();
    render(<BottomNav snap="peek" onSnapChange={() => {}} hasPlace={false} />);
    await user.click(screen.getByRole("button", { name: "Directions" }));
    expect(useMapStore.getState().leftRailTab).toBe("directions");
  });

  it("tapping any tab at `peek` promotes the sheet to `half`", async () => {
    const user = userEvent.setup();
    const onSnap = vi.fn();
    render(<BottomNav snap="peek" onSnapChange={onSnap} hasPlace={false} />);
    await user.click(screen.getByRole("button", { name: "Search" }));
    expect(onSnap).toHaveBeenCalledWith("half");
  });

  it("tapping the current tab while at `half` collapses to `peek`", async () => {
    const user = userEvent.setup();
    useMapStore.setState((s) => ({ ...s, leftRailTab: "search" }));
    const onSnap = vi.fn();
    render(<BottomNav snap="half" onSnapChange={onSnap} hasPlace={false} />);
    await user.click(screen.getByRole("button", { name: "Search" }));
    expect(onSnap).toHaveBeenCalledWith("peek");
  });
});

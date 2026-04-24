import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useActiveToolStore } from "@/lib/tools/active-tool";
import { useMeasureStore } from "@/lib/tools/measure-state";
import { useIsochroneStore } from "@/lib/tools/isochrone-state";
import { ToolsFab } from "./ToolsFab";

describe("<ToolsFab />", () => {
  beforeEach(() => {
    useActiveToolStore.setState({ active: null });
    useMeasureStore.getState().clear();
    useIsochroneStore.getState().clear();
  });
  afterEach(() => {
    useActiveToolStore.setState({ active: null });
    useMeasureStore.getState().clear();
    useIsochroneStore.getState().clear();
  });

  it("opens the popover and activates Measure", async () => {
    const user = userEvent.setup();
    render(<ToolsFab />);
    await user.click(screen.getByRole("button", { name: /^tools$/i }));
    await user.click(
      screen.getByRole("button", { name: /measure.*distance or area/i }),
    );
    expect(useActiveToolStore.getState().active).toBe("measure");
    expect(useMeasureStore.getState().isActive).toBe(true);
  });

  it("switching tools cancels the other", async () => {
    // The coordinator enforces only-one-active-at-a-time; we exercise it
    // directly since Radix popover uses a portal + focus-trap which makes
    // two successive open/pick cycles unreliable in jsdom.
    useActiveToolStore.getState().setActive("measure");
    useMeasureStore.getState().addPoint({ lng: 1, lat: 1 });
    expect(useMeasureStore.getState().isActive).toBe(true);

    useActiveToolStore.getState().setActive("isochrone");
    expect(useActiveToolStore.getState().active).toBe("isochrone");
    expect(useMeasureStore.getState().isActive).toBe(false);
    expect(useMeasureStore.getState().points).toEqual([]);
    expect(useIsochroneStore.getState().isActive).toBe(true);
  });

  it("Close all resets the coordinator", async () => {
    const user = userEvent.setup();
    useActiveToolStore.setState({ active: "measure" });
    useMeasureStore.setState({
      isActive: true,
      points: [{ lng: 1, lat: 1 }],
    });
    render(<ToolsFab />);

    await user.click(
      screen.getByRole("button", { name: /tools \(measure active\)/i }),
    );
    await user.click(screen.getByRole("button", { name: /close all/i }));
    expect(useActiveToolStore.getState().active).toBeNull();
  });
});

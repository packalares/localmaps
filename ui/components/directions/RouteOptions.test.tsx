import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RouteOptions } from "./RouteOptions";

const defaults = {
  avoidHighways: false,
  avoidTolls: false,
  avoidFerries: false,
};

describe("<RouteOptions />", () => {
  it("reveals the checkboxes when the expander is opened", async () => {
    const user = userEvent.setup();
    render(<RouteOptions value={defaults} onChange={() => {}} />);
    await user.click(screen.getByRole("button", { name: /route options/i }));
    expect(
      screen.getByRole("checkbox", { name: /avoid highways/i }),
    ).toBeInTheDocument();
  });

  it("fires a partial patch when a checkbox is toggled", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<RouteOptions value={defaults} onChange={onChange} />);
    await user.click(screen.getByRole("button", { name: /route options/i }));
    await user.click(
      screen.getByRole("checkbox", { name: /avoid ferries/i }),
    );
    expect(onChange).toHaveBeenCalledWith({ avoidFerries: true });
  });
});

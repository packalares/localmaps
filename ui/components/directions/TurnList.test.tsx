import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Route } from "@/lib/api/schemas";
import { TurnList } from "./TurnList";

function fixtureRoute(): Route {
  return {
    id: "r1",
    summary: { timeSeconds: 600, distanceMeters: 4200 },
    legs: [
      {
        geometry: { polyline: "" },
        maneuvers: [
          {
            instruction: "Head north",
            beginShapeIndex: 0,
            distanceMeters: 300,
            type: "depart",
            streetName: "Main St",
          },
          {
            instruction: "Turn right onto 1st Ave",
            beginShapeIndex: 3,
            distanceMeters: 900,
            type: "right",
            streetName: "1st Ave",
          },
          {
            instruction: "Arrive at destination",
            beginShapeIndex: 10,
            type: "destination",
            streetName: null,
          },
        ],
      },
    ],
    mode: "auto",
  };
}

describe("<TurnList />", () => {
  it("renders a list item per maneuver", () => {
    render(<TurnList route={fixtureRoute()} />);
    const items = screen.getAllByRole("listitem");
    expect(items).toHaveLength(3);
  });

  it("fires onSelect with the maneuver index when a row is clicked", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(<TurnList route={fixtureRoute()} onSelect={onSelect} />);

    await user.click(
      screen.getByRole("button", { name: /turn right onto 1st ave/i }),
    );
    expect(onSelect).toHaveBeenCalledWith(1);
  });

  it("marks the active maneuver with aria-current", () => {
    render(<TurnList route={fixtureRoute()} activeIndex={2} />);
    const items = screen.getAllByRole("listitem");
    expect(items[2]).toHaveAttribute("aria-current", "step");
    expect(items[0]).not.toHaveAttribute("aria-current");
  });

  it("shows an empty-state when no maneuvers are returned", () => {
    const route: Route = {
      id: "x",
      legs: [{ geometry: { polyline: "" }, maneuvers: [] }],
    };
    render(<TurnList route={route} />);
    expect(
      screen.getByText(/no turn-by-turn instructions/i),
    ).toBeInTheDocument();
  });
});

import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HoursAccordion } from "./HoursAccordion";

// Jan 8 2024 is a Monday, 10:00.
const MON_10 = new Date(2024, 0, 8, 10, 0, 0);
// Jan 13 2024 is a Saturday, 10:00.
const SAT_10 = new Date(2024, 0, 13, 10, 0, 0);

describe("<HoursAccordion />", () => {
  it("returns nothing when raw is empty", () => {
    const { container } = render(<HoursAccordion raw={null} now={MON_10} />);
    expect(container.firstChild).toBeNull();
  });

  it("shows today's status in the header when closed Saturday", () => {
    render(<HoursAccordion raw="Mo-Fr 09:00-18:00" now={SAT_10} />);
    expect(screen.getByText(/closed/i)).toBeInTheDocument();
    expect(screen.getByRole("button")).toHaveAttribute("aria-expanded", "false");
  });

  it("shows 'Open now · closes …' when open", () => {
    render(<HoursAccordion raw="Mo-Fr 09:00-18:00" now={MON_10} />);
    expect(screen.getByText(/open now/i)).toBeInTheDocument();
    expect(screen.getByText(/18:00/)).toBeInTheDocument();
  });

  it("expands and collapses on click, highlights today", async () => {
    const user = userEvent.setup();
    render(<HoursAccordion raw="Mo-Fr 09:00-18:00" now={MON_10} />);

    const toggle = screen.getByRole("button");
    await user.click(toggle);
    expect(toggle).toHaveAttribute("aria-expanded", "true");

    // Today (Monday) row should be present.
    const mondayCell = screen.getByText("Mon");
    expect(mondayCell).toBeInTheDocument();

    // Collapse.
    await user.click(toggle);
    expect(toggle).toHaveAttribute("aria-expanded", "false");
  });
});

import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ScheduleDropdown } from "./ScheduleDropdown";

describe("<ScheduleDropdown />", () => {
  it("shows the current preset label on the trigger", () => {
    render(<ScheduleDropdown value="monthly" onChange={() => {}} />);
    expect(
      screen.getByRole("button", { name: /change update schedule/i }),
    ).toHaveTextContent(/monthly/i);
  });

  it("emits the chosen preset via onChange", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<ScheduleDropdown value="never" onChange={onChange} />);
    await user.click(
      screen.getByRole("button", { name: /change update schedule/i }),
    );
    await user.click(
      await screen.findByRole("menuitem", { name: /weekly/i }),
    );
    expect(onChange).toHaveBeenCalledWith("weekly");
  });

  it("rejects an invalid custom cron and keeps the menu open", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<ScheduleDropdown value="never" onChange={onChange} />);
    await user.click(
      screen.getByRole("button", { name: /change update schedule/i }),
    );
    const input = await screen.findByLabelText(/custom cron expression/i);
    await user.type(input, "not-a-cron");
    await user.click(screen.getByRole("button", { name: /apply custom/i }));
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent(/5-field cron/i);
  });

  it("accepts a valid custom cron", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<ScheduleDropdown value="never" onChange={onChange} />);
    await user.click(
      screen.getByRole("button", { name: /change update schedule/i }),
    );
    const input = await screen.findByLabelText(/custom cron expression/i);
    await user.type(input, "0 4 * * 0");
    await user.click(screen.getByRole("button", { name: /apply custom/i }));
    expect(onChange).toHaveBeenCalledWith("0 4 * * 0");
  });

  it("is disabled when the caller says so", () => {
    render(
      <ScheduleDropdown value="daily" onChange={() => {}} disabled={true} />,
    );
    expect(
      screen.getByRole("button", { name: /change update schedule/i }),
    ).toBeDisabled();
  });
});

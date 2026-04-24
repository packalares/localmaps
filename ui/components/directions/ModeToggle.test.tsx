import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ModeToggle } from "./ModeToggle";

describe("<ModeToggle />", () => {
  it("renders the selected mode with aria-selected=true", () => {
    render(<ModeToggle value="bicycle" onChange={() => {}} />);
    const cycling = screen.getByRole("tab", { name: /cycling/i });
    expect(cycling).toHaveAttribute("aria-selected", "true");
    const driving = screen.getByRole("tab", { name: /driving/i });
    expect(driving).toHaveAttribute("aria-selected", "false");
  });

  it("fires onChange with the selected mode on click", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<ModeToggle value="auto" onChange={onChange} />);
    await user.click(screen.getByRole("tab", { name: /walking/i }));
    expect(onChange).toHaveBeenCalledWith("pedestrian");
  });

  it("hides modes omitted from the `modes` prop", () => {
    render(
      <ModeToggle
        value="auto"
        onChange={() => {}}
        modes={["auto", "pedestrian"]}
      />,
    );
    expect(screen.queryByRole("tab", { name: /truck/i })).toBeNull();
    expect(screen.queryByRole("tab", { name: /cycling/i })).toBeNull();
  });
});

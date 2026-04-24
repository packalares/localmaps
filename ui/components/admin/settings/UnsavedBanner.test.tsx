import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { UnsavedBanner } from "./UnsavedBanner";

describe("<UnsavedBanner />", () => {
  it("is hidden when clean and no error", () => {
    const { container } = render(
      <UnsavedBanner
        dirtyCount={0}
        canSave
        pending={false}
        onSave={() => {}}
        onRevert={() => {}}
      />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("renders count + wires both buttons", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    const onRevert = vi.fn();
    render(
      <UnsavedBanner
        dirtyCount={3}
        canSave
        pending={false}
        onSave={onSave}
        onRevert={onRevert}
      />,
    );
    expect(screen.getByText(/3 unsaved changes/i)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /save all/i }));
    await user.click(screen.getByRole("button", { name: /revert/i }));
    expect(onSave).toHaveBeenCalledOnce();
    expect(onRevert).toHaveBeenCalledOnce();
  });

  it("disables save when canSave is false", () => {
    render(
      <UnsavedBanner
        dirtyCount={1}
        canSave={false}
        pending={false}
        onSave={() => {}}
        onRevert={() => {}}
      />,
    );
    expect(screen.getByRole("button", { name: /save all/i })).toBeDisabled();
  });

  it("surfaces a server error message", () => {
    render(
      <UnsavedBanner
        dirtyCount={0}
        canSave
        pending={false}
        onSave={() => {}}
        onRevert={() => {}}
        errorMessage="map.maxZoom: 99 above maximum 19"
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent(/99 above maximum/);
  });
});

import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DeleteDialog } from "./DeleteDialog";

describe("<DeleteDialog />", () => {
  it("keeps the delete button disabled until the typed name matches", async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn();
    render(
      <DeleteDialog
        open={true}
        onOpenChange={() => {}}
        regionName="europe-romania"
        displayName="Romania"
        onConfirm={onConfirm}
      />,
    );
    const deleteBtn = screen.getByRole("button", { name: /delete romania/i });
    expect(deleteBtn).toBeDisabled();

    const input = screen.getByLabelText(/confirm region name/i);
    await user.type(input, "europe-rom");
    expect(deleteBtn).toBeDisabled();
    await user.type(input, "ania");
    expect(deleteBtn).toBeEnabled();
  });

  it("fires onConfirm when the name matches and the button is clicked", async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn();
    render(
      <DeleteDialog
        open={true}
        onOpenChange={() => {}}
        regionName="europe-germany"
        displayName="Germany"
        onConfirm={onConfirm}
      />,
    );
    await user.type(
      screen.getByLabelText(/confirm region name/i),
      "europe-germany",
    );
    await user.click(screen.getByRole("button", { name: /delete germany/i }));
    expect(onConfirm).toHaveBeenCalledOnce();
  });

  it("closes on Cancel", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    render(
      <DeleteDialog
        open={true}
        onOpenChange={onOpenChange}
        regionName="x"
        displayName="X"
        onConfirm={() => {}}
      />,
    );
    await user.click(screen.getByRole("button", { name: /cancel/i }));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("renders a pending state on the confirm button", () => {
    render(
      <DeleteDialog
        open={true}
        onOpenChange={() => {}}
        regionName="x"
        displayName="X"
        onConfirm={() => {}}
        pending={true}
      />,
    );
    expect(
      screen.getByRole("button", { name: /delete x/i }),
    ).toHaveTextContent(/deleting/i);
  });

  it("surfaces an error message when provided", () => {
    render(
      <DeleteDialog
        open={true}
        onOpenChange={() => {}}
        regionName="x"
        displayName="X"
        onConfirm={() => {}}
        errorMessage="Nope"
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent(/nope/i);
  });
});

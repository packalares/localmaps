import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { BottomSheet, type SheetSnap } from "./BottomSheet";

function Harness(props: {
  initial?: SheetSnap;
  onSnap?: (s: SheetSnap) => void;
  onRequestClose?: () => void;
}) {
  const [snap, setSnap] = useState<SheetSnap>(props.initial ?? "peek");
  return (
    <BottomSheet
      snap={snap}
      onSnapChange={(s) => {
        setSnap(s);
        props.onSnap?.(s);
      }}
      onRequestClose={props.onRequestClose}
      header={<button type="button">hdr-btn</button>}
    >
      <button type="button">body-btn-1</button>
      <button type="button">body-btn-2</button>
    </BottomSheet>
  );
}

describe("<BottomSheet />", () => {
  it("renders a dialog with the drag handle as initial focus target", () => {
    render(<Harness />);
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /drag handle/i }),
    ).toBeInTheDocument();
  });

  it("Arrow Up on the handle promotes peek → half → full", async () => {
    const user = userEvent.setup();
    const onSnap = vi.fn();
    render(<Harness onSnap={onSnap} />);
    const handle = screen.getByRole("button", { name: /drag handle/i });
    handle.focus();
    await user.keyboard("{ArrowUp}");
    expect(onSnap).toHaveBeenCalledWith("half");
    await user.keyboard("{ArrowUp}");
    expect(onSnap).toHaveBeenCalledWith("full");
  });

  it("Arrow Down on the handle collapses full → half → peek", async () => {
    const user = userEvent.setup();
    const onSnap = vi.fn();
    render(<Harness initial="full" onSnap={onSnap} />);
    const handle = screen.getByRole("button", { name: /drag handle/i });
    handle.focus();
    await user.keyboard("{ArrowDown}");
    expect(onSnap).toHaveBeenCalledWith("half");
    await user.keyboard("{ArrowDown}");
    expect(onSnap).toHaveBeenCalledWith("peek");
  });

  it("Escape at full demotes to half", async () => {
    const user = userEvent.setup();
    const onSnap = vi.fn();
    render(<Harness initial="full" onSnap={onSnap} />);
    await user.keyboard("{Escape}");
    expect(onSnap).toHaveBeenLastCalledWith("half");
  });

  it("Escape at peek fires onRequestClose", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(<Harness initial="peek" onRequestClose={onClose} />);
    await user.keyboard("{Escape}");
    expect(onClose).toHaveBeenCalled();
  });

  it("omits aria-modal at half and sets it at full", () => {
    const { rerender } = render(
      <BottomSheet snap="half" onSnapChange={() => {}}>
        <button type="button">x</button>
      </BottomSheet>,
    );
    expect(screen.getByRole("dialog")).not.toHaveAttribute("aria-modal");
    rerender(
      <BottomSheet snap="full" onSnapChange={() => {}}>
        <button type="button">x</button>
      </BottomSheet>,
    );
    expect(screen.getByRole("dialog")).toHaveAttribute("aria-modal", "true");
  });
});

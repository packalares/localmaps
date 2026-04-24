import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { SettingsSchemaNode } from "@/lib/api/schemas";
import { SettingsForm } from "./SettingsForm";

const nodes: SettingsSchemaNode[] = [
  {
    key: "map.style",
    type: "enum",
    uiGroup: "map",
    default: "light",
    enumValues: ["light", "dark", "auto"],
  },
  {
    key: "map.maxZoom",
    type: "integer",
    uiGroup: "map",
    default: 14,
    minimum: 0,
    maximum: 19,
  },
  {
    key: "search.showHistory",
    type: "boolean",
    uiGroup: "search",
    default: true,
  },
];

const tree = {
  map: { style: "light", maxZoom: 14 },
  search: { showHistory: true },
};

describe("<SettingsForm />", () => {
  it("renders every group from the schema", () => {
    render(
      <SettingsForm tree={tree} nodes={nodes} onSave={() => {}} />,
    );
    expect(screen.getByRole("region", { name: /map/i })).toBeInTheDocument();
    expect(
      screen.getByRole("region", { name: /search/i }),
    ).toBeInTheDocument();
  });

  it("marks the form dirty and enables Save when a field changes", async () => {
    const user = userEvent.setup();
    render(
      <SettingsForm tree={tree} nodes={nodes} onSave={() => {}} />,
    );
    // No unsaved banner yet.
    expect(
      screen.queryByRole("region", { name: /unsaved settings/i }),
    ).toBeNull();

    await user.selectOptions(
      screen.getByLabelText(/^style$/i),
      "dark",
    );

    const saveBtn = await screen.findByRole("button", { name: /save all/i });
    expect(saveBtn).toBeEnabled();
  });

  it("sends only the diff to onSave", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    render(
      <SettingsForm tree={tree} nodes={nodes} onSave={onSave} />,
    );
    await user.selectOptions(
      screen.getByLabelText(/^style$/i),
      "dark",
    );
    await user.click(await screen.findByRole("button", { name: /save all/i }));
    expect(onSave).toHaveBeenCalledWith({ "map.style": "dark" });
  });

  it("disables save when there is an inline validation error", async () => {
    const user = userEvent.setup();
    render(
      <SettingsForm tree={tree} nodes={nodes} onSave={() => {}} />,
    );
    const maxZoom = screen.getByLabelText(/^max zoom$/i);
    await user.clear(maxZoom);
    await user.type(maxZoom, "99");

    const saveBtn = screen.getByRole("button", { name: /save all/i });
    expect(saveBtn).toBeDisabled();
    expect(screen.getByRole("alert")).toHaveTextContent(/≤ 19/);
  });

  it("filters visible fields by the search box", async () => {
    const user = userEvent.setup();
    render(
      <SettingsForm tree={tree} nodes={nodes} onSave={() => {}} />,
    );
    await user.type(
      screen.getByLabelText(/filter settings/i),
      "showHistory",
    );
    expect(
      screen.getByRole("region", { name: /search/i }),
    ).toBeInTheDocument();
    // The map group is filtered out.
    expect(screen.queryByRole("region", { name: /^map$/i })).toBeNull();
  });

  it("reverts changes back to baseline", async () => {
    const user = userEvent.setup();
    render(
      <SettingsForm tree={tree} nodes={nodes} onSave={() => {}} />,
    );
    await user.selectOptions(
      screen.getByLabelText(/^style$/i),
      "dark",
    );
    await user.click(await screen.findByRole("button", { name: /revert/i }));
    expect(
      (screen.getByLabelText(/^style$/i) as HTMLSelectElement).value,
    ).toBe("light");
  });
});

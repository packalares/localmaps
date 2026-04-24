import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { SettingsSchemaNode } from "@/lib/api/schemas";
import { SettingField } from "./SettingField";

const enumNode: SettingsSchemaNode = {
  key: "map.style",
  type: "enum",
  uiGroup: "map",
  default: "light",
  enumValues: ["light", "dark", "auto"],
  description: "Default map theme.",
};

const boolNode: SettingsSchemaNode = {
  key: "map.rotationEnabled",
  type: "boolean",
  uiGroup: "map",
  default: true,
};

const intNode: SettingsSchemaNode = {
  key: "map.maxZoom",
  type: "integer",
  uiGroup: "map",
  default: 14,
  minimum: 0,
  maximum: 19,
};

const arrayNode: SettingsSchemaNode = {
  key: "pois.sources",
  type: "array",
  itemType: "string",
  uiGroup: "pois",
  default: ["overture"],
};

const objNode: SettingsSchemaNode = {
  key: "map.defaultCenter",
  type: "object",
  uiGroup: "map",
  default: { lat: 0, lon: 0, zoom: 2 },
};

describe("<SettingField />", () => {
  it("renders an enum select and fires onChange on selection", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<SettingField node={enumNode} value="light" onChange={onChange} />);
    await user.selectOptions(
      screen.getByRole("combobox"),
      "dark",
    );
    expect(onChange).toHaveBeenCalledWith("dark");
  });

  it("renders a switch for booleans and toggles", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<SettingField node={boolNode} value={true} onChange={onChange} />);
    await user.click(screen.getByRole("switch"));
    expect(onChange).toHaveBeenCalledWith(false);
  });

  it("renders an integer number input", () => {
    const onChange = vi.fn();
    render(<SettingField node={intNode} value={14} onChange={onChange} />);
    const input = screen.getByRole("spinbutton") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "10" } });
    expect(onChange).toHaveBeenLastCalledWith(10);
  });

  it("renders the string-array editor and handles add/remove", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <SettingField
        node={arrayNode}
        value={["overture", "osm"]}
        onChange={onChange}
      />,
    );
    await user.click(screen.getByRole("button", { name: /^add$/i }));
    expect(onChange).toHaveBeenCalledWith(["overture", "osm", ""]);

    await user.click(
      screen.getByRole("button", { name: /remove item 1/i }),
    );
    expect(onChange).toHaveBeenLastCalledWith(["osm"]);
  });

  it("renders a JSON textarea for object fields and emits parsed value", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <SettingField
        node={objNode}
        value={{ lat: 0, lon: 0, zoom: 2 }}
        onChange={onChange}
      />,
    );
    const ta = screen.getByRole("textbox");
    await user.clear(ta);
    await user.type(ta, '{{"lat":1,"lon":2,"zoom":3}');
    // The userEvent `{{` escapes to a literal `{`. Last parse should
    // yield the final object.
    const calls = onChange.mock.calls;
    expect(calls[calls.length - 1][0]).toEqual({ lat: 1, lon: 2, zoom: 3 });
  });

  it("shows the inline error and marks the input invalid", () => {
    render(
      <SettingField
        node={intNode}
        value={99}
        onChange={() => {}}
        error="Must be ≤ 19."
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent(/≤ 19/);
    expect(screen.getByRole("spinbutton")).toHaveAttribute("aria-invalid", "true");
  });
});

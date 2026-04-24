import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { EmbedAttribution } from "./EmbedAttribution";

describe("<EmbedAttribution />", () => {
  it("renders the default OSM + Overture credit", () => {
    render(<EmbedAttribution />);
    const el = screen.getByRole("contentinfo", { name: /map attribution/i });
    expect(el).toBeInTheDocument();
    expect(el.textContent).toMatch(/OpenStreetMap/i);
    expect(el.textContent).toMatch(/Overture/i);
  });

  it("accepts a custom text override", () => {
    render(<EmbedAttribution text="Custom credit" />);
    expect(screen.getByText("Custom credit")).toBeInTheDocument();
  });

  it("applies extra classes without dropping the base ones", () => {
    render(<EmbedAttribution className="mt-4" />);
    const el = screen.getByRole("contentinfo");
    expect(el.className).toContain("mt-4");
    // A couple of the base utility classes the component always emits.
    expect(el.className).toContain("rounded-md");
    expect(el.className).toContain("text-[11px]");
  });
});

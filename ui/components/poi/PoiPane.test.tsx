import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Poi } from "@/lib/api/schemas";
import { PoiPane } from "./PoiPane";

const FIXED_MON_10AM = new Date(2024, 0, 8, 10, 0, 0); // Jan 8 2024 is Monday

function makePoi(partial: Partial<Poi> = {}): Poi {
  return {
    id: "p1",
    label: "Cafe Lume",
    category: "cafe",
    center: { lat: 52.5, lon: 13.4 },
    tags: {
      "addr:street": "Hauptstraße",
      "addr:housenumber": "12",
      "addr:postcode": "10115",
      "addr:city": "Berlin",
      phone: "+49 30 1234567",
      website: "https://example.com",
      opening_hours: "Mo-Fr 09:00-18:00",
      amenity: "cafe",
    },
    source: "osm",
    ...partial,
  };
}

describe("<PoiPane />", () => {
  it("renders a skeleton while loading", () => {
    render(<PoiPane poi={null} status="loading" />);
    expect(screen.getByRole("status")).toHaveAccessibleName(
      /loading place details/i,
    );
  });

  it("renders error state when status=error and no poi", () => {
    const onClose = vi.fn();
    render(<PoiPane poi={null} status="error" onClose={onClose} />);
    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(screen.getByText(/could not load/i)).toBeInTheDocument();
  });

  it("renders a loaded POI with header, address, and hours", () => {
    render(<PoiPane poi={makePoi()} now={FIXED_MON_10AM} />);
    expect(
      screen.getByRole("heading", { level: 2, name: /cafe lume/i }),
    ).toBeInTheDocument();
    // Address lines.
    expect(screen.getByText(/12 Hauptstraße/)).toBeInTheDocument();
    expect(screen.getByText(/10115 Berlin/)).toBeInTheDocument();
    // Hours status line renders the "Open now" prefix.
    expect(screen.getByText(/open now/i)).toBeInTheDocument();
  });

  it("renders without hours when the tag is missing", () => {
    const poi = makePoi({
      tags: {
        "addr:street": "Main",
        "addr:housenumber": "1",
      },
    });
    render(<PoiPane poi={poi} />);
    expect(screen.queryByLabelText(/opening hours/i)).not.toBeInTheDocument();
  });

  it("directions button dispatches with the POI", async () => {
    const user = userEvent.setup();
    const onDirections = vi.fn();
    render(<PoiPane poi={makePoi()} onDirections={onDirections} />);

    await user.click(
      screen.getByRole("button", { name: /directions to cafe lume/i }),
    );
    expect(onDirections).toHaveBeenCalledTimes(1);
    expect(onDirections.mock.calls[0][0].id).toBe("p1");
  });

  it("close button fires onClose", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(<PoiPane poi={makePoi()} onClose={onClose} />);

    await user.click(screen.getByRole("button", { name: /close place details/i }));
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});

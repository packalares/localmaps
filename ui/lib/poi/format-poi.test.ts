import { describe, expect, it } from "vitest";
import { Coffee, MapPin, Utensils, Store, Hospital } from "lucide-react";
import {
  addressLines,
  humaniseCategory,
  iconForPoi,
  openingHoursTag,
  phoneOf,
  primaryText,
  secondaryText,
  websiteOf,
} from "./format-poi";

describe("iconForPoi", () => {
  it("falls back to MapPin when nothing matches", () => {
    expect(iconForPoi({ category: "mystery", tags: {} })).toBe(MapPin);
  });

  it("picks Coffee for category=cafe", () => {
    expect(iconForPoi({ category: "cafe", tags: {} })).toBe(Coffee);
  });

  it("picks Utensils for amenity=restaurant via tags", () => {
    expect(iconForPoi({ category: null, tags: { amenity: "restaurant" } })).toBe(
      Utensils,
    );
  });

  it("picks Store for shop=convenience", () => {
    expect(iconForPoi({ category: null, tags: { shop: "convenience" } })).toBe(
      Store,
    );
  });

  it("picks Hospital for amenity=pharmacy", () => {
    expect(iconForPoi({ category: null, tags: { amenity: "pharmacy" } })).toBe(
      Hospital,
    );
  });
});

describe("primaryText / secondaryText", () => {
  it("uses label when present", () => {
    expect(primaryText({ label: "Blue Moon", category: null })).toBe("Blue Moon");
  });

  it("falls back to humanised category when label empty", () => {
    expect(primaryText({ label: "", category: "ice_cream_shop" })).toBe(
      "Ice Cream Shop",
    );
  });

  it("uses category for secondary", () => {
    expect(secondaryText({ category: "cafe", tags: {} })).toBe("Cafe");
  });

  it("falls back to tags for secondary", () => {
    expect(secondaryText({ category: null, tags: { amenity: "bar" } })).toBe(
      "Bar",
    );
  });

  it("returns empty string when nothing available", () => {
    expect(secondaryText({ category: null, tags: {} })).toBe("");
  });
});

describe("humaniseCategory", () => {
  it("replaces underscores and title-cases", () => {
    expect(humaniseCategory("fast_food")).toBe("Fast Food");
    expect(humaniseCategory("ice-cream")).toBe("Ice Cream");
    expect(humaniseCategory("sit_down.restaurant")).toBe("Sit Down Restaurant");
  });
});

describe("addressLines", () => {
  it("builds street + city + country lines", () => {
    const lines = addressLines({
      "addr:housenumber": "10",
      "addr:street": "Baker St",
      "addr:postcode": "NW1",
      "addr:city": "London",
      "addr:country": "UK",
    });
    expect(lines).toEqual(["10 Baker St", "NW1 London", "UK"]);
  });

  it("returns empty when nothing is addressable", () => {
    expect(addressLines({})).toEqual([]);
  });

  it("tolerates bare keys without addr: prefix", () => {
    const lines = addressLines({
      housenumber: "3",
      street: "Main",
    });
    expect(lines).toEqual(["3 Main"]);
  });
});

describe("tag pickers", () => {
  it("extracts phone/website/opening_hours", () => {
    const tags = {
      phone: "+44 20 1234 5678",
      website: "https://example.com",
      opening_hours: "Mo-Fr 09:00-17:00",
    };
    expect(phoneOf(tags)).toBe("+44 20 1234 5678");
    expect(websiteOf(tags)).toBe("https://example.com");
    expect(openingHoursTag(tags)).toBe("Mo-Fr 09:00-17:00");
  });

  it("uses contact: prefix as fallback", () => {
    expect(phoneOf({ "contact:phone": "+1" })).toBe("+1");
    expect(websiteOf({ "contact:website": "https://x.com" })).toBe(
      "https://x.com",
    );
  });
});

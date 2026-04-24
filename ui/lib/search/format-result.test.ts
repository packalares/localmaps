import { describe, expect, it } from "vitest";
import {
  Building,
  Home,
  Hospital,
  Landmark,
  MapPin,
  Road,
  School,
  Store,
  Utensils,
} from "lucide-react";
import {
  formatDistance,
  formatResult,
  haversineMeters,
  iconFor,
  splitLabel,
} from "./format-result";
import type { GeocodeResult } from "@/lib/api/schemas";

function resultFixture(partial: Partial<GeocodeResult> = {}): GeocodeResult {
  return {
    id: "pelias:whosonfirst:locality:101748129",
    label: "Bucharest, Bucharest, Romania",
    confidence: 0.9,
    center: { lat: 44.4325, lon: 26.1039 },
    ...partial,
  };
}

describe("splitLabel", () => {
  it("returns primary alone when there's no comma", () => {
    expect(splitLabel("Bucharest")).toEqual({
      primary: "Bucharest",
      secondary: "",
    });
  });

  it("splits at the first comma and trims both halves", () => {
    expect(splitLabel("Piata Unirii, Bucharest, Romania")).toEqual({
      primary: "Piata Unirii",
      secondary: "Bucharest, Romania",
    });
  });
});

describe("iconFor", () => {
  it("falls back to MapPin when category is missing", () => {
    expect(iconFor({ category: null })).toBe(MapPin);
    expect(iconFor({ category: undefined })).toBe(MapPin);
  });

  it("picks Home for addresses", () => {
    expect(iconFor({ category: "address" })).toBe(Home);
    expect(iconFor({ category: "building" })).toBe(Home);
  });

  it("picks Road for streets/highways", () => {
    expect(iconFor({ category: "street" })).toBe(Road);
    expect(iconFor({ category: "highway" })).toBe(Road);
  });

  it("picks Utensils for restaurants / food venues", () => {
    expect(iconFor({ category: "restaurant" })).toBe(Utensils);
    expect(iconFor({ category: "cafe" })).toBe(Utensils);
  });

  it("picks Store for shops", () => {
    expect(iconFor({ category: "shop" })).toBe(Store);
    expect(iconFor({ category: "mall" })).toBe(Store);
  });

  it("picks Hospital for medical", () => {
    expect(iconFor({ category: "hospital" })).toBe(Hospital);
    expect(iconFor({ category: "pharmacy" })).toBe(Hospital);
  });

  it("picks School for education", () => {
    expect(iconFor({ category: "school" })).toBe(School);
    expect(iconFor({ category: "library" })).toBe(School);
  });

  it("picks Landmark for tourism / monuments", () => {
    expect(iconFor({ category: "landmark" })).toBe(Landmark);
    expect(iconFor({ category: "museum" })).toBe(Landmark);
  });

  it("picks Building for generic venues / offices", () => {
    expect(iconFor({ category: "office" })).toBe(Building);
    expect(iconFor({ category: "commercial" })).toBe(Building);
  });
});

describe("haversineMeters", () => {
  it("returns 0 for identical points", () => {
    expect(haversineMeters({ lat: 1, lon: 1 }, { lat: 1, lon: 1 })).toBe(0);
  });

  it("returns ~111.2 km for 1° latitude difference", () => {
    const d = haversineMeters({ lat: 0, lon: 0 }, { lat: 1, lon: 0 });
    expect(d).toBeGreaterThan(111_000);
    expect(d).toBeLessThan(111_500);
  });

  it("approximates London→Paris (~343 km)", () => {
    const london = { lat: 51.5074, lon: -0.1278 };
    const paris = { lat: 48.8566, lon: 2.3522 };
    const d = haversineMeters(london, paris);
    expect(d).toBeGreaterThan(340_000);
    expect(d).toBeLessThan(346_000);
  });
});

describe("formatDistance", () => {
  it("uses metres below 1 km, rounded", () => {
    expect(formatDistance(0)).toBe("0 m");
    expect(formatDistance(123)).toBe("123 m");
    expect(formatDistance(999)).toBe("999 m");
  });

  it("uses km with 1dp between 1 and 10 km", () => {
    expect(formatDistance(1000)).toBe("1.0 km");
    expect(formatDistance(2499)).toBe("2.5 km");
    expect(formatDistance(9999)).toBe("10.0 km");
  });

  it("uses whole km beyond 10 km", () => {
    expect(formatDistance(10_500)).toBe("11 km");
    expect(formatDistance(343_000)).toBe("343 km");
  });

  it("returns empty string for invalid input", () => {
    expect(formatDistance(Number.NaN)).toBe("");
    expect(formatDistance(-1)).toBe("");
  });
});

describe("formatResult", () => {
  it("returns icon + primary + secondary without distance when origin missing", () => {
    const out = formatResult(resultFixture());
    expect(out.icon).toBe(MapPin);
    expect(out.primary).toBe("Bucharest");
    expect(out.secondary).toBe("Bucharest, Romania");
    expect(out.distanceMeters).toBeNull();
    expect(out.distanceLabel).toBe("");
  });

  it("computes distance when origin is provided", () => {
    const out = formatResult(resultFixture(), { lat: 44.4325, lon: 26.1039 });
    expect(out.distanceMeters).toBe(0);
    expect(out.distanceLabel).toBe("0 m");
  });

  it("uses icon + category together", () => {
    const out = formatResult(
      resultFixture({ category: "restaurant", label: "Caru' cu Bere, Bucharest" }),
    );
    expect(out.icon).toBe(Utensils);
    expect(out.primary).toBe("Caru' cu Bere");
  });
});

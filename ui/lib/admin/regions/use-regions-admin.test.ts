import { describe, expect, it } from "vitest";
import {
  buildAdminData,
  filterCatalog,
  flattenCatalog,
} from "./use-regions-admin";
import type {
  Region,
  RegionCatalogEntry,
  RegionsListResponse,
} from "@/lib/api/schemas";

const catalog: RegionCatalogEntry[] = [
  {
    name: "europe",
    displayName: "Europe",
    kind: "continent",
    sourceUrl: "https://download.geofabrik.de/europe-latest.osm.pbf",
    children: [
      {
        name: "europe/romania",
        displayName: "Romania",
        kind: "country",
        parent: "europe",
        sourceUrl:
          "https://download.geofabrik.de/europe/romania-latest.osm.pbf",
      },
      {
        name: "europe/germany",
        displayName: "Germany",
        kind: "country",
        parent: "europe",
        sourceUrl:
          "https://download.geofabrik.de/europe/germany-latest.osm.pbf",
        children: [
          {
            name: "europe/germany/berlin",
            displayName: "Berlin",
            kind: "subregion",
            parent: "europe/germany",
            sourceUrl:
              "https://download.geofabrik.de/europe/germany/berlin-latest.osm.pbf",
          },
        ],
      },
    ],
  },
];

describe("flattenCatalog", () => {
  it("walks depth-first with a depth hint", () => {
    const flat = flattenCatalog(catalog);
    expect(flat.map((f) => [f.entry.name, f.depth])).toEqual([
      ["europe", 0],
      ["europe/romania", 1],
      ["europe/germany", 1],
      ["europe/germany/berlin", 2],
    ]);
  });
});

describe("filterCatalog", () => {
  it("keeps ancestors of matching descendants", () => {
    const out = filterCatalog(catalog, "berlin");
    expect(out).toHaveLength(1);
    expect(out[0].name).toBe("europe");
    expect(out[0].children?.[0].name).toBe("europe/germany");
    expect(out[0].children?.[0].children?.[0].name).toBe(
      "europe/germany/berlin",
    );
  });

  it("returns input unchanged for empty query", () => {
    expect(filterCatalog(catalog, "   ")).toBe(catalog);
  });

  it("filters at every depth case-insensitively", () => {
    const out = filterCatalog(catalog, "ROMANIA");
    expect(out).toHaveLength(1);
    expect(out[0].children?.map((c) => c.name)).toEqual(["europe/romania"]);
  });

  it("returns empty when nothing matches", () => {
    expect(filterCatalog(catalog, "qqqqqqqq")).toEqual([]);
  });
});

describe("buildAdminData", () => {
  const r1: Region = {
    name: "europe/romania",
    displayName: "Romania",
    sourceUrl: "https://example/r.pbf",
    state: "ready",
  };
  const r2: Region = {
    name: "europe/france",
    displayName: "France",
    sourceUrl: "https://example/fr.pbf",
    state: "processing_tiles",
  };

  it("counts active regions and maps installed by name", () => {
    const resp: RegionsListResponse = { regions: [r1, r2] };
    const data = buildAdminData(resp, catalog, "2026-04-24T00:00:00Z");
    expect(data.activeCount).toBe(1);
    expect(data.installedByName.get("europe/romania")).toEqual(r1);
    expect(data.installedByName.get("europe/france")).toEqual(r2);
    expect(data.fetchedAt).toBe("2026-04-24T00:00:00Z");
  });

  it("handles undefined inputs gracefully", () => {
    const data = buildAdminData(undefined, undefined, undefined);
    expect(data.regions).toEqual([]);
    expect(data.catalog).toEqual([]);
    expect(data.activeCount).toBe(0);
  });
});

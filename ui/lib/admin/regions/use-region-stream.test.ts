import { describe, expect, it } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { applyEventToCache } from "./use-region-stream";
import type { Region, Job, WsEvent } from "@/lib/api/schemas";

const REGION: Region = {
  name: "europe-romania",
  displayName: "Romania",
  sourceUrl: "https://download.geofabrik.de/europe/romania-latest.osm.pbf",
  state: "processing_tiles",
};

const JOB: Job = {
  id: "j-1",
  kind: "build_pmtiles",
  region: "europe-romania",
  state: "running",
  progress: 0.5,
};

describe("applyEventToCache", () => {
  it("updates regions.byName on region events", () => {
    const qc = new QueryClient();
    qc.setQueryData(["regions", "byName", "europe-romania"], {
      ...REGION,
      state: "downloading",
    });
    applyEventToCache(
      { type: "region.progress", data: REGION } as WsEvent,
      qc,
    );
    expect(
      (qc.getQueryData(["regions", "byName", "europe-romania"]) as Region)
        .state,
    ).toBe("processing_tiles");
  });

  it("merges into an existing regions.list cache entry", () => {
    const qc = new QueryClient();
    qc.setQueryData(["regions", "list"], {
      regions: [
        { ...REGION, state: "downloading" } as Region,
        {
          name: "europe-germany",
          displayName: "Germany",
          sourceUrl: "https://example/germany.pbf",
          state: "ready",
        } as Region,
      ],
    });
    applyEventToCache(
      { type: "region.progress", data: REGION } as WsEvent,
      qc,
    );
    const list = qc.getQueryData(["regions", "list"]) as {
      regions: Region[];
    };
    expect(list.regions).toHaveLength(2);
    const ro = list.regions.find((r) => r.name === "europe-romania");
    expect(ro?.state).toBe("processing_tiles");
  });

  it("appends a region not already in the list", () => {
    const qc = new QueryClient();
    qc.setQueryData(["regions", "list"], { regions: [] });
    applyEventToCache(
      { type: "region.ready", data: REGION } as WsEvent,
      qc,
    );
    const list = qc.getQueryData(["regions", "list"]) as {
      regions: Region[];
    };
    expect(list.regions).toHaveLength(1);
    expect(list.regions[0].name).toBe("europe-romania");
  });

  it("updates jobs.byId on job events", () => {
    const qc = new QueryClient();
    applyEventToCache(
      { type: "job.progress", data: JOB } as WsEvent,
      qc,
    );
    expect(qc.getQueryData(["jobs", "byId", "j-1"])).toEqual(JOB);
  });
});

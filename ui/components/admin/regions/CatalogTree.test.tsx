import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Region, RegionCatalogEntry } from "@/lib/api/schemas";
import { CatalogTree } from "./CatalogTree";

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
        sourcePbfBytes: 400_000_000,
      },
      {
        name: "europe/germany",
        displayName: "Germany",
        kind: "country",
        parent: "europe",
        sourceUrl:
          "https://download.geofabrik.de/europe/germany-latest.osm.pbf",
      },
    ],
  },
  {
    name: "africa",
    displayName: "Africa",
    kind: "continent",
    sourceUrl: "https://download.geofabrik.de/africa-latest.osm.pbf",
  },
];

describe("<CatalogTree />", () => {
  it("collapses children by default; expand reveals countries", async () => {
    const user = userEvent.setup();
    render(
      <CatalogTree
        entries={catalog}
        installedByName={new Map()}
        onInstall={() => {}}
      />,
    );
    expect(screen.queryByRole("treeitem", { name: /romania/i })).toBeNull();
    const europe = screen.getByRole("treeitem", { name: /^europe$/i });
    expect(europe).toHaveAttribute("aria-expanded", "false");
    await user.click(
      screen.getAllByRole("button", { name: /expand/i })[0],
    );
    expect(
      await screen.findByRole("treeitem", { name: /romania/i }),
    ).toBeInTheDocument();
  });

  it("search filter keeps ancestors and auto-expands", async () => {
    const user = userEvent.setup();
    render(
      <CatalogTree
        entries={catalog}
        installedByName={new Map()}
        onInstall={() => {}}
      />,
    );
    await user.type(
      screen.getByLabelText(/filter catalogue/i),
      "roman",
    );
    expect(
      await screen.findByRole("treeitem", { name: /romania/i }),
    ).toBeInTheDocument();
    // Africa should be gone.
    expect(screen.queryByRole("treeitem", { name: /africa/i })).toBeNull();
  });

  it("fires onInstall when a leaf's Install button is clicked", async () => {
    const user = userEvent.setup();
    const onInstall = vi.fn();
    render(
      <CatalogTree
        entries={catalog}
        installedByName={new Map()}
        onInstall={onInstall}
      />,
    );
    await user.type(screen.getByLabelText(/filter catalogue/i), "romania");
    const btn = await screen.findByRole("button", { name: /install romania/i });
    await user.click(btn);
    expect(onInstall).toHaveBeenCalledOnce();
    expect(onInstall.mock.calls[0][0].name).toBe("europe/romania");
  });

  it("shows an installed badge instead of Install when the entry is already installed", async () => {
    const user = userEvent.setup();
    const installed: Region = {
      name: "europe/romania",
      displayName: "Romania",
      sourceUrl: "https://example/r.pbf",
      state: "ready",
    };
    render(
      <CatalogTree
        entries={catalog}
        installedByName={new Map([["europe/romania", installed]])}
        onInstall={() => {}}
      />,
    );
    await user.type(screen.getByLabelText(/filter catalogue/i), "romania");
    await screen.findByRole("treeitem", { name: /romania/i });
    // No install button; an "already ready" chip is there.
    expect(
      screen.queryByRole("button", { name: /install romania/i }),
    ).toBeNull();
    expect(screen.getByLabelText(/already ready/i)).toBeInTheDocument();
  });
});

import { describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TooltipProvider } from "@/components/ui/tooltip";
import type { Region } from "@/lib/api/schemas";
import { InstalledTable } from "./InstalledTable";

function wrap(ui: React.ReactElement) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={qc}>
      <TooltipProvider delayDuration={0}>{ui}</TooltipProvider>
    </QueryClientProvider>,
  );
}

const ready: Region = {
  name: "europe-romania",
  displayName: "Romania",
  sourceUrl: "https://download.geofabrik.de/europe/romania-latest.osm.pbf",
  state: "ready",
  diskBytes: 2 * 1024 * 1024 * 1024,
  schedule: "monthly",
};
const downloading: Region = {
  name: "europe-france",
  displayName: "France",
  sourceUrl: "https://download.geofabrik.de/europe/france-latest.osm.pbf",
  state: "downloading",
  activeJobId: "j-1",
};
const failed: Region = {
  name: "europe-germany",
  displayName: "Germany",
  sourceUrl: "https://download.geofabrik.de/europe/germany-latest.osm.pbf",
  state: "failed",
  lastError: "Pelias importer exited 1",
};

describe("<InstalledTable />", () => {
  it("renders the empty state when no regions are installed", () => {
    wrap(
      <InstalledTable
        regions={[]}
        pendingByName={{}}
        onUpdateNow={() => {}}
        onScheduleChange={() => {}}
        onDelete={() => {}}
      />,
    );
    expect(
      screen.getByText(/no regions installed yet/i),
    ).toBeInTheDocument();
  });

  it("renders one row per region with state chips", () => {
    wrap(
      <InstalledTable
        regions={[ready, downloading, failed]}
        pendingByName={{}}
        onUpdateNow={() => {}}
        onScheduleChange={() => {}}
        onDelete={() => {}}
      />,
    );
    expect(screen.getByText("Romania")).toBeInTheDocument();
    expect(screen.getByText("France")).toBeInTheDocument();
    expect(screen.getByText("Germany")).toBeInTheDocument();
    expect(screen.getAllByText(/ready/i).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/downloading/i).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/failed/i).length).toBeGreaterThan(0);
  });

  it("ready rows get an Update action; failed rows get Retry", () => {
    wrap(
      <InstalledTable
        regions={[ready, failed]}
        pendingByName={{}}
        onUpdateNow={() => {}}
        onScheduleChange={() => {}}
        onDelete={() => {}}
      />,
    );
    expect(
      screen.getByRole("button", { name: /update romania now/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /retry germany/i }),
    ).toBeInTheDocument();
  });

  it("in-progress rows do not render an Update / Retry button", () => {
    wrap(
      <InstalledTable
        regions={[downloading]}
        pendingByName={{}}
        onUpdateNow={() => {}}
        onScheduleChange={() => {}}
        onDelete={() => {}}
      />,
    );
    expect(
      screen.queryByRole("button", { name: /update france/i }),
    ).toBeNull();
    expect(screen.queryByRole("button", { name: /retry france/i })).toBeNull();
  });

  it("fires callbacks on click", async () => {
    const user = userEvent.setup();
    const onUpdateNow = vi.fn();
    const onDelete = vi.fn();
    wrap(
      <InstalledTable
        regions={[ready]}
        pendingByName={{}}
        onUpdateNow={onUpdateNow}
        onScheduleChange={() => {}}
        onDelete={onDelete}
      />,
    );
    await user.click(screen.getByRole("button", { name: /update romania now/i }));
    await user.click(screen.getByRole("button", { name: /delete romania/i }));
    expect(onUpdateNow).toHaveBeenCalledOnce();
    expect(onDelete).toHaveBeenCalledOnce();
  });

  it("disables the Update button when an action is pending", () => {
    wrap(
      <InstalledTable
        regions={[ready]}
        pendingByName={{ [ready.name]: "update" }}
        onUpdateNow={() => {}}
        onScheduleChange={() => {}}
        onDelete={() => {}}
      />,
    );
    expect(
      screen.getByRole("button", { name: /update romania now/i }),
    ).toBeDisabled();
  });

  describe("active routing region", () => {
    it("renders an Active pill on the active row", () => {
      wrap(
        <InstalledTable
          regions={[ready, failed]}
          pendingByName={{}}
          activeRegionName="europe-romania"
          onUpdateNow={() => {}}
          onScheduleChange={() => {}}
          onDelete={() => {}}
          onActivate={() => {}}
        />,
      );
      // The active row is decorated with a "Use for routing" omission and
      // a visible Active label / aria-label.
      expect(
        screen.getByLabelText("Active routing region"),
      ).toBeInTheDocument();
    });

    it("hides Use-for-routing on the already-active row", () => {
      wrap(
        <InstalledTable
          regions={[ready]}
          pendingByName={{}}
          activeRegionName="europe-romania"
          onUpdateNow={() => {}}
          onScheduleChange={() => {}}
          onDelete={() => {}}
          onActivate={() => {}}
        />,
      );
      expect(
        screen.queryByRole("button", { name: /use romania for routing/i }),
      ).toBeNull();
    });

    it("clicking Use-for-routing fires the onActivate callback", async () => {
      const user = userEvent.setup();
      const onActivate = vi.fn();
      // Two ready regions so one becomes activatable.
      const otherReady: Region = {
        ...ready,
        name: "europe-france",
        displayName: "France",
      };
      wrap(
        <InstalledTable
          regions={[ready, otherReady]}
          pendingByName={{}}
          activeRegionName="europe-romania"
          onUpdateNow={() => {}}
          onScheduleChange={() => {}}
          onDelete={() => {}}
          onActivate={onActivate}
        />,
      );
      await user.click(
        screen.getByRole("button", { name: /use france for routing/i }),
      );
      expect(onActivate).toHaveBeenCalledOnce();
      expect(onActivate.mock.calls[0]?.[0]?.name).toBe("europe-france");
    });

    it("disables Use-for-routing while pending", () => {
      const otherReady: Region = {
        ...ready,
        name: "europe-france",
        displayName: "France",
      };
      wrap(
        <InstalledTable
          regions={[ready, otherReady]}
          pendingByName={{ [otherReady.name]: "activate" }}
          activeRegionName="europe-romania"
          onUpdateNow={() => {}}
          onScheduleChange={() => {}}
          onDelete={() => {}}
          onActivate={() => {}}
        />,
      );
      expect(
        screen.getByRole("button", { name: /use france for routing/i }),
      ).toBeDisabled();
    });

    it("ready-but-non-active rows omit Use-for-routing when no onActivate is supplied", () => {
      wrap(
        <InstalledTable
          regions={[ready]}
          pendingByName={{}}
          onUpdateNow={() => {}}
          onScheduleChange={() => {}}
          onDelete={() => {}}
        />,
      );
      expect(
        screen.queryByRole("button", { name: /use romania for routing/i }),
      ).toBeNull();
    });
  });

  describe("mobile card layout", () => {
    it("renders a card list (not a table) when forceMobile is set", () => {
      wrap(
        <InstalledTable
          regions={[ready, failed]}
          pendingByName={{}}
          onUpdateNow={() => {}}
          onScheduleChange={() => {}}
          onDelete={() => {}}
          forceMobile
        />,
      );
      // Cards use <article aria-label="..."> instead of <table>.
      expect(
        screen.getByRole("list", { name: /installed regions/i }),
      ).toBeInTheDocument();
      expect(screen.queryByRole("table")).toBeNull();
      expect(screen.getByLabelText("Romania")).toBeInTheDocument();
      expect(screen.getByLabelText("Germany")).toBeInTheDocument();
    });

    it("mobile cards preserve action button accessibility", async () => {
      const user = userEvent.setup();
      const onDelete = vi.fn();
      wrap(
        <InstalledTable
          regions={[ready]}
          pendingByName={{}}
          onUpdateNow={() => {}}
          onScheduleChange={() => {}}
          onDelete={onDelete}
          forceMobile
        />,
      );
      await user.click(screen.getByRole("button", { name: /delete romania/i }));
      expect(onDelete).toHaveBeenCalledOnce();
    });
  });
});

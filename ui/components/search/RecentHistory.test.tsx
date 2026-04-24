import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RecentHistory,
  clearHistory,
  pushHistoryEntry,
} from "./RecentHistory";
import type { GeocodeResult } from "@/lib/api/schemas";

function fixture(id: string, label = "Bucharest, Romania"): GeocodeResult {
  return {
    id,
    label,
    confidence: 0.9,
    center: { lat: 44.4325, lon: 26.1039 },
  };
}

describe("<RecentHistory />", () => {
  beforeEach(() => {
    clearHistory();
  });

  afterEach(() => {
    clearHistory();
  });

  it("renders nothing when history is empty", () => {
    const { container } = render(<RecentHistory onSelect={() => {}} />);
    expect(container.firstChild).toBeNull();
  });

  it("renders entries pushed via pushHistoryEntry", () => {
    pushHistoryEntry(fixture("a", "Alpha, Test"));
    pushHistoryEntry(fixture("b", "Beta, Test"));
    render(<RecentHistory onSelect={() => {}} />);
    expect(screen.getByText("Alpha")).toBeInTheDocument();
    expect(screen.getByText("Beta")).toBeInTheDocument();
  });

  it("deduplicates by id and brings the repeat to the top", () => {
    pushHistoryEntry(fixture("a", "Alpha, Test"));
    pushHistoryEntry(fixture("b", "Beta, Test"));
    pushHistoryEntry(fixture("a", "Alpha, Test"));
    render(<RecentHistory onSelect={() => {}} />);
    const primaries = screen.getAllByRole("option").map((el) => el.textContent);
    expect(primaries[0]).toContain("Alpha");
    expect(primaries[1]).toContain("Beta");
  });

  it("caps entries at maxEntries", () => {
    for (let i = 0; i < 15; i++) {
      pushHistoryEntry(fixture(`i${i}`, `Item ${i}, Test`));
    }
    render(<RecentHistory onSelect={() => {}} maxEntries={5} />);
    expect(screen.getAllByRole("option")).toHaveLength(5);
  });

  it("fires onSelect with the clicked entry", async () => {
    const user = userEvent.setup();
    pushHistoryEntry(fixture("a", "Alpha, Test"));
    const onSelect = vi.fn();
    render(<RecentHistory onSelect={onSelect} />);
    await user.click(screen.getByRole("option"));
    expect(onSelect).toHaveBeenCalledWith(
      expect.objectContaining({ id: "a" }),
    );
  });

  it("Clear button empties the history", async () => {
    const user = userEvent.setup();
    pushHistoryEntry(fixture("a", "Alpha, Test"));
    render(<RecentHistory onSelect={() => {}} />);
    await user.click(screen.getByRole("button", { name: /clear search history/i }));
    expect(screen.queryByRole("option")).not.toBeInTheDocument();
  });
});

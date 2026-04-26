import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { SearchBar } from "./SearchBar";
import { useMapStore } from "@/lib/state/map";

function renderSearchBar() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <SearchBar embedPanel={false} />
    </QueryClientProvider>,
  );
}

describe("<SearchBar />", () => {
  beforeEach(() => {
    // Reset the store between tests.
    useMapStore.getState().clear();
    useMapStore.setState((s) => ({ ...s, leftRailTab: "saved" }));
  });

  afterEach(() => {
    useMapStore.getState().clear();
    vi.restoreAllMocks();
  });

  it("renders an accessible combobox + search input", () => {
    renderSearchBar();
    expect(screen.getByRole("combobox")).toBeInTheDocument();
    expect(screen.getByRole("searchbox", { name: /search maps/i })).toBeInTheDocument();
  });

  it("'/' shortcut focuses the input from anywhere", async () => {
    const user = userEvent.setup();
    renderSearchBar();
    // Start with the document body focused.
    await user.keyboard("/");
    expect(document.activeElement).toBe(
      screen.getByRole("searchbox", { name: /search maps/i }),
    );
  });

  it("Cmd/Ctrl-K also focuses the input and opens the search tab", async () => {
    const user = userEvent.setup();
    renderSearchBar();
    await user.keyboard("{Control>}k{/Control}");
    expect(document.activeElement).toBe(
      screen.getByRole("searchbox", { name: /search maps/i }),
    );
    expect(useMapStore.getState().leftRailTab).toBe("search");
  });

  it("focusing the input opens the left rail `search` tab", async () => {
    const user = userEvent.setup();
    renderSearchBar();
    const input = screen.getByRole("searchbox", { name: /search maps/i });
    await user.click(input);
    expect(useMapStore.getState().leftRailTab).toBe("search");
  });

  it("shows the clear button when there is a query", async () => {
    const user = userEvent.setup();
    renderSearchBar();
    expect(
      screen.queryByRole("button", { name: /clear search/i }),
    ).not.toBeInTheDocument();
    const input = screen.getByRole("searchbox", { name: /search maps/i });
    await user.type(input, "buch");
    expect(
      screen.getByRole("button", { name: /clear search/i }),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /clear search/i }));
    expect(
      screen.queryByRole("button", { name: /clear search/i }),
    ).not.toBeInTheDocument();
  });

  it("Escape clears the query and blurs", async () => {
    const user = userEvent.setup();
    renderSearchBar();
    const input = screen.getByRole("searchbox", { name: /search maps/i }) as HTMLInputElement;
    await user.click(input);
    await user.type(input, "bucharest");
    expect(input.value).toBe("bucharest");
    await user.keyboard("{Escape}");
    expect(input.value).toBe("");
    expect(document.activeElement).not.toBe(input);
  });

  it("displays the chip label when a chip is active (Change 3)", () => {
    useMapStore.getState().setActiveCategoryChip("food");
    renderSearchBar();
    const input = screen.getByRole("searchbox", {
      name: /search maps/i,
    }) as HTMLInputElement;
    expect(input.value).toBe("Food & drink");
  });

  it("clear-X full-resets when a chip is active (Change 3 + closing)", async () => {
    const user = userEvent.setup();
    useMapStore.getState().runCategorySearch("food", null);
    renderSearchBar();
    expect(useMapStore.getState().activeCategoryChip).toBe("food");
    await user.click(
      screen.getByRole("button", { name: /clear search/i }),
    );
    expect(useMapStore.getState().activeCategoryChip).toBeNull();
    expect(useMapStore.getState().categorySearchResults).toEqual([]);
  });

  it("typing a full chip name only auto-activates after Enter (F12)", async () => {
    const user = userEvent.setup();
    renderSearchBar();
    const input = screen.getByRole("searchbox", { name: /search maps/i });
    await user.click(input);
    await user.type(input, "Hotels");
    // No auto-activation while typing — that path was racing the user
    // mid-type and triggering the chip search before the user finished
    // a phrase like "Hotels in Munich".
    expect(useMapStore.getState().activeCategoryChip).toBeNull();
    await user.keyboard("{Enter}");
    expect(useMapStore.getState().activeCategoryChip).toBe("lodging");
  });

  it("typing a full chip name + blur also activates (F12 fallback)", async () => {
    const user = userEvent.setup();
    renderSearchBar();
    const input = screen.getByRole("searchbox", { name: /search maps/i });
    await user.click(input);
    await user.type(input, "Hotels");
    expect(useMapStore.getState().activeCategoryChip).toBeNull();
    await user.tab();
    expect(useMapStore.getState().activeCategoryChip).toBe("lodging");
  });

  it("typing while a chip is active clears the chip first (Change 4)", async () => {
    const user = userEvent.setup();
    useMapStore.getState().setActiveCategoryChip("food");
    renderSearchBar();
    const input = screen.getByRole("searchbox", {
      name: /search maps/i,
    }) as HTMLInputElement;
    await user.click(input);
    // Typing a single character clears the chip first; the visible
    // value drops the old label and only shows the new char.
    await user.type(input, "a");
    expect(useMapStore.getState().activeCategoryChip).toBeNull();
    expect(input.value).toBe("a");
  });
});

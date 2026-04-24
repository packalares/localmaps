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
    useMapStore.setState((s) => ({ ...s, leftRailTab: "saved" }));
  });

  afterEach(() => {
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
});

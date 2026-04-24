import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ShareButton } from "./ShareButton";

function renderBtn() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <ShareButton />
    </QueryClientProvider>,
  );
}

describe("<ShareButton />", () => {
  it("renders a trigger button with an accessible name", () => {
    renderBtn();
    expect(
      screen.getByRole("button", { name: /share this view/i }),
    ).toBeInTheDocument();
  });

  it("opens the dialog on click", async () => {
    const user = userEvent.setup();
    renderBtn();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /share this view/i }));
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
    // The dialog title is the accessible name Radix wires up.
    expect(
      screen.getByRole("heading", { name: /share this view/i }),
    ).toBeInTheDocument();
  });
});

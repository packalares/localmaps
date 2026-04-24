import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

import LoginPage from "./page";

// Mock next/navigation: only the hooks the component needs.
const replace = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace }),
  useSearchParams: () => new URLSearchParams(""),
}));

function renderWithQuery(ui: React.ReactNode) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>);
}

afterEach(() => {
  vi.restoreAllMocks();
  replace.mockReset();
});

describe("<LoginPage />", () => {
  it("submits credentials and redirects on success", async () => {
    // First call = /api/auth/me (anon), second = /api/auth/login (ok).
    const fetchMock = vi
      .fn()
      .mockImplementationOnce(
        async () => new Response("{}", { status: 401 }),
      )
      .mockImplementationOnce(
        async () =>
          new Response(
            JSON.stringify({
              user: { id: 1, username: "admin", role: "admin" },
            }),
            { status: 200, headers: { "Content-Type": "application/json" } },
          ),
      );
    global.fetch = fetchMock;

    renderWithQuery(<LoginPage />);
    await userEvent.type(screen.getByLabelText(/Username/i), "admin");
    await userEvent.type(screen.getByLabelText(/Password/i), "passphrase-11");
    await userEvent.click(screen.getByRole("button", { name: /Sign in/i }));

    await waitFor(() => expect(replace).toHaveBeenCalledWith("/"));
  });

  it("renders an error message when login fails", async () => {
    const fetchMock = vi
      .fn()
      .mockImplementationOnce(async () => new Response("{}", { status: 401 }))
      .mockImplementationOnce(
        async () =>
          new Response(
            JSON.stringify({
              error: {
                code: "UNAUTHORIZED",
                message: "invalid credentials",
                retryable: false,
              },
              traceId: "t",
            }),
            { status: 401, headers: { "Content-Type": "application/json" } },
          ),
      );
    global.fetch = fetchMock;

    renderWithQuery(<LoginPage />);
    await userEvent.type(screen.getByLabelText(/Username/i), "nope");
    await userEvent.type(screen.getByLabelText(/Password/i), "wrong-password1");
    await userEvent.click(screen.getByRole("button", { name: /Sign in/i }));

    await waitFor(() =>
      expect(screen.getByRole("alert")).toHaveTextContent(
        /invalid credentials/i,
      ),
    );
    expect(replace).not.toHaveBeenCalled();
  });
});

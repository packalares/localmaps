import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ShareDialog } from "./ShareDialog";

function renderDialog(open = true) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <ShareDialog open={open} onOpenChange={() => {}} qrSize={128} />
    </QueryClientProvider>,
  );
}

describe("<ShareDialog />", () => {
  const originalFetch = globalThis.fetch;
  const originalClipboard = navigator.clipboard;

  beforeEach(() => {
    // Place the jsdom window at a known URL so the dialog has a stable
    // `window.location.href` to read from.
    window.history.replaceState(null, "", "/#12/45/25");
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText: vi.fn(() => Promise.resolve()) },
    });
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: originalClipboard,
    });
    vi.restoreAllMocks();
  });

  it("renders three tabs and opens on the Link tab by default", () => {
    renderDialog();
    const tabs = screen.getAllByRole("tab");
    expect(tabs).toHaveLength(3);
    expect(tabs[0]).toHaveAccessibleName(/link/i);
    expect(tabs[0]).toHaveAttribute("aria-selected", "true");
    expect(
      screen.getByRole("button", { name: /make short link/i }),
    ).toBeInTheDocument();
  });

  it("switches to the QR tab and renders an SVG QR for the current URL", async () => {
    const user = userEvent.setup();
    renderDialog();
    await user.click(screen.getByRole("tab", { name: /qr/i }));

    // Radix Dialog renders into a portal, so query the whole document
    // rather than the render container.
    const img = screen.getByRole("img", { name: /qr code/i });
    expect(img).toBeInTheDocument();
    expect(img.querySelector("svg")).not.toBeNull();
  });

  it("switches to the Embed tab and shows a snippet containing /embed", async () => {
    const user = userEvent.setup();
    renderDialog();
    await user.click(screen.getByRole("tab", { name: /embed/i }));
    const ta = screen.getByLabelText(/paste this into your site/i) as HTMLTextAreaElement;
    expect(ta.value).toContain("/embed");
    expect(ta.value).toMatch(/<iframe /);
  });

  it("posts to /api/links and reveals the short URL on success", async () => {
    const fetchMock = vi.fn(async () =>
      new Response(
        JSON.stringify({
          code: "ABC1234",
          url: "/#12/45/25",
          createdAt: "2026-04-24T10:00:00Z",
          hitCount: 0,
        }),
        { status: 201, headers: { "Content-Type": "application/json" } },
      ),
    );
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    const user = userEvent.setup();
    renderDialog();
    await user.click(screen.getByRole("button", { name: /make short link/i }));

    await waitFor(() => {
      expect(screen.getByLabelText("Short link")).toBeInTheDocument();
    });
    const shortInput = screen.getByLabelText("Short link") as HTMLInputElement;
    expect(shortInput.value).toMatch(/\/api\/links\/ABC1234$/);

    // Verify the fetch call used POST /api/links with a relative URL.
    expect(fetchMock).toHaveBeenCalledOnce();
    const call = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect(String(call[0])).toContain("/api/links");
    expect(call[1].method).toBe("POST");
    expect(JSON.parse(String(call[1].body))).toEqual({ url: "/#12/45/25" });
  });
});

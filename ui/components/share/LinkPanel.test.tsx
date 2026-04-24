import { describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { LinkPanel } from "./LinkPanel";

// @testing-library/user-event v14 installs its own clipboard stub
// during setup() (see attachClipboardStubToView in setup.js). To spy
// on writeText we must call setup() first, THEN install our own shim
// — otherwise userEvent overwrites it before the test runs.
function setupClipboard() {
  const user = userEvent.setup();
  const writeText = vi.fn(() => Promise.resolve());
  const original = navigator.clipboard;
  Object.defineProperty(navigator, "clipboard", {
    configurable: true,
    value: { writeText },
  });
  return {
    user,
    writeText,
    restore: () =>
      Object.defineProperty(navigator, "clipboard", {
        configurable: true,
        value: original,
      }),
  };
}

describe("<LinkPanel />", () => {
  it("shows the long URL, lets it be copied, and flips the label to 'Copied!'", async () => {
    const clip = setupClipboard();
    try {
      render(
        <LinkPanel
          longUrl="http://localhost:8080/#12/45/25"
          shortUrl={null}
          onCreateShort={() => {}}
          creating={false}
          errored={false}
        />,
      );
      expect(
        (screen.getByLabelText("Link") as HTMLInputElement).value,
      ).toBe("http://localhost:8080/#12/45/25");

      await clip.user.click(
        screen.getByRole("button", { name: /copy link/i }),
      );
      expect(clip.writeText).toHaveBeenCalledWith(
        "http://localhost:8080/#12/45/25",
      );
      await waitFor(() =>
        expect(
          screen.getByRole("button", { name: /link copied to clipboard/i }),
        ).toBeInTheDocument(),
      );
    } finally {
      clip.restore();
    }
  });

  it("renders 'Make short link' when no short URL yet and invokes the callback", async () => {
    const user = userEvent.setup();
    const onCreateShort = vi.fn();
    render(
      <LinkPanel
        longUrl="http://localhost:8080/#1/0/0"
        shortUrl={null}
        onCreateShort={onCreateShort}
        creating={false}
        errored={false}
      />,
    );
    await user.click(screen.getByRole("button", { name: /make short link/i }));
    expect(onCreateShort).toHaveBeenCalledOnce();
  });

  it("shows the short URL with its own copy button once available", () => {
    render(
      <LinkPanel
        longUrl="http://localhost:8080/#1/0/0"
        shortUrl="http://localhost:8080/api/links/ABC1234"
        onCreateShort={() => {}}
        creating={false}
        errored={false}
      />,
    );
    // Exact-string label avoids matching "Link" as a substring of
    // "Short link" (which the /short link/i regex would also do).
    expect(
      (screen.getByLabelText("Short link") as HTMLInputElement).value,
    ).toBe("http://localhost:8080/api/links/ABC1234");
    expect(
      screen.getByRole("button", { name: /copy short link/i }),
    ).toBeInTheDocument();
  });

  it("surfaces an error alert when errored=true", () => {
    render(
      <LinkPanel
        longUrl="http://localhost:8080/#1/0/0"
        shortUrl={null}
        onCreateShort={() => {}}
        creating={false}
        errored={true}
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent(/couldn.?t create/i);
  });
});

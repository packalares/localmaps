import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EmbedSnippet, buildEmbedSnippet } from "./EmbedSnippet";

describe("buildEmbedSnippet", () => {
  it("produces a single-line iframe with the expected attributes", () => {
    const s = buildEmbedSnippet("http://localhost:8080/embed?lat=45&lon=25");
    expect(s).toMatch(/^<iframe /);
    expect(s).toContain('src="http://localhost:8080/embed?lat=45&lon=25"');
    expect(s).toContain('width="600"');
    expect(s).toContain('height="450"');
    expect(s).toContain('frameborder="0"');
    expect(s.split("\n")).toHaveLength(1);
  });

  it("threads through custom width/height", () => {
    const s = buildEmbedSnippet("/embed?a=1", 320, 240);
    expect(s).toContain('width="320"');
    expect(s).toContain('height="240"');
  });
});

describe("<EmbedSnippet />", () => {
  const originalClipboard = navigator.clipboard;

  afterEach(() => {
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: originalClipboard,
    });
    vi.restoreAllMocks();
  });

  it("renders the snippet in a readonly textarea and copies it on click", async () => {
    // userEvent.setup() installs its own clipboard stub; call it FIRST
    // then overwrite `navigator.clipboard` so our spy is the one the
    // component's copyText() actually invokes.
    const user = userEvent.setup();
    const writeText = vi.fn(() => Promise.resolve());
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });

    render(<EmbedSnippet src="http://localhost:8080/embed?x=1" />);

    const ta = screen.getByLabelText(/paste this into your site/i);
    expect(ta).toHaveAttribute("readOnly");
    expect((ta as HTMLTextAreaElement).value).toContain(
      'src="http://localhost:8080/embed?x=1"',
    );

    await user.click(screen.getByRole("button", { name: /copy snippet/i }));
    expect(writeText).toHaveBeenCalledWith(
      buildEmbedSnippet("http://localhost:8080/embed?x=1"),
    );
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: /copied to clipboard/i }),
      ).toBeInTheDocument(),
    );
  });
});

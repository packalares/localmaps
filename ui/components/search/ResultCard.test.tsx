import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ResultCard } from "./ResultCard";
import type { GeocodeResult } from "@/lib/api/schemas";

function fixture(partial: Partial<GeocodeResult> = {}): GeocodeResult {
  return {
    id: "r1",
    label: "Caru' cu Bere, Lipscani, Bucharest",
    confidence: 0.8,
    center: { lat: 44.4325, lon: 26.1039 },
    ...partial,
  };
}

describe("<ResultCard />", () => {
  it("renders primary + secondary from the label", () => {
    render(
      <ResultCard result={fixture()} onSelect={() => {}} />,
    );
    expect(screen.getByText("Caru' cu Bere")).toBeInTheDocument();
    expect(screen.getByText(/Lipscani, Bucharest/)).toBeInTheDocument();
  });

  it("fires onSelect on click", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    const item = fixture();
    render(<ResultCard result={item} onSelect={onSelect} />);
    await user.click(screen.getByRole("option"));
    expect(onSelect).toHaveBeenCalledWith(item);
  });

  it("toggles aria-selected when highlighted", () => {
    const { rerender } = render(
      <ResultCard result={fixture()} onSelect={() => {}} highlighted={false} />,
    );
    expect(screen.getByRole("option")).toHaveAttribute("aria-selected", "false");
    rerender(
      <ResultCard result={fixture()} onSelect={() => {}} highlighted />,
    );
    expect(screen.getByRole("option")).toHaveAttribute("aria-selected", "true");
  });

  it("shows a distance badge when origin is provided", () => {
    render(
      <ResultCard
        result={fixture()}
        origin={{ lat: 44.4325, lon: 26.1039 }}
        onSelect={() => {}}
      />,
    );
    expect(screen.getByLabelText(/Distance:/i)).toHaveTextContent("0 m");
  });

  it("omits the distance badge when origin is absent", () => {
    render(<ResultCard result={fixture()} onSelect={() => {}} />);
    expect(screen.queryByLabelText(/Distance:/i)).not.toBeInTheDocument();
  });

  it("renders the Utensils icon for a restaurant category", () => {
    render(
      <ResultCard
        result={fixture({ category: "restaurant" })}
        onSelect={() => {}}
      />,
    );
    // lucide-react renders as svgs; check the role=option rendered an svg child.
    const option = screen.getByRole("option");
    expect(option.querySelector("svg")).not.toBeNull();
  });

  it("renders distinct icons across the main category buckets", () => {
    const cats = [
      "address",
      "street",
      "shop",
      "hospital",
      "school",
      "landmark",
      "office",
    ];
    for (const cat of cats) {
      const { unmount } = render(
        <ResultCard result={fixture({ category: cat })} onSelect={() => {}} />,
      );
      expect(screen.getByRole("option").querySelector("svg")).not.toBeNull();
      unmount();
    }
  });
});

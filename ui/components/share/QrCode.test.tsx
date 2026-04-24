import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { QrCode } from "./QrCode";

describe("<QrCode />", () => {
  it("renders an SVG QR image for a non-empty value", () => {
    const { container } = render(
      <QrCode value="http://localhost:8080/api/links/ABCDE12" />,
    );
    expect(screen.getByRole("img", { name: /qr code/i })).toBeInTheDocument();
    // qrcode.react v3's QRCodeSVG renders <svg> with child <path> nodes.
    const svg = container.querySelector("svg");
    expect(svg).not.toBeNull();
    expect(svg!.querySelectorAll("path").length).toBeGreaterThan(0);
  });

  it("renders a placeholder when value is empty", () => {
    render(<QrCode value="" />);
    expect(screen.getByTestId("qr-empty")).toBeInTheDocument();
    expect(
      screen.getByRole("img", { name: /qr code placeholder/i }),
    ).toBeInTheDocument();
  });

  it("honours a custom size prop", () => {
    const { container } = render(<QrCode value="http://x/y" size={128} />);
    const svg = container.querySelector("svg");
    expect(svg).not.toBeNull();
    expect(svg!.getAttribute("height")).toBe("128");
    expect(svg!.getAttribute("width")).toBe("128");
  });
});

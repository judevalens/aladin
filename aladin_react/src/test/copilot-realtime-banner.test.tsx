import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { RealtimeStatusBanner } from "@/modules/copilot/ui/copilot-banners";

describe("RealtimeStatusBanner", () => {
  it("stays hidden when the realtime stream is open", () => {
    const { container } = render(<RealtimeStatusBanner state="open" />);
    expect(container).toBeEmptyDOMElement();
  });

  it("shows a quiet reconnecting state", () => {
    render(<RealtimeStatusBanner state="connecting" />);
    expect(screen.getByText("reconnecting stream…")).toBeTruthy();
  });

  it("shows an offline reconnecting state", () => {
    render(<RealtimeStatusBanner state="closed" />);
    expect(screen.getByText("stream offline — reconnecting")).toBeTruthy();
  });
});

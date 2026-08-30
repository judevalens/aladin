import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { RESEARCH_DEFAULTS, RESEARCH_PROPS, ResearchObjectView, SAMPLE_OBJECTS } from "@/modules/board/spike/research-object";

describe("research board design spike", () => {
  it("uses valid custom-shape props for every sample object and new note", () => {
    for (const props of [RESEARCH_DEFAULTS, ...SAMPLE_OBJECTS.map((object) => object.props)]) {
      for (const [key, validator] of Object.entries(RESEARCH_PROPS)) {
        expect(() => validator.validate(props[key as keyof typeof props])).not.toThrow();
      }
    }
  });

  it("provides heterogeneous objects with unique identities", () => {
    expect(new Set(SAMPLE_OBJECTS.map((item) => item.id)).size).toBe(SAMPLE_OBJECTS.length);
    expect(new Set(SAMPLE_OBJECTS.map((item) => item.props.kind)).size).toBe(7);
  });

  it("labels the instrument as illustrative rather than live market data", () => {
    const instrument = SAMPLE_OBJECTS.find((item) => item.props.kind === "instrument")!;
    render(<ResearchObjectView props={instrument.props} />);
    expect(screen.getByText("Illustrative data")).toBeVisible();
    expect(screen.getByRole("img")).toHaveAccessibleName("Illustrative spread fluctuating around its mean");
  });

  it("renders a personal note as an editable-content preview without permanent action chrome", () => {
    render(<ResearchObjectView props={{ ...RESEARCH_DEFAULTS, title: "My working idea", body: "Keep investigating." }} />);
    expect(screen.getByRole("heading", { name: "My working idea" })).toBeVisible();
    expect(screen.getByText("Keep investigating.")).toBeVisible();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });
});

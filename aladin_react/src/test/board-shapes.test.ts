import { describe, expect, it } from "vitest";

import {
  CARD_DEFAULTS,
  DOC_WINDOW_DEFAULTS,
  EXCERPT_DEFAULTS,
  TASK_DEFAULTS,
  cardProps,
  docWindowProps,
  excerptProps,
  taskProps,
} from "@/modules/board/shapes/shape-types";
import {
  BOARD_WEIGHTS,
  PENCIL_HINTS,
  boardToolFromTldraw,
  tldrawToolId,
} from "@/modules/board/domain/board-tools";

type Validator = { validate(value: unknown): unknown };

function validateAll(props: Record<string, Validator>, values: Record<string, unknown>) {
  for (const [key, validator] of Object.entries(props)) {
    validator.validate(values[key]);
  }
}

describe("board shape props", () => {
  it("accepts each shape's defaults", () => {
    validateAll(docWindowProps as never, DOC_WINDOW_DEFAULTS as never);
    validateAll(excerptProps as never, EXCERPT_DEFAULTS as never);
    validateAll(taskProps as never, TASK_DEFAULTS as never);
    validateAll(cardProps as never, CARD_DEFAULTS as never);
  });

  it("rejects wrong-typed values", () => {
    expect(() => (docWindowProps.page as never as Validator).validate("94")).toThrow();
    expect(() => (taskProps.checked as never as Validator).validate("yes")).toThrow();
    expect(() => (excerptProps.text as never as Validator).validate(null)).toThrow();
  });

  it("excerpt source fields are nullable — a paste has no source yet", () => {
    (excerptProps.sourceArtifactId as never as Validator).validate(null);
    (excerptProps.page as never as Validator).validate(null);
  });
});

describe("board tool mapping", () => {
  it("maps dock tools onto tldraw tool ids", () => {
    expect(tldrawToolId("select", "pen")).toBe("select");
    expect(tldrawToolId("arrow", "pen")).toBe("arrow");
    expect(tldrawToolId("pencil", "pen")).toBe("draw");
    expect(tldrawToolId("pencil", "highlighter")).toBe("highlight");
    expect(tldrawToolId("pencil", "eraser")).toBe("eraser");
    expect(tldrawToolId("pencil", "lasso")).toBe("lasso");
  });

  it("derives dock state from any tldraw tool id — round-trips", () => {
    for (const tool of ["select", "arrow"] as const) {
      expect(boardToolFromTldraw(tldrawToolId(tool, "pen")).tool).toBe(tool);
    }
    for (const subTool of ["pen", "highlighter", "eraser", "lasso"] as const) {
      const derived = boardToolFromTldraw(tldrawToolId("pencil", subTool));
      expect(derived).toEqual({ tool: "pencil", subTool });
    }
    // Unknown tools (e.g. tldraw internals) read as select, never crash the dock.
    expect(boardToolFromTldraw("zoom").tool).toBe("select");
  });

  it("has a hint and a weight for everything the dock offers", () => {
    expect(Object.keys(PENCIL_HINTS).sort()).toEqual(["eraser", "highlighter", "lasso", "pen"]);
    expect(BOARD_WEIGHTS.map((w) => w.size)).toEqual(["s", "m", "l"]);
  });
});

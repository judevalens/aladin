// @vitest-environment jsdom
import { describe, expect, it } from "vitest";

import { selectionExcerpt, type SelectionLike } from "@/modules/documents/domain/selection-excerpt";

function dom() {
  const root = document.createElement("div");
  const slot = document.createElement("div");
  slot.dataset.pdfPage = "94";
  const span = document.createElement("span");
  span.textContent = "the payoff is bounded";
  slot.appendChild(span);
  root.appendChild(slot);
  document.body.appendChild(root);
  return { root, slot, span };
}

function fakeSelection(text: string, node: Node): SelectionLike {
  return {
    isCollapsed: text.length === 0,
    rangeCount: 1,
    toString: () => text,
    getRangeAt: () => ({
      commonAncestorContainer: node,
      getBoundingClientRect: () => ({ left: 10, top: 20, right: 110, bottom: 40 }),
    }),
  };
}

describe("selectionExcerpt", () => {
  it("returns text + page for a selection inside a rendered page", () => {
    const { root, span } = dom();
    const result = selectionExcerpt(fakeSelection("the payoff is bounded", span.firstChild!), root);
    expect(result?.text).toBe("the payoff is bounded");
    expect(result?.page).toBe(94);
  });

  it("rejects collapsed, whitespace, out-of-root and cross-page selections", () => {
    const { root, span } = dom();
    expect(selectionExcerpt(null, root)).toBeNull();
    expect(selectionExcerpt(fakeSelection("", span), root)).toBeNull();
    expect(selectionExcerpt(fakeSelection("   ", span), root)).toBeNull();
    const outside = document.createElement("p");
    document.body.appendChild(outside);
    expect(selectionExcerpt(fakeSelection("text", outside), root)).toBeNull();
    // Common ancestor above the page slots = a cross-page selection.
    expect(selectionExcerpt(fakeSelection("spans pages", root), root)).toBeNull();
  });
});

import { describe, expect, it } from "vitest";
import type { TLEditorSnapshot } from "tldraw";

import { hasSessionCamera } from "@/modules/board/ui/board-pane";

function snapshot(session: unknown): TLEditorSnapshot {
  return { document: { store: {}, schema: {} }, session } as unknown as TLEditorSnapshot;
}

describe("hasSessionCamera", () => {
  it("is true when the saved session carries a page camera — the board reopens where it was left", () => {
    expect(
      hasSessionCamera(
        snapshot({ pageStates: [{ pageId: "page:a", camera: { x: 10, y: 20, z: 0.8 } }] }),
      ),
    ).toBe(true);
  });

  it("is false without one, so the pane frames the content instead", () => {
    expect(hasSessionCamera(snapshot({ pageStates: [] }))).toBe(false);
    expect(hasSessionCamera(snapshot({}))).toBe(false);
    expect(hasSessionCamera(snapshot(undefined))).toBe(false);
  });
});

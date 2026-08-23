import { Box } from "tldraw";
import { describe, expect, it } from "vitest";

import { findFreeRect } from "@/modules/board/domain/board-objects";

describe("findFreeRect — the handoff's 'cascade near free space'", () => {
  it("takes the start point when nothing is there", () => {
    expect(findFreeRect([], { x: 100, y: 100 }, 300, 120)).toEqual({ x: 100, y: 100 });
  });

  it("steps aside when the start overlaps an object, keeping a gap", () => {
    const occupied = [new Box(100, 100, 300, 120)];
    const p = findFreeRect(occupied, { x: 100, y: 100 }, 300, 120);
    const placed = new Box(p.x - 16, p.y - 16, 332, 152);
    expect(placed.collides(occupied[0])).toBe(false);
    // Near, not across the board: within a few spiral rings.
    expect(Math.hypot(p.x - 100, p.y - 100)).toBeLessThan(400);
  });

  it("never stacks on the same slot: successive inserts all land clear of each other", () => {
    const occupied: Box[] = [];
    for (let i = 0; i < 8; i++) {
      const p = findFreeRect(occupied, { x: 0, y: 0 }, 200, 100);
      const box = new Box(p.x, p.y, 200, 100);
      for (const other of occupied) expect(box.collides(other)).toBe(false);
      occupied.push(box);
    }
  });

  it("falls back to the start when the board is packed that far out", () => {
    // A huge occupied plate around the start — nothing within the search radius is free.
    const occupied = [new Box(-5000, -5000, 10000, 10000)];
    expect(findFreeRect(occupied, { x: 0, y: 0 }, 200, 100)).toEqual({ x: 0, y: 0 });
  });
});

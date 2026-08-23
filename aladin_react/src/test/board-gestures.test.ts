import { describe, expect, it, vi } from "vitest";

import { createTapTracker } from "@/modules/board/domain/board-gestures";

describe("multi-finger tap tracker", () => {
  it("two fingers down and up quickly is a two-finger tap", () => {
    const onTap = vi.fn();
    const t = createTapTracker({ onTap });
    t.start(1, 100, 100, 0);
    t.start(2, 160, 100, 10);
    t.end(1, 120);
    t.end(2, 130);
    expect(onTap).toHaveBeenCalledWith(2);
  });

  it("three fingers is a three-finger tap", () => {
    const onTap = vi.fn();
    const t = createTapTracker({ onTap });
    t.start(1, 100, 100, 0);
    t.start(2, 160, 100, 5);
    t.start(3, 220, 100, 10);
    t.end(3, 100);
    t.end(2, 110);
    t.end(1, 120);
    expect(onTap).toHaveBeenCalledWith(3);
  });

  it("one finger is never a tap here (that is a normal tap for the plane)", () => {
    const onTap = vi.fn();
    const t = createTapTracker({ onTap });
    t.start(1, 100, 100, 0);
    t.end(1, 50);
    expect(onTap).not.toHaveBeenCalled();
  });

  it("movement cancels — three-finger swipes belong to the system", () => {
    const onTap = vi.fn();
    const t = createTapTracker({ onTap });
    t.start(1, 100, 100, 0);
    t.start(2, 160, 100, 5);
    t.start(3, 220, 100, 10);
    t.move(2, 200, 100);
    t.end(1, 100);
    t.end(2, 100);
    t.end(3, 100);
    expect(onTap).not.toHaveBeenCalled();
  });

  it("a hold is not a tap", () => {
    const onTap = vi.fn();
    const t = createTapTracker({ onTap, maxDurationMs: 250 });
    t.start(1, 100, 100, 0);
    t.start(2, 160, 100, 5);
    t.end(1, 400);
    t.end(2, 410);
    expect(onTap).not.toHaveBeenCalled();
  });

  it("a fourth finger spoils it, and the next gesture starts clean", () => {
    const onTap = vi.fn();
    const t = createTapTracker({ onTap });
    for (let i = 1; i <= 4; i++) t.start(i, i * 50, 100, i);
    for (let i = 1; i <= 4; i++) t.end(i, 100);
    expect(onTap).not.toHaveBeenCalled();
    t.start(1, 100, 100, 1000);
    t.start(2, 160, 100, 1005);
    t.end(1, 1100);
    t.end(2, 1100);
    expect(onTap).toHaveBeenCalledWith(2);
  });
});

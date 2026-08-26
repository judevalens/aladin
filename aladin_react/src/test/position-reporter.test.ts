/**
 * The reading-position writer's gate + debounce. The scenario that matters: the
 * reader's IntersectionObserver reports "page 1" at mount, BEFORE the synced
 * position loads or the restore jump lands — none of that may reach the server.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { createPositionReporter } from "@/modules/documents/domain/position-reporter";

describe("createPositionReporter", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("sends nothing before arm", () => {
    const send = vi.fn();
    const r = createPositionReporter(send, 2000);
    r.notePage(1);
    vi.advanceTimersByTime(10_000);
    r.flush();
    expect(send).not.toHaveBeenCalled();
  });

  it("never sends the restored page, and the restore jump cancels the mount tick", () => {
    const send = vi.fn();
    const r = createPositionReporter(send, 2000);
    r.arm(87);
    r.notePage(1); // observer's mount tick — schedules
    r.notePage(87); // the restore jump lands — cancels (back at known state)
    vi.advanceTimersByTime(10_000);
    expect(send).not.toHaveBeenCalled();
  });

  it("debounces real page changes to the last value", () => {
    const send = vi.fn();
    const r = createPositionReporter(send, 2000);
    r.arm(null);
    r.notePage(2);
    vi.advanceTimersByTime(500);
    r.notePage(3);
    vi.advanceTimersByTime(500);
    r.notePage(4);
    vi.advanceTimersByTime(2000);
    expect(send).toHaveBeenCalledTimes(1);
    expect(send).toHaveBeenCalledWith(4);
    // A later change from the new known page sends again.
    r.notePage(5);
    vi.advanceTimersByTime(2000);
    expect(send).toHaveBeenLastCalledWith(5);
  });

  it("flush sends the pending page immediately (close/hide)", () => {
    const send = vi.fn();
    const r = createPositionReporter(send, 2000);
    r.arm(1);
    r.notePage(9);
    r.flush();
    expect(send).toHaveBeenCalledWith(9);
    // Nothing pending → flush is a no-op, and the sent page is now "known".
    r.flush();
    r.notePage(9);
    vi.advanceTimersByTime(5000);
    expect(send).toHaveBeenCalledTimes(1);
  });
});

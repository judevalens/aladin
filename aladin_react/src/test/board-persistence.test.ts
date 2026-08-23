import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { createBoardSaver, type BoardSaveState } from "@/modules/board/domain/board-persistence";

function setup(opts: { failTimes?: number } = {}) {
  let failures = opts.failTimes ?? 0;
  const calls: number[] = [];
  const states: BoardSaveState[] = [];
  const save = vi.fn(async () => {
    calls.push(Date.now());
    if (failures > 0) {
      failures -= 1;
      throw new Error("boom");
    }
  });
  const saver = createBoardSaver({
    save,
    onState: (s) => states.push(s),
    debounceMs: 700,
    retryBaseMs: 1000,
    retryCapMs: 30_000,
  });
  return { saver, save, calls, states };
}

describe("board saver", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("never saves before the load armed it — a failed load cannot overwrite the server", async () => {
    const { saver, save } = setup();
    saver.markDirty();
    await vi.advanceTimersByTimeAsync(5000);
    expect(save).not.toHaveBeenCalled();
    expect(saver.dirty).toBe(false);
  });

  it("coalesces edits behind the debounce into one save", async () => {
    const { saver, save, states } = setup();
    saver.arm();
    saver.markDirty();
    await vi.advanceTimersByTimeAsync(300);
    saver.markDirty();
    await vi.advanceTimersByTimeAsync(300);
    expect(save).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(700);
    expect(save).toHaveBeenCalledTimes(1);
    expect(states).toEqual(["dirty", "dirty", "saving", "saved"]);
  });

  it("keeps a failed save dirty and retries on a 1s·2s·4s ladder until it lands", async () => {
    const { saver, save, calls, states } = setup({ failTimes: 3 });
    saver.arm();
    saver.markDirty();
    await vi.advanceTimersByTimeAsync(700); // attempt 1 fails
    expect(saver.dirty).toBe(true);
    expect(saver.state).toBe("error");
    await vi.advanceTimersByTimeAsync(1000); // retry after 1s fails
    await vi.advanceTimersByTimeAsync(2000); // retry after 2s fails
    await vi.advanceTimersByTimeAsync(4000); // retry after 4s succeeds
    expect(save).toHaveBeenCalledTimes(4);
    expect(calls[1] - calls[0]).toBe(1000);
    expect(calls[2] - calls[1]).toBe(2000);
    expect(calls[3] - calls[2]).toBe(4000);
    expect(saver.state).toBe("saved");
    expect(saver.dirty).toBe(false);
    expect(states.at(-1)).toBe("saved");
  });

  it("does not let an edit reset a pending backoff; the retry carries the newest edit", async () => {
    const { saver, save } = setup({ failTimes: 1 });
    saver.arm();
    saver.markDirty();
    await vi.advanceTimersByTimeAsync(700); // fails, retry armed for +1000
    saver.markDirty(); // edit while waiting
    await vi.advanceTimersByTimeAsync(700); // a debounce would fire here — must not
    expect(save).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(300); // the retry fires at +1000
    expect(save).toHaveBeenCalledTimes(2);
    expect(saver.state).toBe("saved");
  });

  it("saves again at once when an edit landed during an in-flight save", async () => {
    let release!: () => void;
    const save = vi.fn(
      () =>
        new Promise<void>((resolve) => {
          release = resolve;
        }),
    );
    const saver = createBoardSaver({ save, debounceMs: 700 });
    saver.arm();
    saver.markDirty();
    await vi.advanceTimersByTimeAsync(700);
    expect(save).toHaveBeenCalledTimes(1);
    saver.markDirty(); // while the first PATCH is in flight
    release();
    await vi.advanceTimersByTimeAsync(0);
    expect(save).toHaveBeenCalledTimes(2);
  });

  it("flush saves immediately when dirty (pane hidden / unmount)", async () => {
    const { saver, save } = setup();
    saver.arm();
    saver.markDirty();
    saver.flush();
    await vi.advanceTimersByTimeAsync(0);
    expect(save).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(2000);
    expect(save).toHaveBeenCalledTimes(1); // the debounce was cancelled, not doubled
  });

  it("dispose stops every timer and callback", async () => {
    const { saver, save, states } = setup({ failTimes: 5 });
    saver.arm();
    saver.markDirty();
    await vi.advanceTimersByTimeAsync(700);
    saver.dispose();
    const seen = states.length;
    await vi.advanceTimersByTimeAsync(60_000);
    expect(save).toHaveBeenCalledTimes(1);
    expect(states.length).toBe(seen);
  });
});

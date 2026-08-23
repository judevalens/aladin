import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { createToastStore } from "@/modules/board/domain/board-toasts";

describe("board toast store", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("shows one toast at a time, newest wins, and auto-dismisses", () => {
    const store = createToastStore();
    const seen: (string | null)[] = [];
    store.subscribe(() => seen.push(store.get()?.text ?? null));
    store.show({ text: "first" }, 1000);
    store.show({ text: "second" }, 1000);
    expect(store.get()?.text).toBe("second");
    vi.advanceTimersByTime(999);
    expect(store.get()?.text).toBe("second");
    vi.advanceTimersByTime(1);
    expect(store.get()).toBeNull();
    expect(seen).toEqual(["first", "second", null]);
  });

  it("dismiss by id ignores a stale id", () => {
    const store = createToastStore();
    const a = store.show({ text: "a" });
    store.show({ text: "b" });
    store.dismiss(a);
    expect(store.get()?.text).toBe("b");
    store.dismiss();
    expect(store.get()).toBeNull();
  });
});

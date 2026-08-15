import { describe, expect, it, vi } from "vitest";

import { KeyedStream } from "@/shared/flow/keyed-stream";
import type { Result } from "@/shared/flow/result";

interface Row {
  id: string;
  title: string;
}

function stream(
  fetch: (id: string) => Promise<Row> = async (id) => ({ id, title: id }),
  retainedKeys?: number,
) {
  return new KeyedStream<string, Row>((row) => row.id, fetch, retainedKeys);
}

/** Collects emissions, and lets a test read what a subscriber saw SYNCHRONOUSLY on subscribe. */
function collect(observable: ReturnType<KeyedStream<string, Row>["observe"]>) {
  const seen: Result<Row>[] = [];
  const subscription = observable.subscribe((next) => seen.push(next));
  return { seen, stop: () => subscription.unsubscribe() };
}

const tick = () => new Promise((resolve) => setTimeout(resolve, 0));

describe("KeyedStream", () => {
  it("fetches once per key and shares the value with later subscribers", async () => {
    const fetch = vi.fn(async (id: string) => ({ id, title: id }));
    const rows = stream(fetch);

    const first = collect(rows.observe("a"));
    const second = collect(rows.observe("a"));
    await tick();

    expect(fetch).toHaveBeenCalledTimes(1);
    expect(first.seen).toEqual([{ ok: true, value: { id: "a", title: "a" } }]);
    expect(second.seen).toEqual([{ ok: true, value: { id: "a", title: "a" } }]);
    first.stop();
    second.stop();
  });

  // The bug this class exists to prevent: a teardown-then-resubscribe inside one React commit
  // used to leave the consumer with nothing until a fresh promise landed, which is a frame of
  // "artifact missing" — enough to unmount a PDF pane and make it reload.
  it("replays the current value synchronously when resubscribed", async () => {
    const fetch = vi.fn(async (id: string) => ({ id, title: id }));
    const rows = stream(fetch);

    const first = collect(rows.observe("a"));
    await tick();
    first.stop();

    const again = collect(rows.observe("a"));
    expect(again.seen).toEqual([{ ok: true, value: { id: "a", title: "a" } }]);
    expect(fetch).toHaveBeenCalledTimes(1);
    again.stop();
  });

  it("pushes updates to everyone observing the key", async () => {
    const rows = stream();
    const watcher = collect(rows.observe("a"));
    await tick();

    rows.push({ id: "a", title: "renamed" });
    rows.push({ id: "b", title: "other key" });

    expect(watcher.seen).toEqual([
      { ok: true, value: { id: "a", title: "a" } },
      { ok: true, value: { id: "a", title: "renamed" } },
    ]);
    watcher.stop();
  });

  it("serves a pushed value without fetching it", async () => {
    const fetch = vi.fn(async (id: string) => ({ id, title: id }));
    const rows = stream(fetch);

    rows.push({ id: "a", title: "from the syncer" });
    const watcher = collect(rows.observe("a"));
    await tick();

    expect(watcher.seen).toEqual([{ ok: true, value: { id: "a", title: "from the syncer" } }]);
    expect(fetch).not.toHaveBeenCalled();
    watcher.stop();
  });

  it("reports a failed fetch and lets the next subscriber retry", async () => {
    const fetch = vi
      .fn<(id: string) => Promise<Row>>()
      .mockRejectedValueOnce(new Error("offline"))
      .mockResolvedValueOnce({ id: "a", title: "a" });
    const rows = stream(fetch);

    const first = collect(rows.observe("a"));
    await tick();
    expect(first.seen).toEqual([{ ok: false, error: new Error("offline") }]);
    first.stop();

    const second = collect(rows.observe("a"));
    await tick();
    expect(second.seen.at(-1)).toEqual({ ok: true, value: { id: "a", title: "a" } });
    expect(fetch).toHaveBeenCalledTimes(2);
    second.stop();
  });

  it("evicts unobserved keys past the cap, oldest first", async () => {
    const fetch = vi.fn(async (id: string) => ({ id, title: id }));
    const rows = stream(fetch, 2);

    for (const id of ["a", "b", "c"]) {
      const watcher = collect(rows.observe(id));
      await tick();
      watcher.stop();
    }
    expect(fetch).toHaveBeenCalledTimes(3);

    // "a" was pushed out by "c"; "b" and "c" are still held.
    collect(rows.observe("b")).stop();
    collect(rows.observe("c")).stop();
    expect(fetch).toHaveBeenCalledTimes(3);

    collect(rows.observe("a")).stop();
    expect(fetch).toHaveBeenCalledTimes(4);
  });

  it("never evicts a key that still has a subscriber", async () => {
    const fetch = vi.fn(async (id: string) => ({ id, title: id }));
    const rows = stream(fetch, 1);

    const held = collect(rows.observe("a"));
    await tick();
    for (const id of ["b", "c", "d"]) {
      const watcher = collect(rows.observe(id));
      await tick();
      watcher.stop();
    }

    // Still live, and still receiving updates rather than a completed stream.
    rows.push({ id: "a", title: "renamed" });
    expect(held.seen.at(-1)).toEqual({ ok: true, value: { id: "a", title: "renamed" } });
    held.stop();
  });
});

import { describe, expect, it, vi } from "vitest";
import type { ApiClient } from "@/shared/api/client";
import { createContentTokenStore } from "@/shared/runtime/content-token-store";

const START = Date.parse("2026-08-26T19:44:50-07:00");
const TTL = 12 * 60 * 60 * 1000;

function setup() {
  let time = START;
  const fetch = vi.fn<ApiClient["fetch"]>().mockImplementation(async () => ({
    token: `content-${fetch.mock.calls.length}`,
    expiresAt: new Date(time + TTL).toISOString(),
  }));
  const client: ApiClient = { fetch: fetch as ApiClient["fetch"], fetchBlob: vi.fn(), resolveUrl: (path) => path };
  const store = createContentTokenStore(client, () => time);
  return { store, fetch, advance: (ms: number) => { time += ms; } };
}

describe("content token cache", () => {
  it("mints once and shares a fresh credential between shards", async () => {
    const { store, fetch } = setup();
    expect(store.peek()).toBeNull();
    const first = store.get();
    const second = store.get();
    expect(second).toBe(first);
    expect(await first).toBe("content-1");
    expect(await store.get()).toBe("content-1");
    expect(store.peek()).toBe("content-1");
    expect(fetch).toHaveBeenCalledExactlyOnceWith("/api/auth/content-token", { method: "POST" });
  });

  it("does not hand yesterday's expired credential to a newly opened shard", async () => {
    const { store, fetch, advance } = setup();
    await store.get();
    advance(14 * 60 * 60 * 1000);
    expect(store.peek()).toBeNull();
    expect(await store.get()).toBe("content-2");
    expect(fetch).toHaveBeenCalledTimes(2);
  });

  it("refreshes at 80% of the lifetime, including short-lived tokens", async () => {
    const { store, fetch, advance } = setup();
    fetch.mockResolvedValueOnce({ token: "short", expiresAt: new Date(START + 10_000).toISOString() });
    expect(await store.get()).toBe("short");
    advance(7_999);
    expect(store.peek()).toBe("short");
    advance(1);
    expect(store.peek()).toBeNull();
    expect(await store.get()).toBe("content-2");
  });

  it.each([
    { token: "bad", expiresAt: "invalid" },
    { token: "old", expiresAt: new Date(START - 1).toISOString() },
    { token: "expired", expiresAt: new Date(START).toISOString() },
    { token: "", expiresAt: new Date(START + TTL).toISOString() },
    { token: " ", expiresAt: new Date(START + TTL).toISOString() },
  ])("rejects unusable mint responses: %j", async (response) => {
    const { store, fetch } = setup();
    fetch.mockResolvedValueOnce(response);
    expect(await store.get()).toBeNull();
    expect(store.peek()).toBeNull();
    expect(await store.get()).toBe("content-2");
  });

  it("never falls back to a stale credential after a refresh failure, and allows retry", async () => {
    const { store, fetch, advance } = setup();
    await store.get();
    advance(TTL);
    fetch.mockRejectedValueOnce(new Error("offline"));
    expect(await store.get()).toBeNull();
    expect(store.peek()).toBeNull();
    expect(await store.get()).toBe("content-3");
  });

  it("releases the in-flight request even if the client throws synchronously", async () => {
    const { store, fetch } = setup();
    fetch.mockImplementationOnce(() => { throw new Error("offline"); });
    expect(await store.get()).toBeNull();
    expect(await store.get()).toBe("content-2");
  });
});

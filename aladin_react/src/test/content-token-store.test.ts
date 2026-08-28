import { describe, expect, it, vi } from "vitest";
import type { ApiClient } from "@/shared/api/client";
import { createContentTokenStore } from "@/shared/runtime/content-token-store";

const START = Date.parse("2026-08-26T19:44:50-07:00");
const TTL = 30 * 24 * 60 * 60 * 1000;

function setup() {
  let time = START;
  let sessionToken: string | null = "session-1";
  const fetch = vi.fn<ApiClient["fetch"]>().mockImplementation(async () => ({
    token: `content-${fetch.mock.calls.length}`,
    expiresAt: new Date(time + TTL).toISOString(),
  }));
  const client: ApiClient = { fetch: fetch as ApiClient["fetch"], fetchBlob: vi.fn(), resolveUrl: (path) => path };
  const store = createContentTokenStore(client, { getToken: () => sessionToken }, () => time);
  return {
    store, fetch,
    advance: (ms: number) => { time += ms; },
    setSession: (token: string | null) => { sessionToken = token; },
  };
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

  it("keeps the same token past 12 hours and 80% of the login lifetime", async () => {
    const { store, fetch, advance } = setup();
    await store.get();
    advance(14 * 60 * 60 * 1000);
    expect(await store.get()).toBe("content-1");
    advance(TTL * 0.8);
    expect(await store.get()).toBe("content-1");
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  it("does not reuse an expired credential", async () => {
    const { store, fetch, advance } = setup();
    await store.get();
    advance(TTL);
    expect(store.peek()).toBeNull();
    expect(await store.get()).toBe("content-2");
    expect(fetch).toHaveBeenCalledTimes(2);
  });

  it("respects the exact reported expiry, even near the end of a session", async () => {
    const { store, fetch, advance } = setup();
    fetch.mockResolvedValueOnce({ token: "short", expiresAt: new Date(START + 10_000).toISOString() });
    expect(await store.get()).toBe("short");
    advance(9_999);
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

  it("clears credentials on logout and mints anew even for a same-user login", async () => {
    const { store, fetch, setSession } = setup();
    await store.get();
    setSession(null);
    expect(store.peek()).toBeNull();
    expect(await store.get()).toBeNull();
    expect(fetch).toHaveBeenCalledTimes(1);
    setSession("session-2");
    expect(store.peek()).toBeNull();
    expect(await store.get()).toBe("content-2");
  });

  it("isolates concurrent token requests across a session change", async () => {
    const { store, fetch, setSession } = setup();
    let resolveOld!: (response: unknown) => void;
    fetch.mockReturnValueOnce(new Promise((resolve) => { resolveOld = resolve; }));
    const oldRequest = store.get();
    setSession("session-2");
    expect(await store.get()).toBe("content-2");
    resolveOld({ token: "old-session-content", expiresAt: new Date(START + TTL).toISOString() });
    expect(await oldRequest).toBeNull();
    expect(store.peek()).toBe("content-2");
    expect(await store.get()).toBe("content-2");
  });

  it("discards a token response that arrives after logout", async () => {
    const { store, setSession } = setup();
    const request = store.get();
    setSession(null);
    expect(await request).toBeNull();
    expect(store.peek()).toBeNull();
  });
});

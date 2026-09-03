import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError, createApiClient, type ApiRuntimeConfig } from "@/shared/api/client";
import { createLocalDesktopSessionStore } from "@/shared/runtime/desktop-session-store";

const NOW = Date.parse("2026-09-03T12:00:00Z");
const config: ApiRuntimeConfig = {
  isDesktopApp: true,
  apiBaseUrl: "http://localhost:8000",
  websocketBaseUrl: "ws://localhost:8000",
  collabWsBaseUrl: "ws://localhost:3500",
};

describe("desktop sessions", () => {
  beforeEach(() => {
    const values = new Map<string, string>();
    Object.defineProperty(window, "localStorage", {
      configurable: true,
      value: {
        getItem: (key: string) => values.get(key) ?? null,
        setItem: (key: string, value: string) => void values.set(key, String(value)),
        removeItem: (key: string) => void values.delete(key),
        clear: () => values.clear(),
      },
    });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("clears an expired bearer before it can be sent", () => {
    const store = createLocalDesktopSessionStore(() => NOW);
    const invalidated = vi.fn();
    store.onInvalidated(invalidated);
    store.save({ token: "expired-token", expiresAt: "2026-09-03T11:59:59Z" });

    expect(store.getToken()).toBeNull();
    expect(store.load()).toBeNull();
    expect(invalidated).toHaveBeenCalledTimes(1);
    expect(window.localStorage.getItem("aladin.desktop_session")).toBeNull();
  });

  it("keeps an unexpired bearer", () => {
    const store = createLocalDesktopSessionStore(() => NOW);
    store.save({ token: "valid-token", expiresAt: "2026-09-03T12:00:01Z" });

    expect(store.getToken()).toBe("valid-token");
  });

  it("clears a malformed session record", () => {
    const store = createLocalDesktopSessionStore(() => NOW);
    const invalidated = vi.fn();
    store.onInvalidated(invalidated);
    window.localStorage.setItem("aladin.desktop_session", "not-json");

    expect(store.getToken()).toBeNull();
    expect(invalidated).toHaveBeenCalledTimes(1);
    expect(window.localStorage.getItem("aladin.desktop_session")).toBeNull();
  });

  it("invalidates the shared session when the API rejects it", async () => {
    const store = createLocalDesktopSessionStore(() => NOW);
    const invalidated = vi.fn();
    store.onInvalidated(invalidated);
    store.save({ token: "rejected-token", expiresAt: "2026-09-04T12:00:00Z" });
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(
      JSON.stringify({ error: "Unauthenticated" }),
      { status: 401, statusText: "Unauthorized", headers: { "Content-Type": "application/json" } },
    )));

    const client = createApiClient(config, store);
    await expect(client.fetch("/api/market/subscribe", { method: "POST" }))
      .rejects.toBeInstanceOf(ApiError);

    expect(invalidated).toHaveBeenCalledTimes(1);
    expect(store.getToken()).toBeNull();
  });
});

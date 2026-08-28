import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { useShardContentToken } from "@/modules/doc-surface/hooks/use-shard-content-token";
import type { ContentTokenStore } from "@/shared/runtime/content-token-store";

function tokenStore() {
  return { get: vi.fn<ContentTokenStore["get"]>().mockResolvedValue("fresh"), peek: vi.fn().mockReturnValue("stale") };
}

describe("shard document credentials", () => {
  it("checks get rather than trusting peek on every page/build load", async () => {
    const store = tokenStore();
    const { result, rerender } = renderHook(({ key }) => useShardContentToken(store, key), { initialProps: { key: "page-1:build-1" } });
    expect(result.current.token).toBeNull();
    await waitFor(() => expect(result.current.token).toBe("fresh"));
    expect(store.peek).not.toHaveBeenCalled();
    rerender({ key: "page-1:build-1" });
    expect(store.get).toHaveBeenCalledTimes(1);
    store.get.mockResolvedValueOnce("new-build-token");
    rerender({ key: "page-1:build-2" });
    expect(result.current.token).toBeNull();
    await waitFor(() => expect(result.current.token).toBe("new-build-token"));
    rerender({ key: "page-2:build-2" });
    expect(result.current.token).toBeNull();
    await waitFor(() => expect(store.get).toHaveBeenCalledTimes(3));
  });

  it("ignores a late response from a previous store/load", async () => {
    const old = tokenStore();
    let resolveOld!: (token: string) => void;
    old.get.mockReturnValueOnce(new Promise((resolve) => { resolveOld = resolve; }));
    const fresh = tokenStore();
    const { result, rerender } = renderHook(({ store }) => useShardContentToken(store, "page-1"), { initialProps: { store: old } });
    rerender({ store: fresh });
    await waitFor(() => expect(result.current.token).toBe("fresh"));
    await act(async () => resolveOld("old-session-token"));
    expect(result.current.token).toBe("fresh");
  });

  it.each(["null", "reject"])("shows mint failure and retries without a request loop (%s)", async (failure) => {
    const store = tokenStore();
    if (failure === "null") store.get.mockResolvedValueOnce(null);
    else store.get.mockRejectedValueOnce(new Error("offline"));
    const { result } = renderHook(() => useShardContentToken(store, "page-1"));
    await waitFor(() => expect(result.current.error).toBe(true));
    expect(result.current.token).toBeNull();
    expect(store.get).toHaveBeenCalledTimes(1);
    act(() => result.current.retry());
    expect(result.current.error).toBe(false);
    await waitFor(() => expect(result.current.token).toBe("fresh"));
    expect(store.get).toHaveBeenCalledTimes(2);
  });

  it("waits until embed planes exist", async () => {
    const { result, rerender } = renderHook(({ store }) => useShardContentToken(store, "page-1"), { initialProps: { store: null as ContentTokenStore | null } });
    expect(result.current.token).toBeNull();
    expect(result.current.error).toBe(false);
    rerender({ store: tokenStore() });
    await waitFor(() => expect(result.current.token).toBe("fresh"));
  });
});

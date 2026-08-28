import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Artifact } from "@/shared/api/models";
import type { ApiClient } from "@/shared/api/client";
import { createContentTokenStore } from "@/shared/runtime/content-token-store";
import { DocSurfaceUI } from "@/modules/doc-surface/ui/doc-surface-ui";
import { useAppStore } from "@/app/state/store";
import { shardBuildFromWire } from "@/app/state/shard-build-slice";

vi.mock("@/app/composition/app-composition", () => ({ useAppComposition: () => ({ runtime }) }));
vi.mock("@/modules/doc-surface/bridge/bridge-host", () => ({
  createBridgeHost: () => ({ attach() {}, detach() {}, pushTheme() {} }),
}));

let now = Date.parse("2026-08-26T19:44:50-07:00");
const TTL = 12 * 60 * 60 * 1000;
const fetch = vi.fn<ApiClient["fetch"]>();
const client: ApiClient = { fetch: fetch as ApiClient["fetch"], fetchBlob: vi.fn(), resolveUrl: (path) => path };
const runtime = {
  config: { apiBaseUrl: "https://api.example.test" },
  desktopSession: { getToken: () => "session-secret-never-in-url" },
  contentTokens: createContentTokenStore(client, () => now),
  apis: { shards: { getBuildState: vi.fn().mockResolvedValue(null) } },
};
const artifact: Artifact = { id: "shard-1", title: "Research", kind: "app", content: "", updatedLabel: "Today" };
const frame = () => screen.getByTitle("Research") as HTMLIFrameElement;

beforeEach(() => {
  fetch.mockReset().mockImplementation(async () => ({
    token: `content-${fetch.mock.calls.length}`,
    expiresAt: new Date(now + TTL).toISOString(),
  }));
  runtime.contentTokens = createContentTokenStore(client, () => now);
  useAppStore.setState({ shardBuilds: {}, theme: "dark" });
});

describe("shard iframe loading", () => {
  it("refreshes yesterday's shared token before mounting an isolated iframe", async () => {
    await runtime.contentTokens.get();
    now += TTL + 1;
    render(<DocSurfaceUI artifact={artifact} />);
    expect(screen.getByRole("status")).toHaveTextContent("Opening shard");
    expect(screen.queryByTitle("Research")).toBeNull();
    await waitFor(() => expect(frame().src).toContain("access_token=content-2"));
    expect(frame().getAttribute("sandbox")).toBe("allow-scripts");
    expect(frame().src).not.toContain("session-secret");
    expect(new URL(frame().src).pathname).toBe("/content/shard-1/");
  });

  it("re-checks freshness for a rebuild but not a theme or visibility change", async () => {
    const { rerender } = render(<DocSurfaceUI artifact={artifact} />);
    await waitFor(() => expect(frame().src).toContain("access_token=content-1"));
    const original = frame();
    const src = original.src;
    now += TTL + 1;
    rerender(<DocSurfaceUI artifact={artifact} hidden />);
    act(() => useAppStore.setState({ theme: "light" }));
    rerender(<DocSurfaceUI artifact={artifact} />);
    expect(frame()).toBe(original);
    expect(frame().src).toBe(src);
    expect(fetch).toHaveBeenCalledTimes(1);
    act(() => useAppStore.getState().setShardBuild(shardBuildFromWire({
      page_id: artifact.id, channel: "draft", status: "ok", build_id: "build-2",
    })));
    await waitFor(() => expect(frame().src).toContain("access_token=content-2"));
    expect(new URL(frame().src).searchParams.get("v")).toBe("build-2");
    expect(new URL(frame().src).searchParams.get("channel")).toBe("draft");
  });

  it("shows a retry instead of a blank or unauthenticated iframe when minting fails", async () => {
    fetch.mockRejectedValueOnce(new Error("offline"));
    render(<DocSurfaceUI artifact={artifact} />);
    await screen.findByRole("alert");
    expect(screen.queryByTitle("Research")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(frame().src).toContain("access_token=content-2"));
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("mints a fresh token when a shard is closed and opened after expiry", async () => {
    const { unmount } = render(<DocSurfaceUI artifact={artifact} />);
    await waitFor(() => expect(frame().src).toContain("access_token=content-1"));
    unmount();
    now += TTL + 1;
    render(<DocSurfaceUI artifact={artifact} />);
    expect(screen.queryByTitle("Research")).toBeNull();
    await waitFor(() => expect(frame().src).toContain("access_token=content-2"));
  });
});

import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Artifact } from "@/shared/api/models";
import type { ApiClient } from "@/shared/api/client";
import { createContentTokenStore } from "@/shared/runtime/content-token-store";
import { DocSurfaceUI } from "@/modules/doc-surface/ui/doc-surface-ui";
import { useAppStore } from "@/app/state/store";
import { shardBuildFromWire } from "@/app/state/shard-build-slice";
import type { ShardChannel } from "@/app/state/shard-build-slice";
import type { ShardReleaseMetadata } from "@/modules/doc-surface/bridge/resource-host-hub";
import { createBridgeV2Host } from "@/modules/doc-surface/bridge/bridge-v2-host";
import { createShardBuildEventHandler } from "@/shared/realtime/shard-build-event-handler";

vi.mock("@/app/composition/app-composition", () => ({ useAppComposition: () => ({ runtime }) }));
vi.mock("@/modules/doc-surface/bridge/bridge-host", () => ({
  createBridgeHost: () => ({ attach() {}, detach() {}, pushTheme() {} }),
}));
vi.mock("@/modules/doc-surface/bridge/bridge-v2-host", () => ({ createBridgeV2Host: vi.fn() }));

let now = Date.parse("2026-08-26T19:44:50-07:00");
const TTL = 12 * 60 * 60 * 1000;
const fetch = vi.fn<ApiClient["fetch"]>();
const client: ApiClient = { fetch: fetch as ApiClient["fetch"], fetchBlob: vi.fn(), resolveUrl: (path) => path };
const desktopSession = { getToken: () => "session-secret-never-in-url" };
const runtime = {
  config: { apiBaseUrl: "https://api.example.test" },
  desktopSession,
  contentTokens: createContentTokenStore(client, desktopSession, () => now),
  apis: {
    shards: { getBuildState: vi.fn().mockResolvedValue(null) },
    shardResources: { release: vi.fn<(id: string, channel: ShardChannel) => Promise<ShardReleaseMetadata>>() },
  },
};
const artifact: Artifact = { id: "shard-1", title: "Research", kind: "app", content: "", updatedLabel: "Today" };
const frame = () => screen.getByTitle("Research") as HTMLIFrameElement;

beforeEach(() => {
  fetch.mockReset().mockImplementation(async () => ({
    token: `content-${fetch.mock.calls.length}`,
    expiresAt: new Date(now + TTL).toISOString(),
  }));
  runtime.contentTokens = createContentTokenStore(client, desktopSession, () => now);
  runtime.apis.shards.getBuildState.mockReset().mockResolvedValue(null);
  runtime.apis.shardResources.release.mockReset().mockImplementation(async (_id, channel) => ({ protocol: "bridge/2", buildId: `${channel}-1`, contractHash: `${channel}-hash` }));
  vi.mocked(createBridgeV2Host).mockReset().mockImplementation(() => ({ attach() {}, detach() {}, pushTheme() {} }));
  useAppStore.setState({ shardBuilds: {}, shardPublications: {}, theme: "dark" });
});

function draftBuild(status: "ok" | "building" | "failed", buildId = "draft-2") {
  act(() => useAppStore.getState().setShardBuild(shardBuildFromWire({
    page_id: artifact.id, channel: "draft", status, build_id: buildId, errors: status === "failed" ? "broken draft" : "",
  })));
}

function publish(buildId = "published-2") {
  act(() => createShardBuildEventHandler()({
    eventId: `publish-${buildId}`, type: "artifact.published", occurredAt: "2026-08-31T00:00:00Z",
    subscriptionKey: { stream: "workspace", resourceKind: "artifact", resourceId: artifact.id },
    payload: { page_id: artifact.id, protocol: "bridge/2", buildId, contractHash: `${buildId}-hash` },
  }));
}

describe("shard iframe loading", () => {
  it("puts the preview control in the existing toolbar only for the active pane", async () => {
    render(<div data-testid="existing-toolbar" />);
    const toolbar = screen.getByTestId("existing-toolbar");
    const { container, rerender } = render(<DocSurfaceUI artifact={artifact} controlsTarget={toolbar} />);
    await waitFor(() => expect(frame()).toBeInTheDocument());
    const previewButton = within(toolbar).getByRole("button", { name: "Preview draft" });
    expect(previewButton).toHaveAttribute("aria-pressed", "false");
    expect(container.querySelector("button")).toBeNull();
    fireEvent.click(previewButton);
    await waitFor(() => expect(new URL(frame().src).searchParams.get("channel")).toBe("draft"));
    const draftFrame = frame();
    expect(within(toolbar).getByRole("button", { name: "Back to published" })).toHaveAttribute("aria-pressed", "true");
    rerender(<DocSurfaceUI artifact={artifact} controlsTarget={toolbar} hidden />);
    expect(within(toolbar).queryByRole("button")).toBeNull();
    expect(frame()).toBe(draftFrame);
    rerender(<DocSurfaceUI artifact={artifact} controlsTarget={toolbar} />);
    expect(within(toolbar).getByRole("button", { name: "Back to published" })).toHaveAttribute("aria-pressed", "true");
    expect(frame()).toBe(draftFrame);
  });

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

  it("ignores draft rebuilds, theme and visibility changes during normal use", async () => {
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
    draftBuild("ok");
    await waitFor(() => expect(runtime.apis.shardResources.release).toHaveBeenCalledWith(artifact.id, "draft"));
    expect(frame()).toBe(original);
    expect(frame().src).toBe(src);
    expect(fetch).toHaveBeenCalledTimes(1);
    expect(new URL(frame().src).searchParams.get("channel")).toBeNull();
  });

  it("opens published data even when a successful draft already exists", async () => {
    draftBuild("ok");
    render(<DocSurfaceUI artifact={artifact} />);
    await waitFor(() => expect(new URL(frame().src).searchParams.get("build_id")).toBe("published-1"));
    expect(new URL(frame().src).searchParams.get("channel")).toBeNull();
    expect(vi.mocked(createBridgeV2Host).mock.lastCall?.[0].target).toMatchObject({ environment: "published", contractHash: "published-hash" });
  });

  it("uses draft data only in an explicit preview and cleans up its bridge on exit", async () => {
    const detach = vi.fn();
    vi.mocked(createBridgeV2Host).mockImplementation(() => ({ attach() {}, detach, pushTheme() {} }));
    render(<DocSurfaceUI artifact={artifact} />);
    await waitFor(() => expect(frame()).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: "Preview draft" }));
    await waitFor(() => expect(new URL(frame().src).searchParams.get("channel")).toBe("draft"));
    expect(screen.getByText(/Separate test data/)).toBeInTheDocument();
    expect(vi.mocked(createBridgeV2Host).mock.lastCall?.[0].target.environment).toBe("draft");
    now += TTL + 1;
    draftBuild("ok", "draft-3");
    await waitFor(() => expect(new URL(frame().src).searchParams.get("v")).toBe("draft-3"));
    expect(frame().src).toContain("access_token=content-2");
    const previousDetaches = detach.mock.calls.length;
    fireEvent.click(screen.getByRole("button", { name: "Back to published" }));
    await waitFor(() => expect(new URL(frame().src).searchParams.get("channel")).toBeNull());
    expect(detach.mock.calls.length).toBeGreaterThan(previousDetaches);
    expect(vi.mocked(createBridgeV2Host).mock.lastCall?.[0].target.environment).toBe("published");
  });

  it("does not cover the published journal with a failed draft build", async () => {
    render(<DocSurfaceUI artifact={artifact} />);
    await waitFor(() => expect(frame()).toBeInTheDocument());
    const src = frame().src;
    draftBuild("failed");
    expect(screen.queryByText("broken draft")).toBeNull();
    expect(frame().src).toBe(src);
    fireEvent.click(screen.getByRole("button", { name: "Preview draft" }));
    expect(await screen.findByText("broken draft")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Back to published" }));
    expect(screen.queryByText("broken draft")).toBeNull();
  });

  it("labels an unpublished shard as draft, then switches only on activation", async () => {
    runtime.apis.shardResources.release.mockImplementation(async (_id, channel) => channel === "published"
      ? { protocol: "bridge/1", available: false }
      : { protocol: "bridge/2", buildId: "draft-1", contractHash: "draft-hash" });
    render(<DocSurfaceUI artifact={artifact} />);
    await waitFor(() => expect(new URL(frame().src).searchParams.get("channel")).toBe("draft"));
    expect(screen.getByText(/Separate test data/)).toBeInTheDocument();
    // Compiling the published channel merely stages it; it isn't active yet.
    act(() => useAppStore.getState().setShardBuild(shardBuildFromWire({ page_id: artifact.id, channel: "published", status: "ok", build_id: "published-2" })));
    expect(new URL(frame().src).searchParams.get("channel")).toBe("draft");
    publish();
    await waitFor(() => expect(new URL(frame().src).searchParams.get("build_id")).toBe("published-2"));
    expect(new URL(frame().src).searchParams.get("channel")).toBeNull();
    draftBuild("ok", "post-publication-draft");
    await waitFor(() => expect(vi.mocked(createBridgeV2Host).mock.lastCall?.[0].target.environment).toBe("published"));
    expect(new URL(frame().src).searchParams.get("build_id")).toBe("published-2");
  });

  it("exits an explicit preview on publication and allows preview again afterward", async () => {
    render(<DocSurfaceUI artifact={artifact} />);
    fireEvent.click(await screen.findByRole("button", { name: "Preview draft" }));
    await waitFor(() => expect(new URL(frame().src).searchParams.get("channel")).toBe("draft"));
    publish();
    await waitFor(() => expect(new URL(frame().src).searchParams.get("build_id")).toBe("published-2"));
    expect(new URL(frame().src).searchParams.get("channel")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Preview draft" }));
    await waitFor(() => expect(new URL(frame().src).searchParams.get("channel")).toBe("draft"));
  });

  it("does not fall back to draft when the published release request fails", async () => {
    runtime.apis.shardResources.release.mockImplementation(async (_id, channel) => {
      if (channel === "published") throw new Error("unavailable");
      return { protocol: "bridge/2", buildId: "draft-1", contractHash: "draft-hash" };
    });
    render(<DocSurfaceUI artifact={artifact} />);
    await screen.findByRole("alert");
    expect(screen.queryByTitle("Research")).toBeNull();
    expect(createBridgeV2Host).not.toHaveBeenCalled();
  });

  it("ignores an old release response arriving after publication", async () => {
    let resolve!: (value: ShardReleaseMetadata) => void;
    runtime.apis.shardResources.release.mockImplementation((_id, channel) => channel === "published"
      ? new Promise(done => { resolve = done; })
      : Promise.resolve({ protocol: "bridge/2", buildId: "draft-1", contractHash: "draft-hash" }));
    render(<DocSurfaceUI artifact={artifact} />);
    publish();
    await waitFor(() => expect(new URL(frame().src).searchParams.get("build_id")).toBe("published-2"));
    await act(async () => resolve({ protocol: "bridge/1", available: false }));
    expect(new URL(frame().src).searchParams.get("build_id")).toBe("published-2");
    expect(new URL(frame().src).searchParams.get("channel")).toBeNull();
  });

  it("preserves legacy publication and labels its shared data accurately", async () => {
    let available = false;
    runtime.apis.shardResources.release.mockImplementation(async (_id, channel) => ({ protocol: "bridge/1", available: channel === "draft" || available }));
    render(<DocSurfaceUI artifact={artifact} />);
    await waitFor(() => expect(new URL(frame().src).searchParams.get("channel")).toBe("draft"));
    expect(screen.getByText("Saved data is shared with published.")).toBeInTheDocument();
    available = true;
    act(() => useAppStore.getState().setShardBuild(shardBuildFromWire({ page_id: artifact.id, channel: "published", status: "ok", build_id: "legacy-published" })));
    await waitFor(() => expect(new URL(frame().src).searchParams.get("channel")).toBeNull());
    expect(new URL(frame().src).searchParams.get("v")).toBe("legacy-published");
  });

  it("does not carry draft preview into another shard", async () => {
    const { rerender } = render(<DocSurfaceUI artifact={artifact} />);
    fireEvent.click(await screen.findByRole("button", { name: "Preview draft" }));
    await waitFor(() => expect(new URL(frame().src).searchParams.get("channel")).toBe("draft"));
    rerender(<DocSurfaceUI artifact={{ ...artifact, id: "shard-2" }} />);
    await waitFor(() => expect(new URL(frame().src).pathname).toBe("/content/shard-2/"));
    expect(new URL(frame().src).searchParams.get("channel")).toBeNull();
    expect(vi.mocked(createBridgeV2Host).mock.lastCall?.[0].target).toMatchObject({ shardId: "shard-2", environment: "published" });
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

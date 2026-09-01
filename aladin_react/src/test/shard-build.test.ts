import { describe, expect, it, beforeEach } from "vitest";
import { useAppStore } from "@/app/state/store";
import { shardBuildFromWire, shardBuildKey } from "@/app/state/shard-build-slice";
import { createShardBuildEventHandler } from "@/shared/realtime/shard-build-event-handler";
import type { AppEventEnvelope } from "@/shared/realtime/app-event";

function buildStatusEvent(payload: unknown): AppEventEnvelope {
  return {
    eventId: `evt-${Math.random()}`,
    type: "artifact.build-status",
    subscriptionKey: { stream: "workspace", resourceKind: "artifact", resourceId: "shard-1" },
    payload,
    occurredAt: "2026-06-13T00:00:00Z",
  };
}

describe("shardBuildFromWire", () => {
  it("maps snake_case wire fields and defaults channel to draft", () => {
    const info = shardBuildFromWire({
      page_id: "shard-1",
      channel: "draft",
      status: "ok",
      build_id: "abc",
      built_at: "2026-06-13T00:00:00Z",
    });
    expect(info).toMatchObject({
      pageId: "shard-1",
      channel: "draft",
      status: "ok",
      buildId: "abc",
      errors: "",
    });
  });

  it("treats unknown channel as draft and missing fields as empty", () => {
    const info = shardBuildFromWire({ page_id: "s", channel: "weird", status: "building" });
    expect(info.channel).toBe("draft");
    expect(info.buildId).toBe("");
    expect(info.errors).toBe("");
  });
});

describe("shard build event handler", () => {
  beforeEach(() => {
    useAppStore.setState({ shardBuilds: {}, shardPublications: {} });
  });

  it("routes build-status events into the slice keyed by page+channel", () => {
    const handle = createShardBuildEventHandler();
    handle(
      buildStatusEvent({ page_id: "shard-1", channel: "draft", status: "failed", errors: "boom" }),
    );
    const entry = useAppStore.getState().shardBuilds[shardBuildKey("shard-1", "draft")];
    expect(entry?.status).toBe("failed");
    expect(entry?.errors).toBe("boom");
  });

  it("ignores non-build-status events and malformed payloads", () => {
    const handle = createShardBuildEventHandler();
    handle({ ...buildStatusEvent({ page_id: "x" }), type: "artifact.updated" });
    handle(buildStatusEvent(null));
    handle(buildStatusEvent({ status: "ok" })); // no page_id
    expect(Object.keys(useAppStore.getState().shardBuilds)).toHaveLength(0);
  });

  it("keeps committed publication separate from successful staging and rejects mismatched targets", () => {
    const handle = createShardBuildEventHandler();
    handle(buildStatusEvent({ page_id: "shard-1", channel: "published", status: "ok", build_id: "staged" }));
    expect(useAppStore.getState().shardPublications).toEqual({});
    const event = { ...buildStatusEvent({ page_id: "shard-1", protocol: "bridge/2", buildId: "active", contractHash: "hash" }), type: "artifact.published" };
    handle(event);
    expect(useAppStore.getState().shardPublications["shard-1"]).toMatchObject({ buildId: "active", contractHash: "hash" });
    const state = useAppStore.getState();
    handle(event);
    expect(useAppStore.getState()).toBe(state);
    handle({ ...event, payload: { ...event.payload as object, page_id: "another-shard" } });
    handle({ ...event, payload: null });
    handle({ ...event, payload: { page_id: "shard-1" } });
    expect(useAppStore.getState()).toBe(state);
  });
});

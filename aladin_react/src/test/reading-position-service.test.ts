/**
 * The reading-position read model: replica-seeded per-key stream, committed-write
 * push-back (the echo frame lands as a no-op re-emit of the same value), and the
 * REST fallback when no local replica exists (web).
 */
import { describe, expect, it, vi } from "vitest";
import { firstValueFrom } from "rxjs";

import type { LocalReadingPositionRepo } from "@/repos/reading-position/local-reading-position-repo";
import type { ReadingPositionRepo } from "@/repos/reading-position/reading-position-repo";
import {
  ReadingPositionService,
  type ReadingPositionState,
} from "@/services/reading-position/reading-position-service";
import type { Result } from "@/shared/flow/result";

function collect(service: ReadingPositionService, id: string) {
  const seen: ReadingPositionState[] = [];
  const sub = service.byArtifact(id).subscribe((r: Result<ReadingPositionState>) => {
    if (r.ok) seen.push(r.value);
  });
  return { seen, stop: () => sub.unsubscribe() };
}

const restStub = (overrides: Partial<ReadingPositionRepo> = {}): ReadingPositionRepo => ({
  get: vi.fn(async () => null),
  put: vi.fn(async (artifactId: string, page: number) => ({
    artifactId,
    page,
    seq: 1,
    updatedAt: 1000,
  })),
  ...overrides,
});

describe("ReadingPositionService", () => {
  it("seeds from the local replica when present, and 'no position' is a value", async () => {
    const local: LocalReadingPositionRepo = {
      get: vi.fn(async (id: string) =>
        id === "doc-1" ? { id, page: 87, updatedAt: 500 } : null,
      ),
    };
    const svc = new ReadingPositionService(restStub(), local);

    const first = await firstValueFrom(svc.byArtifact("doc-1"));
    expect(first.ok && first.value).toEqual({ artifactId: "doc-1", page: 87, updatedAt: 500 });

    const none = await firstValueFrom(svc.byArtifact("doc-2"));
    expect(none.ok && none.value).toEqual({ artifactId: "doc-2", page: null, updatedAt: 0 });
  });

  it("falls back to REST without a replica (web)", async () => {
    const rest = restStub({
      get: vi.fn(async (id: string) => ({ id, page: 12, updatedAt: 900 })),
    });
    const svc = new ReadingPositionService(rest, null);
    const first = await firstValueFrom(svc.byArtifact("doc-1"));
    expect(first.ok && first.value).toEqual({ artifactId: "doc-1", page: 12, updatedAt: 900 });
  });

  it("report pushes the committed row; a frame re-applies as the same value", async () => {
    const rest = restStub();
    const svc = new ReadingPositionService(rest, { get: async () => null });
    const { seen, stop } = collect(svc, "doc-1");
    await Promise.resolve(); // let the seed fetch land

    svc.report("doc-1", 42);
    await vi.waitFor(() => {
      expect(seen.at(-1)).toEqual({ artifactId: "doc-1", page: 42, updatedAt: 1000 });
    });
    expect(rest.put).toHaveBeenCalledWith("doc-1", 42);

    // The echo frame from the replica carries the same committed value.
    svc.handleUpserted({ id: "doc-1", page: 42, updatedAt: 1000 });
    expect(seen.at(-1)).toEqual({ artifactId: "doc-1", page: 42, updatedAt: 1000 });
    stop();
  });

  it("a remote frame updates the stream (apply-at-open reads it on next open)", async () => {
    const svc = new ReadingPositionService(restStub(), { get: async () => null });
    svc.handleUpserted({ id: "doc-9", page: 7, updatedAt: 2000 });
    const first = await firstValueFrom(svc.byArtifact("doc-9"));
    expect(first.ok && first.value).toEqual({ artifactId: "doc-9", page: 7, updatedAt: 2000 });
  });
});

import { createContext, useContext } from "react";

import { ApiError, type ApiClient } from "@/shared/api/client";
import type { SearchResponse, UserArtifact } from "@/shared/api/models";

import type { UnfurlResult } from "./board-links";

/**
 * The read-live content plane for doc windows and the picker.
 *
 * A doc window stores only (artifactId, page) — its body text resolves through this
 * context at render time. BoardPane mounts the provider around <Tldraw>, so shape
 * components (which render inside the editor's tree) can reach it.
 *
 * Resolution: try the ingested-document plane first (`/document` meta once per artifact,
 * then `/document/pages` per page); a 404 there means nothing was ingested — the normal
 * state for a note — so fall back to the page-blocks projection. Everything is cached
 * with in-flight dedupe and a short TTL ("read-live": fresh enough, never a fetch storm).
 */

export type DocPageContent =
  | { state: "loading" }
  | { state: "missing" }
  | { state: "ready"; sourceLine: string; excerpt: string; pageCount: number };

/** One row the picker can insert. */
export interface PickerArtifact {
  id: string;
  /** Doc-window kind ("file" | "note" | "link"). */
  kind: string;
  title: string;
  meta: string;
}

export interface BoardContentSource {
  /** Subscribe to (artifactId, page); useSyncExternalStore-compatible. */
  subscribe(artifactId: string, page: number, onChange: () => void): () => void;
  get(artifactId: string, page: number): DocPageContent;
  /** The board's folder siblings, insertable as live windows. */
  listFolderArtifacts(folderId: string | null): Promise<PickerArtifact[]>;
  /** The picker's "then everywhere": the workspace search, narrowed to insertable kinds. */
  searchArtifacts(query: string): Promise<PickerArtifact[]>;
  /**
   * Resolve an external URL's preview (server-side — CORS + SSRF live there). Optional:
   * hosts without a backend leave link objects at their bare-URL rendering.
   */
  unfurl?(url: string): Promise<UnfurlResult>;
  /**
   * "Work this": create a paged worksheet (a board wearing the paper costume) in the
   * study's folder, born citing its exercise. Returns the new artifact id. Optional —
   * hosts without a writable workspace (the spike) simply hide the affordance.
   */
  createWorksheet?(opts: {
    folderId: string | null;
    title: string;
    cite: { artifactId: string; page: number; title: string };
  }): Promise<string>;
}

export const BoardContentContext = createContext<BoardContentSource | null>(null);

export function useBoardContent(): BoardContentSource | null {
  return useContext(BoardContentContext);
}

/** The board's own folder — scopes the picker's "this folder" listing. */
export const BoardFolderContext = createContext<string | null>(null);

export function useBoardFolder(): string | null {
  return useContext(BoardFolderContext);
}

const TTL_MS = 30_000;
const RETRY_MS = 8_000;
const EXCERPT_LIMIT = 700;

interface DocumentMeta {
  pageCount: number;
  sections: { title: string; pageFrom: number }[];
}

interface Entry {
  value: DocPageContent;
  fetchedAt: number;
  inflight: boolean;
  listeners: Set<() => void>;
}

/** tolerant BlockNote-blocks → plain text (unknown node shapes just contribute nothing) */
export function flattenBlocks(blocks: unknown): string {
  const out: string[] = [];
  const walk = (node: unknown) => {
    if (Array.isArray(node)) {
      node.forEach(walk);
      return;
    }
    if (typeof node !== "object" || node === null) return;
    const record = node as { text?: unknown; content?: unknown; children?: unknown };
    if (typeof record.text === "string") out.push(record.text);
    if (record.content) walk(record.content);
    if (record.children) walk(record.children);
  };
  walk(blocks);
  return out.join(" ").replace(/\s+/g, " ").trim();
}

/** The section a page falls in — the doc window's source line, per the design. */
export function sectionForPage(meta: DocumentMeta, page: number): string {
  let best: string | null = null;
  for (const section of meta.sections) {
    if (section.pageFrom <= page) best = section.title;
  }
  return best ?? `p. ${page}`;
}

// Insertable as live windows today: PDFs (ingested docs) and notes (blocks projection).
// Links/voice get their own window bodies later — listing them would insert tombstones.
const ARTIFACT_TYPE_TO_KIND: Record<string, string> = {
  file: "file",
  page: "note",
};

export function createBoardContentSource(client: ApiClient): BoardContentSource {
  const entries = new Map<string, Entry>();
  const metas = new Map<string, Promise<DocumentMeta | null>>();

  function entry(key: string): Entry {
    let found = entries.get(key);
    if (!found) {
      found = { value: { state: "loading" }, fetchedAt: 0, inflight: false, listeners: new Set() };
      entries.set(key, found);
    }
    return found;
  }

  function publish(key: string, value: DocPageContent) {
    const e = entry(key);
    e.value = value;
    e.fetchedAt = Date.now();
    e.inflight = false;
    e.listeners.forEach((listener) => listener());
  }

  function documentMeta(artifactId: string): Promise<DocumentMeta | null> {
    let meta = metas.get(artifactId);
    if (!meta) {
      meta = client
        .fetch<{
          pageCount: number;
          sections?: { title: string; pageFrom: number }[] | null;
        }>(`/api/artifacts/${encodeURIComponent(artifactId)}/document`)
        .then((doc) => ({ pageCount: doc.pageCount, sections: doc.sections ?? [] }))
        .catch((error: unknown) => {
          if (error instanceof ApiError && error.status === 404) return null;
          metas.delete(artifactId);
          throw error;
        });
      metas.set(artifactId, meta);
    }
    return meta;
  }

  async function resolve(artifactId: string, page: number): Promise<DocPageContent> {
    const meta = await documentMeta(artifactId);
    if (meta) {
      const clamped = Math.max(1, Math.min(meta.pageCount || 1, page));
      const result = await client.fetch<{ pages: { page: number; text: string }[] }>(
        `/api/artifacts/${encodeURIComponent(artifactId)}/document/pages?from=${clamped}&to=${clamped}`,
      );
      const text = result.pages[0]?.text ?? "";
      return {
        state: "ready",
        sourceLine: sectionForPage(meta, clamped),
        excerpt: text.slice(0, EXCERPT_LIMIT),
        pageCount: meta.pageCount || 1,
      };
    }
    // Nothing ingested — a note (or a link). The blocks projection is the body.
    const pageDoc = await client.fetch<{ blocks: unknown }>(
      `/api/pages/${encodeURIComponent(artifactId)}`,
    );
    return {
      state: "ready",
      sourceLine: "note · edits sync both ways",
      excerpt: flattenBlocks(pageDoc.blocks).slice(0, EXCERPT_LIMIT),
      pageCount: 1,
    };
  }

  function fetchInto(key: string, artifactId: string, page: number) {
    const e = entry(key);
    if (e.inflight) return;
    e.inflight = true;
    resolve(artifactId, page)
      .then((value) => publish(key, value))
      .catch((error: unknown) => {
        if (error instanceof ApiError && error.status === 404) {
          publish(key, { state: "missing" });
          return;
        }
        // Transient failure: stay in loading, retry while someone still watches.
        const stale = entry(key);
        stale.inflight = false;
        stale.fetchedAt = Date.now() - TTL_MS + RETRY_MS;
        if (stale.value.state === "loading") stale.listeners.forEach((l) => l());
      });
  }

  return {
    subscribe(artifactId, page, onChange) {
      const key = `${artifactId}:${page}`;
      const e = entry(key);
      e.listeners.add(onChange);
      if (Date.now() - e.fetchedAt > TTL_MS) fetchInto(key, artifactId, page);
      return () => {
        e.listeners.delete(onChange);
      };
    },
    get(artifactId, page) {
      return entry(`${artifactId}:${page}`).value;
    },
    async searchArtifacts(query) {
      const q = query.trim();
      if (!q) return [];
      const params = new URLSearchParams({ q, limit: "12" });
      const response = await client.fetch<SearchResponse>(`/api/search?${params.toString()}`);
      return (response?.sections ?? [])
        .filter((section) => section.type === "artifact")
        .flatMap((section) => section.hits)
        .filter((hit) => ARTIFACT_TYPE_TO_KIND[hit.kind])
        .map((hit) => ({
          id: hit.id,
          kind: ARTIFACT_TYPE_TO_KIND[hit.kind],
          title: hit.title,
          meta: hit.subtitle || (hit.kind === "page" ? "note" : hit.kind),
        }));
    },
    async unfurl(url) {
      return client.fetch<UnfurlResult>(`/api/unfurl`, {
        method: "POST",
        body: JSON.stringify({ url }),
      });
    },
    async createWorksheet({ folderId, title, cite }) {
      const record = await client.fetch<UserArtifact>(`/api/artifacts/`, {
        method: "POST",
        body: JSON.stringify({
          type: "board",
          folderId,
          title,
          content: "",
          metadata: { board: { paper: "paged", cite } },
        }),
      });
      return record.id;
    },
    async listFolderArtifacts(folderId) {
      const query = folderId ? `?folderId=${encodeURIComponent(folderId)}` : "";
      const artifacts = await client.fetch<UserArtifact[]>(`/api/artifacts/${query}`);
      return artifacts
        .filter((artifact) => ARTIFACT_TYPE_TO_KIND[artifact.type])
        .map((artifact) => ({
          id: artifact.id,
          kind: ARTIFACT_TYPE_TO_KIND[artifact.type],
          title: artifact.title,
          meta: artifact.type === "page" ? "note" : artifact.type,
        }));
    },
  };
}

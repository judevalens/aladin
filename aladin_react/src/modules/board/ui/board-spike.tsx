import { useEffect, useMemo } from "react";

import { ApiError, type ApiClient } from "@/shared/api/client";
import type { UserArtifact } from "@/shared/api/models";
import { BoardPane } from "./board-pane";

/**
 * /spike/board — the board surface on an in-memory ApiClient, no auth, no backend.
 *
 * This is where day-to-day board iteration happens: the iOS Simulator kills tldraw's
 * WebContent process (sim-only JSC bug), so the loop is browser spike → real iPad. The
 * fake client persists the snapshot in localStorage so a reload keeps the board.
 */

const SPIKE_BOARD_ID = "spike-board";
const STORAGE_KEY = "aladin.spike-board.content";

function nowIso() {
  return new Date().toISOString();
}

function spikeArtifact(content: string): UserArtifact {
  return {
    id: SPIKE_BOARD_ID,
    type: "board",
    folderId: "spike-folder",
    title: "Collar board",
    content,
    metadata: {},
    createdAt: nowIso(),
    updatedAt: nowIso(),
  };
}

const SPIKE_FOLDER = [
  { id: "a_opt", type: "file", title: "Option Strategies & Payoff Algebra" },
  { id: "a_worksheet", type: "file", title: "Worksheet 2 — collars" },
  { id: "a_writeup", type: "page", title: "Collars — write-up" },
  { id: "a_macro", type: "link", title: "macrodesk — collar screens" },
];

const SPIKE_SECTIONS = [
  { title: "§2 · payoff algebra of the basic legs", pageFrom: 41 },
  { title: "§4.2 · collars", pageFrom: 88 },
  { title: "§6 · greeks as partial derivatives", pageFrom: 152 },
];

function spikePageText(page: number): string {
  if (page === 94) {
    return (
      "The holder buys a put struck at K₁ and sells a call struck at K₂ > K₁, financing " +
      "part or all of the protection with the call premium. Because the payoff is bounded " +
      "on both sides, the maximum gain and loss follow directly from the strikes and the " +
      "net premium C."
    );
  }
  return `Page ${page} of the spike document — the board reads this window's own page, not the folder's.`;
}

/**
 * `/spike/board?fail=load` makes the board GET reject; `?fail=save` makes every PATCH
 * reject. The two failure paths the pane must survive (a failed load must never arm saving;
 * a failed save must retry, not drop the edit), driven without a backend.
 */
function failMode(): "load" | "save" | null {
  const value = new URLSearchParams(window.location.search).get("fail");
  return value === "load" || value === "save" ? value : null;
}

function createSpikeApiClient(): ApiClient {
  const fail = failMode();
  return {
    resolveUrl: (path) => path,
    fetch<T>(path: string, init?: RequestInit): Promise<T> {
      const method = init?.method ?? "GET";
      const url = new URL(path, "http://spike.local");

      if (url.pathname === `/api/artifacts/${SPIKE_BOARD_ID}`) {
        if (method === "GET") {
          if (fail === "load") return Promise.reject(new ApiError("spike: load failure", 503));
          const content = window.localStorage.getItem(STORAGE_KEY) ?? "";
          return Promise.resolve(spikeArtifact(content) as T);
        }
        if (method === "PATCH") {
          if (fail === "save") return Promise.reject(new ApiError("spike: save failure", 503));
          const body = JSON.parse(String(init?.body ?? "{}")) as { content?: string };
          if (typeof body.content === "string") {
            window.localStorage.setItem(STORAGE_KEY, body.content);
          }
          return Promise.resolve(spikeArtifact(body.content ?? "") as T);
        }
      }

      if (url.pathname === "/api/artifacts/" && method === "GET") {
        return Promise.resolve(
          SPIKE_FOLDER.map((a) => ({
            ...a,
            folderId: "spike-folder",
            content: "",
            metadata: {},
            createdAt: nowIso(),
            updatedAt: nowIso(),
          })) as T,
        );
      }

      const pagesMatch = url.pathname.match(/^\/api\/artifacts\/(a_opt|a_worksheet)\/document\/pages$/);
      if (pagesMatch) {
        const from = Number(url.searchParams.get("from") ?? "1");
        return Promise.resolve({ pages: [{ page: from, text: spikePageText(from) }] } as T);
      }

      const docMatch = url.pathname.match(/^\/api\/artifacts\/(a_opt|a_worksheet)\/document$/);
      if (docMatch) {
        const isBook = docMatch[1] === "a_opt";
        return Promise.resolve({
          artifactId: docMatch[1],
          status: "ready",
          pageCount: isBook ? 361 : 6,
          sections: isBook ? SPIKE_SECTIONS : [{ title: "§1 · exercises", pageFrom: 1 }],
        } as T);
      }

      if (url.pathname === "/api/pages/a_writeup" && method === "GET") {
        return Promise.resolve({
          blocks: [
            { type: "paragraph", content: [{ type: "text", text: "Floor comes from the put; width from both strikes; premium decides zero-cost." }] },
          ],
          revision: 1,
        } as T);
      }

      return Promise.reject(new ApiError(`spike client: no route for ${method} ${path}`, 404));
    },
    fetchBlob() {
      return Promise.reject(new ApiError("spike client: no blobs", 404));
    },
  };
}

export function BoardSpike() {
  const client = useMemo(createSpikeApiClient, []);

  // The spike renders outside the auth shell, which is what normally stamps the theme.
  useEffect(() => {
    const html = document.documentElement;
    const previous = html.dataset.theme;
    html.dataset.theme = "dark";
    return () => {
      if (previous) html.dataset.theme = previous;
      else delete html.dataset.theme;
    };
  }, []);

  return (
    <div className="h-screen w-screen overflow-hidden bg-bg">
      <BoardPane boardId={SPIKE_BOARD_ID} title="Collar board" client={client} />
    </div>
  );
}

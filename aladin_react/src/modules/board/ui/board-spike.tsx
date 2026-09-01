import { useMemo } from "react";

import { ApiError, type ApiClient } from "@/shared/api/client";
import type { UserArtifact } from "@/shared/api/models";
import { BoardPane } from "./board-pane";

/**
 * /spike/board — the board surface with no auth and no backend.
 *
 * This is where day-to-day board iteration happens: the iOS Simulator kills tldraw's
 * WebContent process (sim-only JSC bug), so the loop is browser spike → real iPad. The
 * pane runs in LOCAL mode (tldraw's own IndexedDB persistence via persistenceKey); the
 * fake client only feeds the picker/doc-window content plane.
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

function createSpikeApiClient(): ApiClient {
  return {
    resolveUrl: (path) => path,
    fetch<T>(path: string, init?: RequestInit): Promise<T> {
      const method = init?.method ?? "GET";
      const url = new URL(path, "http://spike.local");

      if (url.pathname === `/api/artifacts/${SPIKE_BOARD_ID}`) {
        if (method === "GET") {
          // `/spike/board?paper=1` walks the paged (worksheet) regime without a backend.
          const paper = new URLSearchParams(window.location.search).get("paper") === "1";
          const record = spikeArtifact("");
          if (paper) {
            record.metadata = {
              board: {
                paper: "paged",
                cite: { artifactId: "a_opt", page: 96, title: "Option Strategies" },
              },
            };
          }
          return Promise.resolve(record as T);
        }
        if (method === "PATCH") {
          const body = JSON.parse(String(init?.body ?? "{}")) as { content?: string };
          if (typeof body.content === "string") {
            window.localStorage.setItem(STORAGE_KEY, body.content);
          }
          return Promise.resolve(spikeArtifact(body.content ?? "") as T);
        }
      }

      if (url.pathname === "/api/search" && method === "GET") {
        const q = (url.searchParams.get("q") ?? "").toLowerCase();
        const everywhere = [
          ...SPIKE_FOLDER,
          { id: "a_greeks", type: "file", title: "Greeks — a field guide" },
          { id: "a_journal", type: "page", title: "Journal — week 34" },
        ];
        return Promise.resolve({
          sections: [
            {
              type: "artifact",
              label: "Artifacts",
              hits: everywhere
                .filter((a) => a.title.toLowerCase().includes(q))
                .map((a) => ({ kind: a.type, id: a.id, title: a.title, subtitle: "elsewhere", score: 1 })),
            },
          ],
        } as T);
      }

      if (url.pathname === "/api/unfurl" && method === "POST") {
        const body = JSON.parse(String(init?.body ?? "{}")) as { url?: string };
        const target = new URL(body.url ?? "https://example.com");
        // A slow-ish fake so the pending state is visible in the spike.
        return new Promise((resolve) =>
          setTimeout(
            () =>
              resolve({
                url: target.toString(),
                domain: target.hostname.replace(/^www\./, ""),
                title: `${target.hostname} — spike preview`,
                description:
                  "A faked unfurl so the link object's ready state renders without a backend.",
                siteName: "Spike",
                imageUrl: "",
                faviconUrl: "",
              } as T),
            600,
          ),
        );
      }

      if (url.pathname === "/api/artifacts/" && method === "GET") {
        return Promise.resolve(
          SPIKE_FOLDER.map((a) => ({
            ...a,
            folderId: "spike-folder",
            content: "",
            metadata: a.type === "file" ? { mimeType: "application/pdf" } : {},
            createdAt: nowIso(),
            updatedAt: nowIso(),
          })) as T,
        );
      }

      const fileMatch = url.pathname.match(/^\/api\/artifacts\/(a_opt|a_worksheet|a_greeks)$/);
      if (fileMatch && method === "GET") {
        return Promise.resolve({
          id: fileMatch[1], type: "file", title: "Sample PDF", content: "",
          metadata: { mimeType: "application/pdf" }, createdAt: nowIso(), updatedAt: nowIso(),
        } as T);
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
    async fetchBlob(path, init) {
      if (/^\/api\/artifacts\/(a_opt|a_worksheet|a_greeks)\/resource$/.test(path)) {
        // Existing local reader fixture: real page pixels exercise the production renderer.
        const response = await fetch("/__harness.pdf", { signal: init?.signal });
        if (!response.ok) throw new ApiError("PDF fixture unavailable", response.status);
        return response.blob();
      }
      return Promise.reject(new ApiError("spike client: no blobs", 404));
    },
  };
}

export function BoardSpike() {
  const client = useMemo(createSpikeApiClient, []);

  return (
    <div className="h-screen w-screen overflow-hidden bg-bg">
      <BoardPane boardId={SPIKE_BOARD_ID} title="Collar board" client={client} />
    </div>
  );
}

import { useEffect, useRef, useState } from "react";
import type { Artifact } from "@/shared/api/models";
import { useAppComposition } from "@/app/composition/app-composition";
import { useAppStore } from "@/app/state/store";
import { shardBuildFromWire, shardBuildKey } from "@/app/state/shard-build-slice";
import type { ShardChannel } from "@/app/state/shard-build-slice";
import { cn } from "@/lib/utils";

// useServedUrl resolves the content-origin URL for a Doc Surface page. In web dev
// apiBaseUrl is "" so this is relative (/content/...) and goes through the vite
// proxy (same origin); in the desktop app it is the absolute API origin.
//
// The desktop app authenticates with a bearer token and sets no cookie, and an
// iframe can send neither — so the session token is passed as ?access_token (the
// same scheme the realtime WebSocket uses). The serve route propagates it onto
// the page's sub-resource URLs.
//
// channel selects the published (default) or draft build; nonce (a build id) is
// appended so a fresh build reloads the iframe by changing its src.
function useServedUrl(pageId: string, channel: ShardChannel, nonce?: string): string {
  const { runtime } = useAppComposition();
  const base = runtime.config.apiBaseUrl;
  const token = runtime.desktopSession.getToken();
  const params = new URLSearchParams();
  if (token) params.set("access_token", token);
  if (channel === "draft") params.set("channel", "draft");
  if (nonce) params.set("v", nonce);
  // In the desktop app, signal the serve route to emit vendor://deps/<sha> import
  // URLs so deps load from the local Tauri cache (zero network after first fetch).
  if (typeof window !== "undefined" && "__TAURI_INTERNALS__" in window) {
    params.set("client", "tauri");
  }
  const q = params.toString();
  return `${base}/content/${pageId}/${q ? `?${q}` : ""}`;
}

// useShardBuild seeds the draft build state for a page (one fetch on mount) and
// returns the live entry from the store (updated by realtime build-status events).
function useShardBuild(pageId: string) {
  const { runtime } = useAppComposition();
  const setShardBuild = useAppStore((s) => s.setShardBuild);
  const draft = useAppStore((s) => s.shardBuilds[shardBuildKey(pageId, "draft")]);

  useEffect(() => {
    let cancelled = false;
    void runtime.apis.shards
      .getBuildState(pageId, "draft")
      .then((wire) => {
        if (!cancelled && wire?.page_id) setShardBuild(shardBuildFromWire(wire));
      })
      .catch(() => undefined); // no draft yet / offline — fine, stays unseeded
    return () => {
      cancelled = true;
    };
  }, [pageId, runtime, setShardBuild]);

  return draft;
}

function BuildStatusChip({ status }: { status: "building" | "ok" | "failed" }) {
  const label = status === "building" ? "Building…" : status === "ok" ? "Built" : "Build failed";
  const tone =
    status === "building"
      ? "text-amber border-amber-line"
      : status === "failed"
        ? "text-against border-against/40"
        : "text-ink-3 border-line";
  return (
    <div
      className={cn(
        "pointer-events-none absolute right-3 top-3 z-10 flex items-center gap-1.5 rounded-chip border bg-panel/90 px-2.5 py-1 text-meta font-mono shadow-panel backdrop-blur",
        tone,
      )}
    >
      <span
        className={cn(
          "h-1.5 w-1.5 rounded-full",
          status === "building" ? "bg-amber animate-pulse" : status === "failed" ? "bg-against" : "bg-for",
        )}
      />
      {label}
    </div>
  );
}

function BuildErrorOverlay({ log }: { log: string }) {
  return (
    <div className="absolute inset-0 z-20 overflow-auto bg-bg/95 p-6 backdrop-blur">
      <div className="mb-2 text-small font-medium text-against">Build failed</div>
      <pre className="whitespace-pre-wrap break-words font-mono text-small leading-relaxed text-ink-2">
        {log || "(no diagnostics)"}
      </pre>
    </div>
  );
}

/**
 * DocSurfaceUI renders one agent-authored "app" page in a sandboxed iframe.
 *
 * Isolation: `sandbox="allow-scripts"` with NO `allow-same-origin` → the frame
 * has an opaque origin and cannot reach Aladin's DOM/cookies/storage. The only
 * channel back is the origin/source-checked postMessage bridge below (read-only
 * stub for v1).
 *
 * Live build view: once a successful DRAFT build exists, the iframe serves the
 * draft channel and reloads on each new build; a status chip reflects the live
 * build state and a failed build replaces the (stale) frame with its diagnostics.
 */
export function DocSurfaceUI({ artifact, hidden = false }: { artifact: Artifact; hidden?: boolean }) {
  const iframeRef = useRef<HTMLIFrameElement>(null);
  const draft = useShardBuild(artifact.id);

  // Serve draft only once a successful draft build exists; reload on its build id.
  // Until then, serve the published build (the default, always-valid view).
  const [draftNonce, setDraftNonce] = useState<string>("");
  useEffect(() => {
    if (draft?.status === "ok" && draft.buildId) setDraftNonce(draft.buildId);
  }, [draft?.status, draft?.buildId]);

  const hasDraftBuild = draftNonce !== "";
  const channel: ShardChannel = hasDraftBuild ? "draft" : "published";
  const src = useServedUrl(artifact.id, channel, hasDraftBuild ? draftNonce : undefined);

  const status = draft?.status;
  const showChip = status === "building" || status === "ok" || status === "failed";
  const showError = status === "failed";

  useEffect(() => {
    function onMessage(event: MessageEvent) {
      // A sandboxed (opaque-origin) frame has event.origin "null"; the reliable
      // check is the source window, not the origin string.
      if (event.source !== iframeRef.current?.contentWindow) return;
      const data = event.data as { type?: string; cmd?: string; id?: number } | null;
      if (data?.type !== "aladin:bridge") return;
      // v1 capability set is read-only and tiny. Unknown commands are rejected.
      if (data.cmd === "ping") {
        (event.source as Window).postMessage(
          { type: "aladin:bridge-reply", id: data.id, ok: true, result: "pong" },
          "*",
        );
      } else {
        (event.source as Window).postMessage(
          { type: "aladin:bridge-reply", id: data.id, ok: false, error: "unknown command" },
          "*",
        );
      }
    }
    window.addEventListener("message", onMessage);
    return () => window.removeEventListener("message", onMessage);
  }, []);

  return (
    <div className={cn("relative h-full w-full", hidden && "hidden")}>
      <iframe
        ref={iframeRef}
        title={artifact.title}
        src={src}
        sandbox="allow-scripts"
        className="h-full w-full border-0 bg-bg"
      />
      {showChip && status && <BuildStatusChip status={status} />}
      {showError && <BuildErrorOverlay log={draft?.errors ?? ""} />}
    </div>
  );
}

// The shard-only keep-alive that used to live here (DocSurfaceKeepAlive + useKeepAliveIds) is
// gone: the work pane now keeps EVERY tab kind mounted, so shards no longer need their own
// mechanism, and two nested keep-alive windows would have double-mounted the iframes. See
// `modules/workspace/hooks/use-keep-alive.ts` — the `hidden` prop below is still how a shard
// learns it is off screen.

import { useEffect, useRef, useState } from "react";
import type { Artifact } from "@/shared/api/models";
import { useAppComposition } from "@/app/composition/app-composition";
import { cn } from "@/lib/utils";

// useServedUrl resolves the content-origin URL for a Doc Surface page. In web dev
// apiBaseUrl is "" so this is relative (/content/...) and goes through the vite
// proxy (same origin); in the desktop app it is the absolute API origin.
//
// The desktop app authenticates with a bearer token and sets no cookie, and an
// iframe can send neither — so the session token is passed as ?access_token (the
// same scheme the realtime WebSocket uses). The serve route propagates it onto
// the page's sub-resource URLs.
function useServedUrl(pageId: string): string {
  const { runtime } = useAppComposition();
  const base = runtime.config.apiBaseUrl;
  const token = runtime.desktopSession.getToken();
  const query = token ? `?access_token=${encodeURIComponent(token)}` : "";
  return `${base}/content/${pageId}/${query}`;
}

/**
 * DocSurfaceUI renders one agent-authored "app" page in a sandboxed iframe.
 *
 * Isolation: `sandbox="allow-scripts"` with NO `allow-same-origin` → the frame
 * has an opaque origin and cannot reach Aladin's DOM/cookies/storage. The only
 * channel back is the origin/source-checked postMessage bridge below (read-only
 * stub for v1).
 */
export function DocSurfaceUI({ artifact, hidden = false }: { artifact: Artifact; hidden?: boolean }) {
  const iframeRef = useRef<HTMLIFrameElement>(null);
  const src = useServedUrl(artifact.id);

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
    <iframe
      ref={iframeRef}
      title={artifact.title}
      src={src}
      sandbox="allow-scripts"
      className={cn("h-full w-full border-0 bg-bg", hidden && "hidden")}
    />
  );
}

// useKeepAliveIds tracks the most-recently-active page ids (LRU, capped) so their
// iframes stay mounted across tab switches and keep their JS state.
function useKeepAliveIds(activeId: string, cap: number): string[] {
  const [ids, setIds] = useState<string[]>([activeId]);
  useEffect(() => {
    setIds((prev) => [activeId, ...prev.filter((id) => id !== activeId)].slice(0, cap));
  }, [activeId, cap]);
  return ids;
}

/**
 * DocSurfaceKeepAlive renders every open "app" page that is in the keep-alive
 * window, showing the active one and CSS-hiding the rest. Switching between app
 * tabs preserves each iframe's runtime state; an LRU cap bounds memory.
 */
export function DocSurfaceKeepAlive({ activeId, artifacts }: { activeId: string; artifacts: Artifact[] }) {
  const keep = useKeepAliveIds(activeId, 5);
  const live = artifacts.filter((a) => keep.includes(a.id) || a.id === activeId);
  return (
    <div className="relative h-full w-full">
      {live.map((a) => (
        <div key={a.id} className={cn("absolute inset-0", a.id !== activeId && "hidden")}>
          <DocSurfaceUI artifact={a} hidden={a.id !== activeId} />
        </div>
      ))}
    </div>
  );
}

import { StrictMode, useEffect, useMemo, useRef, useState } from "react";
import type { ReactNode } from "react";
import type { EmbedConfig } from "@/embed/embed-config";
import { createRoot } from "react-dom/client";

import { BlockNotePageEditorDriver } from "@/modules/pages/editor/page-editor-driver";
import { createApiClient } from "@/shared/api/client";
import { createShardApi } from "@/shared/api/shard-api";
import { createContentTokenStore } from "@/shared/runtime/content-token-store";
import { createBridgeHost } from "@/modules/doc-surface/bridge/bridge-host";
import { createShardDataHub } from "@/modules/doc-surface/bridge/shard-data-hub";
import { createShardKVPort } from "@/modules/doc-surface/bridge/shard-kv-port";
import "@/index.css";

/**
 * Entry point for the **bundled page editor host** — the artifact the iPad companion ships
 * inside its app bundle and loads from `file://`.
 *
 * Why a separate entry rather than a route in the app: a route would tie the companion to
 * a running web server, dev or deployed. The iPad is a full client, so the editor's code
 * travels with it and the only thing it needs at runtime is the collab server — which any
 * implementation would need, native or not, because that is what a shared document is.
 *
 * ## Why this is a host, not an editor
 *
 * The first version mounted one editor per web view, so N open notes meant N `WKWebView`s —
 * and a WebContent process is ~187 MB. Switching between them meant hiding one native view
 * and revealing another, which reclaims nothing and repaints visibly.
 *
 * So there is exactly **one** web view for the whole app, and this is the pager inside it.
 * Every open note is a mounted `BlockNotePageEditorDriver`; only the active one is
 * displayed. The others keep their Y.Doc, their provider and their undo history alive, and
 * cost a document and a socket each rather than a process. Switching notes is a CSS
 * change — no remount, no reconnect, nothing to repaint from scratch.
 *
 * Consequences of living at `file://`, both deliberate:
 *   - **No router.** `createBrowserRouter` needs history/origin semantics a file URL does
 *     not have, so page ids arrive over the bridge instead.
 *   - **No auth shell.** There is no session in a web view; the editor authenticates to
 *     the collab server with the injected token alone.
 *
 * The host sets `window.__ALADIN_EMBED__` in a document-start script. A token in the URL
 * would leak into history, logs and referrers.
 */

/**
 * One mounted surface. `kind` decides what renders; the host is deliberately not limited
 * to notes, because the same argument applies to every web-backed surface the companion
 * has — a shard is another thing that would otherwise cost a whole web view to open.
 */
export interface HostPane {
  /** Artifact id. Also the React key, so a pane keeps its state across reorders. */
  id: string;
  kind: "page" | "shard";
  title?: string;
}

/** What the native side asks for: the whole desired state, not a delta. */
export interface HostState {
  /** Every pane that should stay mounted, in whatever order. */
  panes: HostPane[];
  /** The one that is on screen. Null hides them all. */
  active: string | null;
}

/**
 * The bridge, native → web. Declarative and idempotent on purpose: the host sends the state
 * it wants and this diffs against what exists, so a dropped or repeated call cannot leave
 * the two sides disagreeing about which notes are open.
 */
export interface AladinHost {
  sync: (state: HostState) => void;
}

declare global {
  interface Window {
    __aladinHost?: AladinHost & { _queued?: HostState };
    webkit?: {
      messageHandlers?: Record<string, { postMessage: (body: unknown) => void }>;
    };
  }
}

/** Web → native. A no-op off-device, so the bundle still runs in a plain browser. */
function post(message: Record<string, unknown>) {
  window.webkit?.messageHandlers?.anchor?.postMessage(message);
}

function Host({ config }: { config: EmbedConfig }) {
  const [state, setState] = useState<HostState>({ panes: [], active: null });

  useEffect(() => {
    // Replaces the stub the bootstrap script installed, then drains whatever arrived
    // before React mounted. Without that queue the native side would have to wait for a
    // "ready" message before it could ask for anything, and the first note would open a
    // round trip later than it needs to.
    const queued = window.__aladinHost?._queued;
    window.__aladinHost = { sync: setState };
    if (queued) setState(queued);
    post({ type: "ready" });
  }, []);

  return (
    <div className="h-screen w-screen overflow-hidden bg-bg">
      {state.panes.map((pane) => (
        <div
          key={pane.id}
          // `hidden` rather than unmounting: unmounting is what drops the collab session
          // and the undo stack, which is the whole thing this host exists to keep.
          hidden={pane.id !== state.active}
          className="h-full w-full"
        >
          <Pane pane={pane} config={config} />
        </div>
      ))}
    </div>
  );
}

function Pane({ pane, config }: { pane: HostPane; config: EmbedConfig }) {
  if (pane.kind === "page") {
    return (
      <div className="mx-auto flex h-full w-[92%] flex-col">
        <BlockNotePageEditorDriver
          pageId={pane.id}
          collabWsUrl={config.collabWsUrl}
          token={config.token}
          user={config.user}
          // Entity/ref search and projection are omitted: they route through the
          // desktop repo layer and the props are optional by design. The editor is
          // fully usable without them.
        />
      </div>
    );
  }

  return <ShardPane pane={pane} config={config} />;
}

/**
 * The three planes a shard talks to, assembled REST-only.
 *
 * Desktop reads shard state from its local replica and gets per-key liveness from the sync
 * engine's change stream. There is no replica in here, so this is the assembly web-dev
 * already uses: **REST reads, REST writes, no live stream**. A shard sees its own writes
 * (they return the new revision) and the current state when it subscribes; what it does not
 * see is another client changing a key underneath it until the frame reloads.
 *
 * Built once and shared by every shard pane — the hub keys its subscriptions by shard id,
 * and one content token serves them all.
 */
function useShardPlanes(config: EmbedConfig) {
  return useMemo(() => {
    if (!config.apiBaseUrl) return null;
    const client = createApiClient(
      {
        isDesktopApp: false,
        apiBaseUrl: config.apiBaseUrl,
        websocketBaseUrl: "",
        collabWsBaseUrl: config.collabWsUrl,
      },
      { getToken: () => config.token },
    );
    const api = createShardApi(client);
    return {
      api,
      kv: createShardKVPort(api, null),
      hub: createShardDataHub(api),
      contentTokens: createContentTokenStore(client),
    };
  }, [config]);
}

/**
 * One shard, sandboxed to an opaque origin exactly as on desktop (`allow-scripts`, no
 * `allow-same-origin`), so postMessage is its only way out — and `createBridgeHost` is what
 * answers, verbatim, filtering on the source window because an opaque origin reports "null".
 *
 * **The token in the URL is a content token, never the session bearer.** A shard's own JS can
 * read its document URL, so the bearer there would hand every shard the viewer's whole API.
 * It is minted asynchronously; until it lands no frame is mounted, because one without it
 * would only 401.
 *
 * Published channel only. Draft is the agent's sandbox and belongs to the surface where the
 * agent is working; the companion shows the real thing.
 */
function ShardPane({ pane, config }: { pane: HostPane; config: EmbedConfig }) {
  const planes = useShardPlanes(config);
  const frameRef = useRef<HTMLIFrameElement>(null);
  const [contentToken, setContentToken] = useState<string | null>(
    () => planes?.contentTokens.peek() ?? null,
  );

  useEffect(() => {
    if (!planes || contentToken) return;
    let alive = true;
    void planes.contentTokens.get().then((t) => {
      if (alive) setContentToken(t);
    });
    return () => {
      alive = false;
    };
  }, [planes, contentToken]);

  // One host per pane, attached for as long as the pane exists — not for as long as it is
  // visible. A hidden shard keeps answering, which is what lets it re-surface with its state
  // rather than rebuilding it.
  useEffect(() => {
    if (!planes) return;
    const host = createBridgeHost({
      pageId: pane.id,
      getWindow: () => frameRef.current?.contentWindow,
      // Read at answer time from the document, so the host never holds a stale copy of
      // something the shell owns.
      getTheme: () => document.documentElement.dataset.theme ?? "dark",
      kv: planes.kv,
      api: planes.api,
      hub: planes.hub,
    });
    host.attach();
    return () => host.detach();
  }, [pane.id, planes]);

  const src = useMemo(() => {
    if (!planes || !contentToken) return null;
    const params = new URLSearchParams({ access_token: contentToken });
    const theme = document.documentElement.dataset.theme;
    // Stamped for a correct FIRST paint; later switches ride the bridge's theme push.
    if (theme) params.set("theme", theme);
    return `${config.apiBaseUrl}/content/${pane.id}/?${params.toString()}`;
  }, [planes, contentToken, config.apiBaseUrl, pane.id]);

  if (!planes) {
    return (
      <ShardNotice title={pane.title}>
        The host did not send an apiBaseUrl, so there is nothing to serve the shard from.
      </ShardNotice>
    );
  }
  if (!src) return <ShardNotice title={pane.title}>Minting a content token…</ShardNotice>;

  return (
    <iframe
      ref={frameRef}
      title={pane.title ?? pane.id}
      src={src}
      sandbox="allow-scripts"
      className="h-full w-full border-0 bg-bg"
    />
  );
}

function ShardNotice({ title, children }: { title?: string; children: ReactNode }) {
  return (
    <div className="flex h-full w-full flex-col items-center justify-center gap-2 px-8 text-center">
      <p className="font-display text-body font-medium text-ink-2">{title ?? "Shard"}</p>
      <p className="font-mono text-small text-ink-4">{children}</p>
    </div>
  );
}

function Embed() {
  const config = window.__ALADIN_EMBED__;

  if (!config?.token || !config?.collabWsUrl) {
    // Loud, not a blank editor that silently never syncs.
    return (
      <div className="flex h-screen w-screen flex-col items-center justify-center gap-2 bg-bg px-8 text-center">
        <p className="font-display text-body font-medium text-ink-2">Not configured</p>
        <p className="font-mono text-small text-ink-4">
          The host must set window.__ALADIN_EMBED__ (token, collabWsUrl, user) before this
          document loads.
        </p>
      </div>
    );
  }

  return <Host config={config} />;
}

const root = document.getElementById("root");
if (root) {
  createRoot(root).render(
    <StrictMode>
      <Embed />
    </StrictMode>,
  );
}

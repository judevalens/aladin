import { StrictMode, useEffect, useState } from "react";
import { createRoot } from "react-dom/client";

import { BlockNotePageEditorDriver } from "@/modules/pages/editor/page-editor-driver";
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
export interface EmbedConfig {
  token: string;
  /** Hocuspocus, e.g. `ws://192.168.1.109:3501` — a device cannot use localhost. */
  collabWsUrl: string;
  user: { name: string; color: string };
}

/**
 * One mounted surface. `kind` decides what renders; the host is deliberately not limited
 * to notes, because the same argument applies to every web-backed surface the companion
 * has — a shard is another thing that would otherwise cost a whole web view to open.
 */
export interface HostPane {
  /** Artifact id. Also the React key, so a pane keeps its state across reorders. */
  id: string;
  kind: "page" | "shard";
  /** `shard` only: the sandboxed bundle URL, already carrying its content token. */
  src?: string;
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
    __ALADIN_EMBED__?: EmbedConfig;
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

  // A shard is an agent-authored app, sandboxed to an opaque origin exactly as on desktop
  // (`allow-scripts`, no `allow-same-origin`), so postMessage is its only way out.
  //
  // NOT YET WIRED: the `bridge/1` host — theme, shard_kv, the manifest-granted workspace
  // plane. Desktop answers those from its repo layer, which does not exist in here. A
  // shard that only paints will render; one that calls the kit will hang into its own 8s
  // timeout, so the frame is deliberately not mounted until a src has been minted.
  if (!pane.src) {
    return (
      <div className="flex h-full w-full flex-col items-center justify-center gap-2 px-8 text-center">
        <p className="font-display text-body font-medium text-ink-2">
          {pane.title ?? "Shard"}
        </p>
        <p className="font-mono text-small text-ink-4">
          Shards need a content token and the bridge planes, which the companion does not
          serve yet.
        </p>
      </div>
    );
  }

  return (
    <iframe
      title={pane.title ?? pane.id}
      src={pane.src}
      sandbox="allow-scripts"
      className="h-full w-full border-0 bg-bg"
    />
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

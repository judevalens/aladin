import { useParams } from "react-router-dom";

import { BlockNotePageEditorDriver } from "@/modules/pages/editor/page-editor-driver";

/**
 * The page editor, mounted standalone for a **native host** to embed in a web view.
 *
 * This is the iPad companion's note editor. Rather than reimplement BlockNote and a Yjs
 * client in Kotlin — weeks of work for a second, diverging implementation of the block
 * model — the companion loads this route in a WKWebView. It gets the real editor: the same
 * schema (including `entityMention` / `artifactRef`), the same Hocuspocus session desktop
 * joins, live collaboration, and agent edits appearing as they land.
 *
 * **Configuration comes from the host, not the URL.** The native side sets
 * `window.__ALADIN_EMBED__` in a document-start user script. A bearer token in a query
 * string would leak into history, logs and any referrer — so it is never put there.
 *
 * The route deliberately sits outside the auth shell: there is no session cookie in a web
 * view, and the editor authenticates to the collab server with the injected token alone.
 */
export interface EmbedConfig {
  token: string;
  /** Hocuspocus, e.g. `ws://192.168.1.109:3501` — the device cannot use localhost. */
  collabWsUrl: string;
  user: { name: string; color: string };
}

declare global {
  interface Window {
    __ALADIN_EMBED__?: EmbedConfig;
  }
}

export function PageEditorEmbed() {
  const { id } = useParams<{ id: string }>();
  const config = window.__ALADIN_EMBED__;

  if (!id) {
    return <EmbedNotice title="No page" detail="The host did not supply a page id." />;
  }
  if (!config?.token || !config?.collabWsUrl) {
    // Loud rather than a blank editor that silently fails to sync: without the
    // handshake there is no way to join the document.
    return (
      <EmbedNotice
        title="Not configured"
        detail="The host must set window.__ALADIN_EMBED__ (token, collabWsUrl, user) before this page loads."
      />
    );
  }

  return (
    <div className="h-screen w-screen overflow-hidden bg-bg">
      <div className="mx-auto flex h-full w-[92%] flex-col">
        <BlockNotePageEditorDriver
          key={id}
          pageId={id}
          collabWsUrl={config.collabWsUrl}
          token={config.token}
          user={config.user}
          // Entity/ref search and projection are omitted deliberately: they run
          // through the desktop repo layer, and the props are optional by design.
          // The editor is fully usable without them; wiring them to the web API is
          // a follow-up, not a prerequisite.
        />
      </div>
    </div>
  );
}

function EmbedNotice({ title, detail }: { title: string; detail: string }) {
  return (
    <div className="flex h-screen w-screen flex-col items-center justify-center gap-2 bg-bg px-8 text-center">
      <p className="font-display text-body font-medium text-ink-2">{title}</p>
      <p className="font-mono text-small text-ink-4">{detail}</p>
    </div>
  );
}

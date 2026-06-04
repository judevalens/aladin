import { useCreateBlockNote } from "@blocknote/react";
import { BlockNoteView } from "@blocknote/shadcn";
import { HocuspocusProvider } from "@hocuspocus/provider";
import { useCallback, useEffect, useRef, useState } from "react";
import { IndexeddbPersistence } from "y-indexeddb";
import * as Y from "yjs";
import "@blocknote/shadcn/style.css";
import "@/modules/pages/editor/agent-presence.css";
import type { BlockAttribution } from "@/repos/pages/page-attribution-repo";

export interface PageEditorDriverProps {
  pageId: string;
  collabWsUrl: string;
  token: string;
  user: { name: string; color: string };
  // Block-level agent presence: which blocks an agent last touched, and a
  // callback to refetch it when the doc changes (so agent edits show up live).
  attribution?: BlockAttribution;
  onContentChange?: () => void;
}

interface CollabResources {
  ydoc: Y.Doc;
  provider: HocuspocusProvider;
  idb: IndexeddbPersistence;
}

// Collaborative BlockNote editor (M8). The editor binds to a Y.Doc that is
// kept in sync two ways:
//   - y-indexeddb: local-first durability (offline edits survive restarts)
//   - HocuspocusProvider: real-time sync with the collab server + other clients
//
// Content is no longer loaded/saved through the API (M7 path) — Yjs owns it.
// The Y.Doc hydrates the editor; edits stream to both providers automatically.
//
// IMPORTANT: the parent must mount this with `key={pageId}` so a page switch
// fully remounts the component — resources are created once per mount in the
// useState initializer and torn down in the useEffect cleanup.
export function BlockNotePageEditorDriver({
  pageId,
  collabWsUrl,
  token,
  user,
  attribution,
  onContentChange,
}: PageEditorDriverProps) {
  // Created once per mount (key={pageId} guarantees one page per mount).
  const [resources] = useState<CollabResources>(() => {
    const ydoc = new Y.Doc();
    const idb = new IndexeddbPersistence(`page-${pageId}`, ydoc);
    const provider = new HocuspocusProvider({
      url: collabWsUrl,
      name: pageId,
      document: ydoc,
      token,
    });
    return { ydoc, provider, idb };
  });

  useEffect(() => {
    const { ydoc, provider, idb } = resources;
    return () => {
      provider.destroy();
      void idb.destroy();
      ydoc.destroy();
    };
  }, [resources]);

  const editor = useCreateBlockNote({
    collaboration: {
      // BlockNote only reads `.awareness` off the provider (document sync
      // rides the fragment). HocuspocusProvider.awareness is typed
      // `Awareness | null` but is always an Awareness instance at runtime;
      // assert non-null to bridge BlockNote's `| undefined` type.
      provider: resources.provider as HocuspocusProvider & {
        awareness: NonNullable<HocuspocusProvider["awareness"]>;
      },
      fragment: resources.ydoc.getXmlFragment("document"),
      user,
    },
  });

  const containerRef = useRef<HTMLDivElement>(null);

  // Paint the per-block agent markers: set `data-agent-by` + a native title on
  // each block element BlockNote renders with a matching `data-id`. The CSS
  // (agent-presence.css) draws a layout-safe left accent from the attribute.
  const paintMarkers = useCallback(() => {
    const root = containerRef.current;
    if (!root) return;
    const escape = (s: string) =>
      typeof CSS !== "undefined" && CSS.escape ? CSS.escape(s) : s;
    root.querySelectorAll<HTMLElement>("[data-agent-by]").forEach((el) => {
      el.removeAttribute("data-agent-by");
      el.removeAttribute("title");
    });
    for (const [id, info] of Object.entries(attribution ?? {})) {
      const el = root.querySelector<HTMLElement>(`[data-id="${escape(id)}"]`);
      if (el) {
        el.setAttribute("data-agent-by", info.by);
        el.setAttribute("title", `✦ Edited by ${info.by}`);
      }
    }
  }, [attribution]);

  // Repaint when attribution changes.
  useEffect(() => {
    paintMarkers();
  }, [paintMarkers]);

  // BlockNote re-renders blocks on edit (local OR remote/agent via Yjs), which
  // wipes the attributes — so observe the editor DOM, repaint, and debounce a
  // refetch so a peer agent's edit surfaces shortly after it lands.
  useEffect(() => {
    const root = containerRef.current;
    if (!root) return;
    let timer: number | undefined;
    const observer = new MutationObserver(() => {
      paintMarkers();
      if (onContentChange) {
        window.clearTimeout(timer);
        timer = window.setTimeout(onContentChange, 800);
      }
    });
    observer.observe(root, { childList: true, subtree: true });
    return () => {
      observer.disconnect();
      window.clearTimeout(timer);
    };
  }, [paintMarkers, onContentChange]);

  return (
    <div className="relative flex min-h-0 flex-1 flex-col">
      <div ref={containerRef} className="min-h-0 flex-1 overflow-y-auto px-8 py-4">
        <BlockNoteView
          editor={editor}
          slashMenu={true}
          formattingToolbar={true}
          linkToolbar={true}
          theme="light"
        />
      </div>
    </div>
  );
}

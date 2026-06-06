import { useCreateBlockNote } from "@blocknote/react";
import { BlockNoteView } from "@blocknote/shadcn";
import { HocuspocusProvider } from "@hocuspocus/provider";
import { useEffect, useState } from "react";
import { IndexeddbPersistence } from "y-indexeddb";
import * as Y from "yjs";
import "@blocknote/shadcn/style.css";

export interface PageEditorDriverProps {
  pageId: string;
  collabWsUrl: string;
  token: string;
  user: { name: string; color: string };
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

  return (
    <div className="relative flex min-h-0 flex-1 flex-col">
      <div className="min-h-0 flex-1 overflow-y-auto px-8 py-4">
        <BlockNoteView
          editor={editor}
          slashMenu={true}
          formattingToolbar={true}
          linkToolbar={true}
          theme="dark"
        />
      </div>
    </div>
  );
}

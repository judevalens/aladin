import { useCreateBlockNote, SuggestionMenuController } from "@blocknote/react";
import type { DefaultReactSuggestionItem } from "@blocknote/react";
import { BlockNoteView } from "@blocknote/shadcn";
import { HocuspocusProvider } from "@hocuspocus/provider";
import { useEffect, useRef, useState } from "react";
import { IndexeddbPersistence } from "y-indexeddb";
import * as Y from "yjs";
import "@blocknote/shadcn/style.css";

import type { EntityHit, MentionRef } from "@/modules/graph/graph-pane-types";
import {
  editorPlainText,
  extractEntityMentions,
  pageSchema,
} from "@/modules/pages/editor/entity-mention";

export interface PageEditorDriverProps {
  pageId: string;
  collabWsUrl: string;
  token: string;
  user: { name: string; color: string };
  // Entity @-mentions (P2). Optional so the editor still works standalone.
  searchEntities?: (query: string) => Promise<EntityHit[]>;
  createEntity?: (name: string) => Promise<EntityHit>;
  onMentionsChange?: (mentions: MentionRef[]) => void;
  // Exposes a stable getter for the page's plain text (authored claim extraction, P3).
  onReady?: (api: { getPlainText: () => string }) => void;
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
  searchEntities,
  createEntity,
  onMentionsChange,
  onReady,
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
    schema: pageSchema,
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

  // Expose a stable plain-text getter for authored extraction (read at click time).
  useEffect(() => {
    onReady?.({ getPlainText: () => editorPlainText(editor.document) });
  }, [editor, onReady]);

  // Project @entity mentions out of the doc into the backend, debounced. Keyed off the
  // local editor changes — enough to keep this client's edits in sync with the graph.
  const flushTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => {
    if (!onMentionsChange) return;
    const flush = () => onMentionsChange(extractEntityMentions(editor.document));
    const schedule = () => {
      if (flushTimer.current) clearTimeout(flushTimer.current);
      flushTimer.current = setTimeout(flush, 700);
    };
    const unsubscribe = editor.onChange ? editor.onChange(schedule) : undefined;
    return () => {
      if (flushTimer.current) clearTimeout(flushTimer.current);
      if (typeof unsubscribe === "function") unsubscribe();
    };
  }, [editor, onMentionsChange]);

  async function mentionItems(query: string): Promise<DefaultReactSuggestionItem[]> {
    if (!searchEntities) return [];
    const insert = (hit: EntityHit) => {
      editor.insertInlineContent([
        { type: "entityMention", props: { entityId: hit.id, label: hit.name, kind: hit.kind } },
        " ",
      ]);
    };
    const hits = await searchEntities(query);
    const items: DefaultReactSuggestionItem[] = hits.map((hit) => ({
      title: hit.name,
      subtext: hit.kind || "entity",
      onItemClick: () => insert(hit),
    }));
    const q = query.trim();
    if (q && createEntity && !hits.some((h) => h.name.toLowerCase() === q.toLowerCase())) {
      items.push({
        title: `Create “${q}”`,
        subtext: "new entity",
        onItemClick: async () => {
          const created = await createEntity(q);
          insert(created);
        },
      });
    }
    return items;
  }

  return (
    <div className="relative flex min-h-0 flex-1 flex-col">
      <div className="min-h-0 flex-1 overflow-y-auto px-8 py-4">
        <BlockNoteView
          editor={editor}
          slashMenu={true}
          formattingToolbar={true}
          linkToolbar={true}
          theme="dark"
        >
          {searchEntities ? (
            <SuggestionMenuController triggerCharacter="@" getItems={mentionItems} />
          ) : null}
        </BlockNoteView>
      </div>
    </div>
  );
}

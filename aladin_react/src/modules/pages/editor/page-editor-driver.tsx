import { useCreateBlockNote, SuggestionMenuController } from "@blocknote/react";
import type { DefaultReactSuggestionItem } from "@blocknote/react";
import { BlockNoteView } from "@blocknote/shadcn";
import { HocuspocusProvider } from "@hocuspocus/provider";
import { useEffect, useRef, useState } from "react";
import { IndexeddbPersistence } from "y-indexeddb";
import * as Y from "yjs";
import "@blocknote/shadcn/style.css";

import type {
  ArtifactRef,
  EntityHit,
  MentionRef,
  RefHit,
} from "@/modules/graph/graph-pane-types";
import {
  extractEntityMentions,
  pageSchema,
} from "@/modules/pages/editor/entity-mention";
import { extractArtifactRefs } from "@/modules/pages/editor/ref-mention";

export interface PageEditorDriverProps {
  pageId: string;
  collabWsUrl: string;
  token: string;
  user: { name: string; color: string };
  // Entity @-mentions (P2). Optional so the editor still works standalone.
  searchEntities?: (query: string) => Promise<EntityHit[]>;
  createEntity?: (name: string) => Promise<EntityHit>;
  onMentionsChange?: (mentions: MentionRef[]) => void;
  // `#` cross-references to claims/pages/shards (Y2). Optional.
  searchRefs?: (query: string) => Promise<RefHit[]>;
  onRefsChange?: (refs: ArtifactRef[]) => void;
  // Create-on-link: mint a new page for an unmatched `#` query and return it for the chip.
  // Desktop-only (page creation goes through the local browser store) — omit on web.
  createPageRef?: (title: string) => Promise<{ id: string; label: string } | null>;
  // Navigate to a referenced page/shard when its chip is clicked.
  onOpenArtifact?: (artifactId: string) => void;
  // Navigate to a mentioned entity's context when its @chip is clicked.
  onOpenEntity?: (entityId: string) => void;
}

const refGroupLabel: Record<string, string> = {
  claim: "Claims",
  page: "Pages",
  shard: "Shards",
};

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
  searchRefs,
  onRefsChange,
  createPageRef,
  onOpenArtifact,
  onOpenEntity,
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

  // Project @entity mentions + `#` refs out of the doc into the backend, debounced. Keyed
  // off the local editor changes — enough to keep this client's edits in sync with the graph.
  const flushTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => {
    if (!onMentionsChange && !onRefsChange) return;
    const flush = () => {
      if (onMentionsChange) onMentionsChange(extractEntityMentions(editor.document));
      if (onRefsChange) onRefsChange(extractArtifactRefs(editor.document));
    };
    const schedule = () => {
      if (flushTimer.current) clearTimeout(flushTimer.current);
      flushTimer.current = setTimeout(flush, 700);
    };
    const unsubscribe = editor.onChange ? editor.onChange(schedule) : undefined;
    return () => {
      if (flushTimer.current) clearTimeout(flushTimer.current);
      if (typeof unsubscribe === "function") unsubscribe();
    };
  }, [editor, onMentionsChange, onRefsChange]);

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

  // `#` picker: unified search across claims + pages + shards, sectioned by kind.
  async function refItems(query: string): Promise<DefaultReactSuggestionItem[]> {
    if (!searchRefs) return [];
    const hits = await searchRefs(query);
    const items: DefaultReactSuggestionItem[] = hits.map((hit) => ({
      title: hit.label,
      subtext: hit.kind === "claim" ? hit.detail || "claim" : hit.kind,
      group: refGroupLabel[hit.kind] ?? hit.kind,
      onItemClick: () => {
        editor.insertInlineContent([
          {
            type: "artifactRef",
            props: {
              kind: hit.kind,
              targetId: hit.id,
              label: hit.label,
              polarity: hit.kind === "claim" ? hit.detail ?? "" : "",
            },
          },
          " ",
        ]);
      },
    }));
    // Create-on-link (desktop only): offer to mint a page for an unmatched query, so a
    // `#` to a page you haven't written yet just works — the chip is inserted only after
    // the artifact exists, keeping the "refs never dangle" invariant.
    const q = query.trim();
    if (
      q &&
      createPageRef &&
      !hits.some((h) => h.kind === "page" && h.label.toLowerCase() === q.toLowerCase())
    ) {
      items.push({
        title: `Create page “${q}”`,
        subtext: "new page",
        group: refGroupLabel.page,
        onItemClick: async () => {
          const created = await createPageRef(q);
          if (!created) return;
          editor.insertInlineContent([
            {
              type: "artifactRef",
              props: { kind: "page", targetId: created.id, label: created.label, polarity: "" },
            },
            " ",
          ]);
        },
      });
    }
    return items;
  }

  // Delegated click: an @entity mention opens its context; a page/shard ref chip opens that
  // artifact in the workspace. Both are keyed off the inline node's data-* attributes.
  function onChipClick(e: React.MouseEvent<HTMLDivElement>) {
    const target = e.target as HTMLElement;

    const entityEl = target.closest("[data-entity-id]");
    if (entityEl) {
      const entityId = entityEl.getAttribute("data-entity-id");
      if (entityId && onOpenEntity) {
        e.preventDefault();
        onOpenEntity(entityId);
      }
      return;
    }

    if (!onOpenArtifact) return;
    const el = target.closest("[data-ref-target]");
    if (!el) return;
    const kind = el.getAttribute("data-ref-kind");
    const refTarget = el.getAttribute("data-ref-target");
    if (refTarget && (kind === "page" || kind === "shard")) {
      e.preventDefault();
      onOpenArtifact(refTarget);
    }
  }

  return (
    <div className="relative flex min-h-0 flex-1 flex-col">
      <div className="min-h-0 flex-1 overflow-y-auto px-8 py-4" onClick={onChipClick}>
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
          {searchRefs ? (
            <SuggestionMenuController triggerCharacter="#" getItems={refItems} />
          ) : null}
        </BlockNoteView>
      </div>
    </div>
  );
}

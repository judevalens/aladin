import { useCreateBlockNote, SuggestionMenuController } from "@blocknote/react";
import type { DefaultReactSuggestionItem } from "@blocknote/react";
import { BlockNoteView } from "@blocknote/shadcn";
import { HocuspocusProvider } from "@hocuspocus/provider";
import { useCallback, useEffect, useRef, useState } from "react";
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
  page: "Pages",
  shard: "Shards",
};

interface CollabResources {
  ydoc: Y.Doc;
  provider: HocuspocusProvider;
  idb: IndexeddbPersistence;
}

// blockHasChip reports whether a block's inline content holds an @entity / # chip. Used to gate
// projection: a change can only move the mention/ref set if a block it touched carries a chip.
function blockHasChip(block: unknown): boolean {
  const content = (block as { content?: unknown } | null)?.content;
  if (!Array.isArray(content)) return false;
  return content.some((ic) => {
    const type = (ic as { type?: string } | null)?.type;
    return type === "entityMention" || type === "artifactRef";
  });
}

// chipKeys is a stable, order-independent identity of a block's chips (entity ids + ref targets).
// Comparing a change's block vs prevBlock tells a STRUCTURAL edit (a chip added/removed → the set
// changed) apart from an incidental one (same chips, surrounding text moved → only snippets drift).
function chipKeys(block: unknown): string {
  const content = (block as { content?: unknown } | null)?.content;
  if (!Array.isArray(content)) return "";
  const keys: string[] = [];
  for (const ic of content) {
    const item = ic as { type?: string; props?: { entityId?: string; targetId?: string } } | null;
    if (item?.type === "entityMention") keys.push(`e:${item.props?.entityId ?? ""}`);
    else if (item?.type === "artifactRef") keys.push(`r:${item.props?.targetId ?? ""}`);
  }
  return keys.sort().join("|");
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

  // Project @entity mentions + `#` refs out of the doc into the backend — event-based. Menu
  // inserts flush directly (see mentionItems/refItems → flush() right after insertInlineContent).
  // Everything else comes through onChange, which BlockNote hands a block-level change list: we
  // (a) GATE on it — skip entirely unless a change touched a chip-bearing block (so plain prose
  // typing does no work), and (b) classify — a STRUCTURAL change (chip added/removed, by comparing
  // block vs prevBlock chip identity) or any non-local edit flushes immediately; only incidental
  // local edits (same chips, surrounding text moved) are debounced. So adding AND deleting a chip
  // are both instant. The full-doc walk in flush() still builds the set (ReplaceMentions is a
  // full-set replace); getChanges only decides whether/when to run it. Every path stays idempotent.
  const flushTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Last-projected mentions/refs (serialized) — only sync when they actually change, so
  // typing near/around a mention (or plain text) doesn't fire a no-op write + node frame,
  // which would otherwise make reactive views (the graph pane) refetch on every pause.
  const lastMentions = useRef<string | null>(null);
  const lastRefs = useRef<string | null>(null);
  const flush = useCallback(() => {
    if (onMentionsChange) {
      const mentions = extractEntityMentions(editor.document);
      const key = JSON.stringify(mentions);
      if (key !== lastMentions.current) {
        lastMentions.current = key;
        onMentionsChange(mentions);
      }
    }
    if (onRefsChange) {
      const refs = extractArtifactRefs(editor.document);
      const key = JSON.stringify(refs);
      if (key !== lastRefs.current) {
        lastRefs.current = key;
        onRefsChange(refs);
      }
    }
  }, [editor, onMentionsChange, onRefsChange]);
  useEffect(() => {
    if (!onMentionsChange && !onRefsChange) return;
    const handleChange = (
      _editor: typeof editor,
      ctx: {
        getChanges: () => Array<{
          block?: unknown;
          prevBlock?: unknown;
          source: { type: string };
        }>;
      },
    ) => {
      const changes = ctx.getChanges();
      // Gate: only edits that touch a chip-bearing block (before or after) can move the set.
      // Prose typing in a chip-free block is a no-op here — no walk, no timer.
      const touchesChip = changes.some(
        (c) => blockHasChip(c.block) || blockHasChip(c.prevBlock),
      );
      if (!touchesChip) return;
      if (flushTimer.current) clearTimeout(flushTimer.current);
      // Structural (a chip added OR removed) → project now, even when local: an insert/delete is
      // a discrete event, not a burst. Only INCIDENTAL local edits (same chips, text moved around
      // them → snippets drift) are debounced. Paste/drop/undo-redo/remote are always immediate.
      const structural = changes.some((c) => chipKeys(c.block) !== chipKeys(c.prevBlock));
      const immediate = structural || changes.some((c) => c.source.type !== "local");
      if (immediate) {
        flush();
      } else {
        flushTimer.current = setTimeout(flush, 700);
      }
    };
    const unsubscribe = editor.onChange ? editor.onChange(handleChange) : undefined;
    return () => {
      if (flushTimer.current) clearTimeout(flushTimer.current);
      if (typeof unsubscribe === "function") unsubscribe();
    };
  }, [editor, onMentionsChange, onRefsChange, flush]);

  async function mentionItems(query: string): Promise<DefaultReactSuggestionItem[]> {
    if (!searchEntities) return [];
    const insert = (hit: EntityHit) => {
      editor.insertInlineContent([
        { type: "entityMention", props: { entityId: hit.id, label: hit.name, kind: hit.kind } },
        " ",
      ]);
      flush(); // event-based: project the new mention now, not on the onChange debounce
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

  // `#` picker: unified search across pages + shards, sectioned by kind.
  async function refItems(query: string): Promise<DefaultReactSuggestionItem[]> {
    if (!searchRefs) return [];
    const hits = await searchRefs(query);
    const items: DefaultReactSuggestionItem[] = hits.map((hit) => ({
      title: hit.label,
      subtext: hit.kind,
      group: refGroupLabel[hit.kind] ?? hit.kind,
      onItemClick: () => {
        editor.insertInlineContent([
          {
            type: "artifactRef",
            props: {
              kind: hit.kind,
              targetId: hit.id,
              label: hit.label,
              polarity: "",
            },
          },
          " ",
        ]);
        flush(); // event-based: project the new ref now
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
          flush(); // event-based: project the new ref now
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

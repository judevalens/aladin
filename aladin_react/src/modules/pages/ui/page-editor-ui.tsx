import { History } from "lucide-react";
import { Icon } from "@/components/ui/icon";
import { useState } from "react";
import { useAppComposition } from "@/app/composition/app-composition";
import { useAppStore } from "@/app/state/store";
import { EntityPeek } from "@/modules/entities/ui/entity-peek-ui";
import { BlockNotePageEditorDriver } from "@/modules/pages/editor/page-editor-driver";
import { PageHistoryPanel } from "@/modules/pages/ui/page-history-panel";
import type { BrowserTreeNode } from "@/shared/api/models";
import { useObservableState } from "@/shared/flow/use-observable-state";

// Stable, readable cursor color per user id (awareness).
function colorForUser(id: string): string {
  let hash = 0;
  for (let i = 0; i < id.length; i += 1) {
    hash = (hash * 31 + id.charCodeAt(i)) | 0;
  }
  const hue = Math.abs(hash) % 360;
  return `hsl(${hue} 70% 45%)`;
}

// The folder containing the given page in the browser tree.
// Returns null for "at root", undefined for "not found".
function folderOfPage(nodes: BrowserTreeNode[], pageId: string): string | null | undefined {
  for (const node of nodes) {
    if (node.artifactId === pageId) return node.parentId ?? null;
    const found = folderOfPage(node.children ?? [], pageId);
    if (found !== undefined) return found;
  }
  return undefined;
}

export function PageEditorUI({ pageId }: { pageId: string }) {
  const { runtime, services, repos } = useAppComposition();
  const authState = useObservableState(services.auth.session.session());
  // Hooks before the early return (rules of hooks).
  const [historyOpen, setHistoryOpen] = useState(false);
  const [peekEntityId, setPeekEntityId] = useState<string | null>(null);
  const openArtifact = useAppStore((s) => s.openArtifact);

  const user = authState.status === "data" ? authState.value.user : null;
  if (!user) {
    return (
      <div className="px-7 py-6 text-body text-ink-2">Loading page…</div>
    );
  }

  const token = runtime.desktopSession.getToken() ?? "";

  return (
    <div className="relative flex h-full min-h-0 flex-col">
      <div className="absolute right-3 top-2 z-10 flex items-center gap-2">
        <button
          onClick={() => setHistoryOpen((v) => !v)}
          title="Edit history"
          className="flex items-center gap-1.5 rounded-control border border-line bg-field/90 px-2.5 py-1 text-small font-medium text-ink-2 backdrop-blur-sm hover:bg-raise hover:text-ink"
        >
          <Icon as={History} size="inline" mark />
          History
        </button>
      </div>
      <div className="min-h-0 flex-1 overflow-hidden">
        {/* No reading-measure cap here on purpose. Before the design-system pass this class
            had no token behind it, so the editor had always filled the pane — and that is
            what it should do: BlockNote carries its own gutters for drag handles and the
            slash menu, so an outer cap stacks on top of them and pinches the text twice.
            The measure token still applies to the card-style artifact panes. */}
        <div className="mx-auto flex h-full w-full flex-col">
          <BlockNotePageEditorDriver
            key={pageId}
            pageId={pageId}
            collabWsUrl={runtime.config.collabWsBaseUrl}
            token={token}
            user={{ name: user.email, color: colorForUser(user.id) }}
            searchEntities={(q) => repos.graphPane.searchEntities(q)}
            createEntity={(name) => repos.graphPane.createEntity(name, "other")}
            onMentionsChange={(mentions) =>
              void repos.graphPane.syncMentions(pageId, mentions).catch(() => undefined)
            }
            searchRefs={(q) => repos.graphPane.searchRefs(q)}
            onRefsChange={(refs) =>
              void repos.graphPane.syncRefs(pageId, refs).catch(() => undefined)
            }
            createPageRef={
              // Page creation flows through the local browser store — desktop only.
              runtime.config.isDesktopApp
                ? async (title) => {
                    try {
                      const tree = await repos.workspace.getLocalNodeTree();
                      const artifact = await services.workspace.createArtifact({
                        type: "page",
                        folderId: folderOfPage(tree, pageId) ?? null,
                        title,
                        content: "",
                      });
                      return { id: artifact.id, label: artifact.title || title };
                    } catch {
                      return null;
                    }
                  }
                : undefined
            }
            onOpenArtifact={(artifactId) => openArtifact(artifactId)}
            onOpenEntity={(entityId) => setPeekEntityId(entityId)}
          />
        </div>
      </div>
      {historyOpen ? (
        <PageHistoryPanel pageId={pageId} onClose={() => setHistoryOpen(false)} />
      ) : null}
      {/* Clicking an inline @entity opens its context in a peek modal — no navigating away. */}
      <EntityPeek entityId={peekEntityId} onClose={() => setPeekEntityId(null)} />
    </div>
  );
}

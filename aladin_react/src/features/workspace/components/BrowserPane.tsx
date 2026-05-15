import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  ChevronDown,
  ChevronRight,
  Ellipsis,
} from "lucide-react";
import { useMemo } from "react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { ScrollArea } from "@/components/ui/scroll-area";
import { AladinPanel } from "@/components/ui/aladin";
import {
  ancestorFolderIds,
  buildBrowserRows,
  buildFolderBreadcrumbs,
  nextArtifactTitle,
  nextFolderTitle,
} from "@/features/workspace/browser-tree";
import { browserTreeQueryKey, useBrowserTreeQuery } from "@/features/workspace/queries";
import { useWorkspaceShell } from "@/features/workspace/workspace-state";
import { api, toArtifact } from "@/shared/api/client";
import { cn } from "@/shared/lib/utils";

interface BrowserPaneProps {
  activeArtifactId: string | null;
}

export function BrowserPane({ activeArtifactId }: BrowserPaneProps) {
  const queryClient = useQueryClient();
  const browserTreeQuery = useBrowserTreeQuery();
  const { state, dispatch } = useWorkspaceShell();

  const rows = useMemo(
    () =>
      buildBrowserRows(
        browserTreeQuery.data ?? [],
        state.scopeFolderId,
        state.expandedFolderIds,
      ),
    [browserTreeQuery.data, state.scopeFolderId, state.expandedFolderIds],
  );

  const scopeBreadcrumbs = useMemo(
    () => buildFolderBreadcrumbs(browserTreeQuery.data ?? [], state.scopeFolderId),
    [browserTreeQuery.data, state.scopeFolderId],
  );

  const createFolderMutation = useMutation({
    mutationFn: async (parentId: string | null) =>
      api.createFolder({
        parentId,
        title: nextFolderTitle(browserTreeQuery.data ?? [], parentId),
      }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: browserTreeQueryKey });
    },
  });

  const createArtifactMutation = useMutation({
    mutationFn: async ({ folderId, kind }: { folderId: string | null; kind: "note" | "link" }) =>
      api.createUserArtifact({
        type: kind === "note" ? "page" : "link",
        folderId,
        title: nextArtifactTitle(browserTreeQuery.data ?? [], folderId, kind),
        content: kind === "note" ? "New artifact" : "",
        sourceUrl: kind === "link" ? "https://example.com" : undefined,
      }),
    onSuccess: async (record) => {
      await queryClient.invalidateQueries({ queryKey: browserTreeQueryKey });
      dispatch({ type: "open-artifact", artifactId: toArtifact(record).id });
    },
  });

  if (browserTreeQuery.isLoading) {
    return (
      <section className="flex w-[352px] flex-col bg-white">
        <div className="p-4 text-sm text-gray-700">Loading browser tree…</div>
      </section>
    );
  }

  if (browserTreeQuery.error) {
    return (
      <section className="flex w-[352px] flex-col bg-white p-4">
        <AladinPanel className="p-4 text-sm text-red-700">
          {browserTreeQuery.error instanceof Error
            ? browserTreeQuery.error.message
            : "Failed to load browser tree."}
        </AladinPanel>
      </section>
    );
  }

  return (
    <section className="flex w-[352px] flex-col bg-white">
      <div className="space-y-3 border-b border-gray-300 px-4 py-4">
        <div className="flex items-center gap-2">
          <Button
            variant="secondary"
            size="sm"
            disabled={state.scopeBackStack.length === 0}
            onClick={() => dispatch({ type: "navigate-scope-back" })}
          >
            Back
          </Button>
          <div className="min-w-0 flex-1 truncate text-xs text-gray-500">
            {scopeBreadcrumbs.length > 0
              ? scopeBreadcrumbs.map((crumb) => crumb.label).join(" / ")
              : "All items"}
          </div>
        </div>
      </div>
      <ScrollArea className="min-h-0 flex-1">
        <div className="flex flex-col px-2 py-3">
          {rows.map((row) => {
            const isActiveArtifact = row.artifactId === activeArtifactId;
            const isExpanded = row.folderId ? state.expandedFolderIds.includes(row.folderId) : false;
            return (
              <div key={row.id} className="group">
                <div
                  className={cn(
                    "flex items-center gap-2 px-2 py-1.5 text-sm",
                    isActiveArtifact
                      ? "bg-gray-100 text-black"
                      : "text-gray-700 hover:bg-gray-100 hover:text-black",
                  )}
                  style={{ paddingLeft: `${row.depth * 16 + 8}px` }}
                >
                  {row.kind === "folder" ? (
                    <button
                      className="flex h-6 w-6 items-center justify-center hover:bg-gray-100"
                      onClick={() => {
                        if (row.scopeCandidate && row.folderId) {
                          dispatch({
                            type: "navigate-scope",
                            folderId: row.folderId,
                            pushCurrent: true,
                            ancestorFolderIds: ancestorFolderIds(browserTreeQuery.data ?? [], row.folderId),
                          });
                        } else if (row.folderId) {
                          dispatch({ type: "toggle-folder", folderId: row.folderId });
                        }
                      }}
                      type="button"
                    >
                      {isExpanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
                    </button>
                  ) : (
                    <span className="w-6" />
                  )}
                  <button
                    className="min-w-0 flex-1 truncate text-left"
                    onClick={() => {
                      if (row.kind === "artifact" && row.artifactId) {
                        dispatch({ type: "open-artifact", artifactId: row.artifactId });
                      } else if (row.folderId) {
                        dispatch({ type: "set-focused-folder", folderId: row.folderId });
                      }
                    }}
                    type="button"
                  >
                    {row.title}
                  </button>
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <button
                        className="flex h-7 w-7 items-center justify-center text-gray-500 opacity-0 transition-opacity hover:bg-gray-100 hover:text-black group-hover:opacity-100"
                        type="button"
                      >
                        <Ellipsis className="h-4 w-4" />
                      </button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end" className="w-52">
                      {row.kind === "folder" && row.folderId ? (
                        <>
                          <DropdownMenuItem
                            onClick={() =>
                              dispatch({
                                type: "start-rename",
                                draft: {
                                  kind: "folder",
                                  rowId: row.folderId!,
                                  originalTitle: row.title,
                                  draftTitle: row.title,
                                },
                              })
                            }
                          >
                            Rename folder
                          </DropdownMenuItem>
                          <DropdownMenuItem
                            onClick={() => createFolderMutation.mutate(row.folderId!)}
                          >
                            New folder here
                          </DropdownMenuItem>
                          <DropdownMenuItem
                            onClick={() =>
                              createArtifactMutation.mutate({
                                folderId: row.folderId!,
                                kind: "note",
                              })
                            }
                          >
                            New note here
                          </DropdownMenuItem>
                        </>
                      ) : null}
                      {row.kind === "artifact" && row.artifactId ? (
                        <>
                          <DropdownMenuItem
                            onClick={() => dispatch({ type: "open-artifact", artifactId: row.artifactId! })}
                          >
                            Open artifact
                          </DropdownMenuItem>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem
                            onClick={() =>
                              dispatch({
                                type: "start-rename",
                                draft: {
                                  kind: "artifact",
                                  rowId: row.artifactId!,
                                  originalTitle: row.title,
                                  draftTitle: row.title,
                                },
                              })
                            }
                          >
                            Rename artifact
                          </DropdownMenuItem>
                        </>
                      ) : null}
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>
              </div>
            );
          })}
          {rows.length === 0 ? (
            <div className="px-3 py-6 text-sm text-gray-500">
              Nothing in this section yet.
            </div>
          ) : null}
        </div>
      </ScrollArea>
    </section>
  );
}

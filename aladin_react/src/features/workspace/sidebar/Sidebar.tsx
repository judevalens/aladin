import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  AudioLines,
  Folder,
  GitGraph,
  Home,
  Network,
  LogOut,
  NotebookText,
  Plus,
  Signal,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { AladinToolbarField } from "@/components/ui/aladin";
import { useBrowserTreeQuery, browserTreeQueryKey } from "@/features/workspace/queries";
import { nextArtifactTitle, nextFolderTitle } from "@/features/workspace/browser-tree";
import { useWorkspaceShell } from "@/features/workspace/workspace-state";
import { api, toArtifact, toFolderNode } from "@/shared/api/client";

interface SidebarProps {
  selectedDestination: "home" | "folders" | "sources" | "signals" | "graph";
  userEmail: string;
  onNavigate: (path: string) => void;
  onLogout: () => void;
  logoutPending: boolean;
}

const navItems = [
  { key: "home", label: "Home", icon: Home, path: "/home" },
  { key: "folders", label: "Folders", icon: Folder, path: "/folders" },
  { key: "signals", label: "Signals", icon: Signal, path: "/signals" },
  { key: "sources", label: "Sources", icon: Network, path: "/sources" },
  { key: "graph", label: "Graph", icon: GitGraph, path: "/graph" },
] as const;

export function Sidebar({
  selectedDestination,
  userEmail,
  onNavigate,
  onLogout,
  logoutPending,
}: SidebarProps) {
  const queryClient = useQueryClient();
  const browserTreeQuery = useBrowserTreeQuery();
  const { state, dispatch } = useWorkspaceShell();

  const createFolderMutation = useMutation({
    mutationFn: async () => {
      const tree = browserTreeQuery.data ?? [];
      const parentId = state.focusedFolderId ?? state.scopeFolderId;
      return api.createFolder({
        title: nextFolderTitle(tree, parentId),
        parentId,
      });
    },
    onSuccess: async (record) => {
      await queryClient.invalidateQueries({ queryKey: browserTreeQueryKey });
      dispatch({ type: "set-focused-folder", folderId: toFolderNode(record).id });
    },
  });

  const createArtifactMutation = useMutation({
    mutationFn: async (kind: "note" | "link") => {
      const tree = browserTreeQuery.data ?? [];
      const folderId = state.focusedFolderId ?? state.scopeFolderId;
      const title = nextArtifactTitle(tree, folderId, kind);
      return api.createUserArtifact({
        type: kind === "note" ? "page" : "link",
        folderId,
        title,
        content: kind === "note" ? "New artifact" : "",
        sourceUrl: kind === "link" ? "https://example.com" : undefined,
      });
    },
    onSuccess: async (record) => {
      await queryClient.invalidateQueries({ queryKey: browserTreeQueryKey });
      dispatch({ type: "open-artifact", artifactId: toArtifact(record).id });
    },
  });

  const openVoiceDraft = () => {
    const tree = browserTreeQuery.data ?? [];
    const folderId = state.focusedFolderId ?? state.scopeFolderId;
    dispatch({
      type: "open-voice-draft",
      draft: {
        folderId,
        title: nextArtifactTitle(tree, folderId, "voice"),
        description: "",
        phase: "ready",
        errorMessage: null,
      },
    });
  };

  return (
    <aside className="flex h-full w-sidebar flex-col gap-3 bg-aladin-panel-muted px-3.5 py-3.5">
      <div className="flex h-9 items-center gap-2.5">
        <div className="flex h-7 w-7 items-center justify-center rounded-sharp border border-aladin-ink bg-aladin-panel text-sm font-bold text-aladin-ink">
          A
        </div>
        <div className="space-y-0.5">
          <div className="text-sm font-semibold text-aladin-ink">Aladin</div>
          <div className="font-mono text-xs text-aladin-ink-muted">workspace</div>
        </div>
      </div>

      <AladinToolbarField text="Search or jump..." />

      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="secondary" className="h-10 justify-between border-aladin-border bg-aladin-panel text-aladin-ink">
            <span>Create</span>
            <Plus className="h-4 w-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-52">
          <DropdownMenuLabel>Create</DropdownMenuLabel>
          <DropdownMenuSeparator />
          <DropdownMenuItem onClick={() => createFolderMutation.mutate()}>
            <Folder className="mr-2 h-4 w-4" />
            New folder
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => createArtifactMutation.mutate("note")}>
            <NotebookText className="mr-2 h-4 w-4" />
            New note
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => createArtifactMutation.mutate("link")}>
            <Network className="mr-2 h-4 w-4" />
            New link
          </DropdownMenuItem>
          <DropdownMenuItem onClick={openVoiceDraft}>
            <AudioLines className="mr-2 h-4 w-4" />
            New voice
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <div className="mx-1 h-px bg-aladin-divider" />

      <nav className="flex flex-col gap-1">
        {navItems.map((item) => {
          const Icon = item.icon;
          const active = item.key === selectedDestination;
          return (
            <button
              key={item.key}
              className={`flex h-[38px] items-center gap-3 rounded-control px-3 text-sm transition-colors ${
                active
                  ? "bg-aladin-panel text-aladin-ink"
                  : "text-aladin-ink-muted hover:bg-aladin-control-hover hover:text-aladin-ink"
              }`}
              onClick={() => onNavigate(item.path)}
              type="button"
            >
              <Icon className="h-[18px] w-[18px]" />
              <span>{item.label}</span>
            </button>
          );
        })}
      </nav>

      <div className="mt-auto space-y-2 px-2 py-1">
        <div className="font-mono text-[11px] uppercase tracking-[0.2em] text-aladin-ink-muted">
          signed in
        </div>
        <div className="truncate text-sm text-aladin-ink-secondary">{userEmail}</div>
      </div>

      <button
        className="flex h-[38px] items-center gap-3 rounded-control px-3 text-sm text-aladin-ink-muted transition-colors hover:bg-aladin-control-hover hover:text-aladin-ink disabled:opacity-50"
        onClick={onLogout}
        type="button"
        disabled={logoutPending}
      >
        <LogOut className="h-[18px] w-[18px]" />
        <span>Sign out</span>
      </button>
    </aside>
  );
}

import {
  ChevronDown,
  ChevronRight,
  Ellipsis,
  Folder,
  GitGraph,
  Home,
  LogOut,
  Network,
  Plus,
  Signal,
} from "lucide-react";
import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { AladinPanel, AladinToolbarField, PlaceholderPane } from "@/components/ui/aladin";
import type { BrowserTreeRow, RenameDraft } from "@/modules/workspace/domain";
import type { Artifact, VoiceCaptureDraft } from "@/shared/api/models";
import { cn } from "@/shared/lib/utils";

const navItems = [
  { key: "home", label: "Home", icon: Home, path: "/home" },
  { key: "folders", label: "Folders", icon: Folder, path: "/folders" },
  { key: "signals", label: "Signals", icon: Signal, path: "/signals" },
  { key: "sources", label: "Sources", icon: Network, path: "/sources" },
  { key: "graph", label: "Graph", icon: GitGraph, path: "/graph" },
] as const;

export function WorkspaceShellView({
  selectedDestination,
  userEmail,
  logoutPending,
  onNavigate,
  onLogout,
  onCreateFolder,
  onCreateNote,
  onCreateLink,
  onCreateVoice,
  children,
}: {
  selectedDestination: string;
  userEmail: string;
  logoutPending: boolean;
  onNavigate: (path: string) => void;
  onLogout: () => void;
  onCreateFolder: () => void;
  onCreateNote: () => void;
  onCreateLink: () => void;
  onCreateVoice: () => void;
  children: ReactNode;
}) {
  return (
    <div className="app-canvas flex h-screen text-[#0a0a0a]">
      <aside className="flex h-full w-[252px] shrink-0 border-r border-[#e7e5e4] bg-white sm:w-[272px] lg:w-sidebar">
        <div className="flex min-h-0 w-full flex-col">
          <div className="px-4 pt-5 pb-4">
            <div className="flex items-center gap-2.5">
              <div className="flex h-7 w-7 items-center justify-center rounded-md bg-[#18181b] text-[12px] font-semibold text-[#fafaf9]">
                A
              </div>
              <div className="min-w-0">
                <div className="text-[13px] font-semibold tracking-[-0.01em] text-[#0a0a0a]">Aladin</div>
                <div className="text-[11px] leading-4 text-[#78716c]">Workspace memory</div>
              </div>
            </div>
          </div>

          <div className="space-y-2 px-3">
            <AladinToolbarField text="Search…" />
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="default" size="sm" className="w-full justify-between">
                  <span>New item</span>
                  <Plus className="h-3.5 w-3.5" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="start" className="w-52">
                <DropdownMenuLabel>Create</DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={onCreateFolder}>New folder</DropdownMenuItem>
                <DropdownMenuItem onClick={onCreateNote}>New note</DropdownMenuItem>
                <DropdownMenuItem onClick={onCreateLink}>New link</DropdownMenuItem>
                <DropdownMenuItem onClick={onCreateVoice}>New voice note</DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>

          <div className="mt-5 px-3">
            <div className="eyebrow px-2 pb-2">Browse</div>
            <nav className="space-y-0.5">
              {navItems.map((item) => {
                const Icon = item.icon;
                const active = item.key === selectedDestination;
                return (
                  <button
                    key={item.key}
                    className={cn(
                      "flex h-8 w-full items-center gap-2.5 rounded-md px-2 text-[13px] transition-colors",
                      active
                        ? "bg-[#ebebe8] font-medium text-[#0a0a0a]"
                        : "text-[#57534e] hover:bg-[#f2f0ee] hover:text-[#0a0a0a]",
                    )}
                    onClick={() => onNavigate(item.path)}
                    type="button"
                  >
                    <Icon className="h-[15px] w-[15px]" strokeWidth={1.75} />
                    <span>{item.label}</span>
                  </button>
                );
              })}
            </nav>
          </div>

          <div className="mt-auto border-t border-[#e7e5e4] px-3 py-3">
            <div className="truncate px-2 text-[12px] text-[#78716c]">{userEmail}</div>
            <button
              className="mt-1 flex h-8 w-full items-center gap-2.5 rounded-md px-2 text-[13px] text-[#57534e] transition-colors hover:bg-[#f2f0ee] hover:text-[#0a0a0a] disabled:opacity-50"
              onClick={onLogout}
              type="button"
              disabled={logoutPending}
            >
              <LogOut className="h-[15px] w-[15px]" strokeWidth={1.75} />
              <span>Sign out</span>
            </button>
          </div>
        </div>
      </aside>
      <main className="flex min-w-0 flex-1 overflow-hidden">{children}</main>
    </div>
  );
}

export function BrowserPaneView({
  loading,
  errorMessage,
  breadcrumbs,
  rows,
  activeArtifactId,
  expandedFolderIds,
  canNavigateBack,
  onNavigateBack,
  onFolderPrimaryAction,
  onSelectFolder,
  onOpenArtifact,
  onStartRenameFolder,
  onStartRenameArtifact,
  onCreateFolderHere,
  onCreateNoteHere,
}: {
  loading: boolean;
  errorMessage: string | null;
  breadcrumbs: Array<{ id?: string | null; label: string }>;
  rows: BrowserTreeRow[];
  activeArtifactId: string | null;
  expandedFolderIds: string[];
  canNavigateBack: boolean;
  onNavigateBack: () => void;
  onFolderPrimaryAction: (row: BrowserTreeRow) => void;
  onSelectFolder: (folderId: string) => void;
  onOpenArtifact: (artifactId: string) => void;
  onStartRenameFolder: (folderId: string, title: string) => void;
  onStartRenameArtifact: (artifactId: string, title: string) => void;
  onCreateFolderHere: (folderId: string) => void;
  onCreateNoteHere: (folderId: string) => void;
}) {
  if (loading) {
    return (
        <section className="flex w-[336px] flex-col overflow-hidden border-r border-[#e7e5e4] bg-white sm:w-[368px]">
        <div className="p-5 text-sm text-[#44403c]">Loading browser tree…</div>
      </section>
    );
  }

  if (errorMessage) {
    return (
      <section className="flex w-[336px] flex-col overflow-hidden border-r border-[#e7e5e4] bg-white p-4 sm:w-[368px]">
        <AladinPanel className="border-[#ef4444] bg-white p-4 text-sm text-[#dc2626]">{errorMessage}</AladinPanel>
      </section>
    );
  }

  return (
    <section className="flex w-[300px] flex-col overflow-hidden border-r border-[#e7e5e4] bg-[#fafaf9] sm:w-[332px]">
      <div className="border-b border-[#e7e5e4] px-3 py-3">
        <div className="flex items-center gap-2">
          <Button variant="ghost" size="sm" disabled={!canNavigateBack} onClick={onNavigateBack} className="px-2">
            ← Back
          </Button>
          <div className="min-w-0 flex-1 truncate text-[12px] text-[#78716c]">
            {breadcrumbs.length > 0 ? breadcrumbs.map((crumb) => crumb.label).join(" / ") : "All items"}
          </div>
        </div>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto browser-pane-scroll">
        <div className="flex flex-col gap-0.5 px-2 py-2">
          {rows.map((row) => {
            const isActiveArtifact = row.artifactId === activeArtifactId;
            const isExpanded = row.folderId ? expandedFolderIds.includes(row.folderId) : false;
            return (
              <div key={row.id} className="group">
                <button
                  type="button"
                  onClick={() => {
                    if (row.kind === "artifact" && row.artifactId) {
                      onOpenArtifact(row.artifactId);
                      return;
                    }
                    if (row.kind === "folder") {
                      onFolderPrimaryAction(row);
                    }
                  }}
                  className={cn(
                    "flex w-full items-center gap-1.5 rounded-md px-1.5 py-1 text-left text-[13px]",
                    isActiveArtifact
                      ? "bg-[#ebebe8] font-medium text-[#0a0a0a]"
                      : "text-[#44403c] hover:bg-[#f2f0ee] hover:text-[#0a0a0a]",
                  )}
                  style={{ paddingLeft: `${row.depth * 14 + 6}px` }}
                >
                  {row.kind === "folder" ? (
                    <span className="flex h-5 w-5 items-center justify-center text-[#78716c]">
                      {row.scopeCandidate ? (
                        <ChevronRight className="h-3.5 w-3.5" strokeWidth={2} />
                      ) : isExpanded ? (
                        <ChevronDown className="h-3.5 w-3.5" strokeWidth={2} />
                      ) : (
                        <ChevronRight className="h-3.5 w-3.5" strokeWidth={2} />
                      )}
                    </span>
                  ) : (
                    <span className="w-5" />
                  )}
                  <span className="min-w-0 flex-1 truncate">{row.title}</span>
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <span
                        role="button"
                        onClick={(e) => e.stopPropagation()}
                        onPointerDown={(e) => e.stopPropagation()}
                        className="flex h-5 w-5 items-center justify-center rounded text-[#a8a29e] opacity-0 transition-opacity hover:bg-[#e7e5e4] hover:text-[#0a0a0a] group-hover:opacity-100"
                      >
                        <Ellipsis className="h-3.5 w-3.5" />
                      </span>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end" className="w-52">
                      {row.kind === "folder" && row.folderId ? (
                        <>
                          <DropdownMenuItem onClick={() => onStartRenameFolder(row.folderId!, row.title)}>
                            Rename folder
                          </DropdownMenuItem>
                          <DropdownMenuItem onClick={() => onCreateFolderHere(row.folderId!)}>
                            New folder here
                          </DropdownMenuItem>
                          <DropdownMenuItem onClick={() => onCreateNoteHere(row.folderId!)}>
                            New note here
                          </DropdownMenuItem>
                        </>
                      ) : null}
                      {row.kind === "artifact" && row.artifactId ? (
                        <>
                          <DropdownMenuItem onClick={() => onOpenArtifact(row.artifactId!)}>
                            Open artifact
                          </DropdownMenuItem>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem onClick={() => onStartRenameArtifact(row.artifactId!, row.title)}>
                            Rename artifact
                          </DropdownMenuItem>
                        </>
                      ) : null}
                    </DropdownMenuContent>
                  </DropdownMenu>
                </button>
              </div>
            );
          })}
          {rows.length === 0 ? (
            <div className="px-3 py-10 text-center text-[12px] text-[#a8a29e]">Nothing here yet.</div>
          ) : null}
        </div>
      </div>
    </section>
  );
}

export function WorkPaneView({
  openArtifacts,
  activeArtifact,
  onActivateArtifact,
  onCloseArtifact,
  children,
}: {
  openArtifacts: Artifact[];
  activeArtifact: Artifact | null;
  onActivateArtifact: (artifactId: string) => void;
  onCloseArtifact: (artifactId: string) => void;
  children: ReactNode;
}) {
  return (
    <section className="flex min-w-0 flex-1 flex-col overflow-hidden bg-white">
      <div className="border-b border-[#e7e5e4] bg-[#fafaf9] px-2 pt-1.5">
        <div className="scrollbar-hidden flex min-w-0 items-end gap-0.5 overflow-x-auto overflow-y-hidden">
          {openArtifacts.map((artifact) => {
            const active = artifact.id === activeArtifact?.id;
            return (
              <button
                key={artifact.id}
                className={cn(
                  "group relative flex h-9 items-center gap-2 rounded-t-md border border-b-0 px-3 text-[12.5px] transition-colors",
                  active
                    ? "border-[#e7e5e4] bg-white font-medium text-[#0a0a0a] -mb-px"
                    : "border-transparent bg-transparent text-[#78716c] hover:text-[#0a0a0a]",
                )}
                onClick={() => onActivateArtifact(artifact.id)}
                type="button"
              >
                <span className="max-w-[200px] truncate">{artifact.title}</span>
                <span
                  className="ml-1 flex h-4 w-4 items-center justify-center rounded text-[#a8a29e] transition-colors hover:bg-[#ececea] hover:text-[#0a0a0a]"
                  onClick={(event) => {
                    event.stopPropagation();
                    onCloseArtifact(artifact.id);
                  }}
                >
                  ×
                </span>
              </button>
            );
          })}
        </div>
      </div>
      <div className="min-h-0 flex-1 bg-white">{children}</div>
    </section>
  );
}

export function RenameDialogView({
  rename,
  pending,
  onDraftTitleChange,
  onCancel,
  onSave,
}: {
  rename: RenameDraft | null;
  pending: boolean;
  onDraftTitleChange: (title: string) => void;
  onCancel: () => void;
  onSave: () => void;
}) {
  return (
    <Dialog open={Boolean(rename)} onOpenChange={(open) => !open && onCancel()}>
      <DialogContent className="w-[min(92vw,520px)]">
        <DialogHeader>
          <DialogTitle>Rename {rename?.kind === "folder" ? "folder" : "artifact"}</DialogTitle>
          <DialogDescription>Update the title in place without changing its location.</DialogDescription>
        </DialogHeader>
        <DialogBody className="pb-2">
          <Input
            autoFocus
            value={rename?.draftTitle ?? ""}
            onChange={(event) => onDraftTitleChange(event.target.value)}
          />
        </DialogBody>
        <DialogFooter>
          <Button variant="secondary" onClick={onCancel}>
            Cancel
          </Button>
          <Button onClick={onSave} disabled={pending || !rename?.draftTitle.trim()}>
            {pending ? "Saving…" : "Save"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export function VoiceCaptureDialogView({
  draft,
  permissionError,
  pending,
  onClose,
  onPatchDraft,
  onStartRecording,
  onStopRecording,
  onSave,
}: {
  draft: VoiceCaptureDraft | null;
  permissionError: string | null;
  pending: boolean;
  onClose: () => void;
  onPatchDraft: (patch: Partial<VoiceCaptureDraft>) => void;
  onStartRecording: () => void;
  onStopRecording: () => void;
  onSave: () => void;
}) {
  return (
    <Dialog open={Boolean(draft)} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="w-[min(92vw,640px)]">
        <DialogHeader>
          <DialogTitle>New voice note</DialogTitle>
          <DialogDescription>Record audio, review it, then save it to the workspace.</DialogDescription>
        </DialogHeader>
        {draft ? (
          <DialogBody className="space-y-5 pb-4">
            <div className="space-y-2">
              <label className="eyebrow">Title</label>
              <Input
                value={draft.title}
                onChange={(event) => onPatchDraft({ title: event.target.value })}
              />
            </div>
            <div className="space-y-2">
              <label className="eyebrow">Notes</label>
              <Textarea
                value={draft.description}
                onChange={(event) => onPatchDraft({ description: event.target.value })}
              />
            </div>

            <div className="panel-white bg-white p-4">
              {draft.phase === "recording" ? (
                <div className="flex items-center gap-3 text-sm text-[#44403c]">
                  <span className="h-2.5 w-2.5 animate-pulse rounded-full bg-red-500" />
                  Recording in progress…
                </div>
              ) : draft.audioUrl ? (
                <audio className="w-full" controls src={draft.audioUrl} />
              ) : (
                <p className="text-sm leading-7 text-[#78716c]">
                  No audio captured yet. Start recording to create a preview.
                </p>
              )}
            </div>

            {permissionError || draft.errorMessage ? (
            <div className="rounded-[12px] border border-[#0a0a0a] bg-white px-4 py-3 text-sm leading-6 text-[#0a0a0a]">
              {permissionError ?? draft.errorMessage}
            </div>
            ) : null}
          </DialogBody>
        ) : null}
        <DialogFooter>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          {draft?.phase === "recording" ? (
            <Button variant="destructive" onClick={onStopRecording}>
              Stop recording
            </Button>
          ) : (
            <Button variant="secondary" onClick={onStartRecording}>
              {draft?.audioUrl ? "Record again" : "Start recording"}
            </Button>
          )}
          <Button onClick={onSave} disabled={pending || !draft?.audioBlob}>
            {pending ? "Saving…" : "Save voice note"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export function PlaceholderDestinationView({
  paneTitle,
  paneBody,
  workTitle,
  workBody,
}: {
  paneTitle: string;
  paneBody: string;
  workTitle: string;
  workBody: string;
}) {
  return (
    <>
      <section className="flex w-[336px] flex-col overflow-hidden border-r border-[#e7e5e4] bg-white sm:w-[368px]">
        <PlaceholderPane title={paneTitle} body={paneBody} className="h-full" />
      </section>
      <section className="min-w-0 flex-1 overflow-hidden bg-white">
        <PlaceholderPane title={workTitle} body={workBody} className="h-full" />
      </section>
    </>
  );
}

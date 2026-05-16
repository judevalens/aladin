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
import { ScrollArea } from "@/components/ui/scroll-area";
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
    <div className="app-canvas flex h-screen text-[#111111]">
      <aside className="flex h-full w-[280px] shrink-0 border-r border-[#e4e4e4] bg-white sm:w-[304px] lg:w-sidebar">
        <div className="flex min-h-0 w-full flex-col">
          <div className="border-b border-[#e4e4e4] px-4 py-4">
            <div className="flex items-start gap-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-[12px] border border-[#dedbd4] bg-white text-sm font-semibold text-[#111111]">
                A
              </div>
              <div className="min-w-0 space-y-1">
                <div className="text-sm font-semibold tracking-[-0.02em] text-[#111111]">Aladin</div>
                <div className="text-sm leading-6 text-[#52525b]">Local workspace memory for notes, sources, and signals.</div>
              </div>
            </div>
          </div>

          <div className="space-y-3 px-4 py-4">
            <AladinToolbarField text="Search, jump, or reopen..." />
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="default" className="w-full justify-between">
                  <span>New item</span>
                  <Plus className="h-4 w-4" />
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

          <div className="hairline-divider h-px" />

          <div className="space-y-3 px-4 py-4">
            <div className="eyebrow">Browse</div>
            <nav className="space-y-1">
              {navItems.map((item) => {
                const Icon = item.icon;
                const active = item.key === selectedDestination;
                return (
                  <button
                    key={item.key}
                    className={cn(
                      "flex h-[42px] w-full items-center gap-3 rounded-[12px] px-3 text-sm transition-colors",
                      active
                        ? "bg-[#ededed] text-[#111111]"
                        : "text-[#52525b] hover:bg-[#f3f3f3] hover:text-[#111111]",
                    )}
                    onClick={() => onNavigate(item.path)}
                    type="button"
                  >
                    <Icon className="h-[18px] w-[18px]" />
                    <span className="font-medium">{item.label}</span>
                  </button>
                );
              })}
            </nav>
          </div>

          <div className="mt-auto border-t border-[#e4e4e4] px-4 py-4">
            <div className="eyebrow">Signed in</div>
            <div className="mt-3 truncate text-sm text-[#3f3f46]">{userEmail}</div>
            <button
              className="mt-4 flex h-[40px] w-full items-center gap-3 rounded-[10px] px-3 text-sm text-[#52525b] transition-colors hover:bg-[#f3f3f3] hover:text-[#111111] disabled:opacity-50"
              onClick={onLogout}
              type="button"
              disabled={logoutPending}
            >
              <LogOut className="h-[18px] w-[18px]" />
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
        <section className="flex w-[336px] flex-col overflow-hidden border-r border-[#e4e4e4] bg-white sm:w-[368px]">
        <div className="p-5 text-sm text-[#3f3f46]">Loading browser tree…</div>
      </section>
    );
  }

  if (errorMessage) {
    return (
      <section className="flex w-[336px] flex-col overflow-hidden border-r border-[#e4e4e4] bg-white p-4 sm:w-[368px]">
        <AladinPanel className="border-[#ef4444] bg-white p-4 text-sm text-[#b42318]">{errorMessage}</AladinPanel>
      </section>
    );
  }

  return (
    <section className="flex w-[336px] flex-col overflow-hidden border-r border-[#e4e4e4] bg-white sm:w-[368px]">
      <div className="border-b border-[#e4e4e4] bg-white px-4 py-4">
        <div className="flex items-start gap-3">
          <Button variant="secondary" size="sm" disabled={!canNavigateBack} onClick={onNavigateBack}>
            Back
          </Button>
          <div className="min-w-0 flex-1">
            <div className="eyebrow">Browser</div>
            <div className="mt-2 truncate text-sm leading-6 text-[#52525b]">
              {breadcrumbs.length > 0 ? breadcrumbs.map((crumb) => crumb.label).join(" / ") : "All items"}
            </div>
          </div>
        </div>
      </div>
      <ScrollArea className="min-h-0 flex-1">
        <div className="flex flex-col px-3 py-3">
          {rows.map((row) => {
            const isActiveArtifact = row.artifactId === activeArtifactId;
            const isExpanded = row.folderId ? expandedFolderIds.includes(row.folderId) : false;
            return (
              <div key={row.id} className="group">
                <div
                  className={cn(
                    "flex items-center gap-2 rounded-[10px] px-2 py-2 text-sm",
                    isActiveArtifact
                      ? "bg-[#ededed] text-[#111111]"
                      : "text-[#52525b] hover:bg-[#f3f3f3] hover:text-[#111111]",
                  )}
                  style={{ paddingLeft: `${row.depth * 16 + 8}px` }}
                >
                  {row.kind === "folder" ? (
                    <button
                      className="flex h-6 w-6 items-center justify-center rounded-[8px] hover:bg-[#f3f3f3]"
                      onClick={() => onFolderPrimaryAction(row)}
                      type="button"
                    >
                      {row.scopeCandidate ? (
                        <ChevronRight className="h-4 w-4" />
                      ) : isExpanded ? (
                        <ChevronDown className="h-4 w-4" />
                      ) : (
                        <ChevronRight className="h-4 w-4" />
                      )}
                    </button>
                  ) : (
                    <span className="w-6" />
                  )}
                  <button
                    className="min-w-0 flex-1 truncate text-left"
                    onClick={() => {
                      if (row.kind === "artifact" && row.artifactId) {
                        onOpenArtifact(row.artifactId);
                        return;
                      }
                      if (row.folderId) {
                        onSelectFolder(row.folderId);
                      }
                    }}
                    type="button"
                  >
                    {row.title}
                  </button>
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <button
                        className="flex h-7 w-7 items-center justify-center rounded-[8px] text-[#6b7280] opacity-0 transition-opacity hover:bg-[#f3f3f3] hover:text-[#111111] group-hover:opacity-100"
                        type="button"
                      >
                        <Ellipsis className="h-4 w-4" />
                      </button>
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
                </div>
              </div>
            );
          })}
          {rows.length === 0 ? (
            <div className="px-3 py-8 text-sm leading-7 text-[#6b7280]">Nothing in this section yet.</div>
          ) : null}
        </div>
      </ScrollArea>
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
      <div className="border-b border-[#e4e4e4] bg-white px-4 py-3">
        <div className="scrollbar-hidden flex min-w-0 gap-2 overflow-x-auto overflow-y-hidden">
          {openArtifacts.map((artifact) => {
            const active = artifact.id === activeArtifact?.id;
            return (
              <button
                key={artifact.id}
                className={cn(
                  "group flex h-[42px] items-center gap-2 rounded-[11px] border px-3 text-sm transition-colors",
                  active
                    ? "border-[#ededed] bg-[#ededed] text-[#111111]"
                    : "border-transparent bg-transparent text-[#52525b] hover:bg-[#f3f3f3] hover:text-[#111111]",
                )}
                onClick={() => onActivateArtifact(artifact.id)}
                type="button"
              >
                <span className="max-w-[220px] truncate">{artifact.title}</span>
                <span
                  className="rounded-[6px] px-1 text-xs text-current/70 transition-colors hover:bg-white hover:text-[#111111]"
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
                <div className="flex items-center gap-3 text-sm text-[#3f3f46]">
                  <span className="h-2.5 w-2.5 animate-pulse rounded-full bg-red-500" />
                  Recording in progress…
                </div>
              ) : draft.audioUrl ? (
                <audio className="w-full" controls src={draft.audioUrl} />
              ) : (
                <p className="text-sm leading-7 text-[#6b7280]">
                  No audio captured yet. Start recording to create a preview.
                </p>
              )}
            </div>

            {permissionError || draft.errorMessage ? (
            <div className="rounded-[12px] border border-[#111111] bg-white px-4 py-3 text-sm leading-6 text-[#111111]">
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
      <section className="flex w-[336px] flex-col overflow-hidden border-r border-[#e4e4e4] bg-white sm:w-[368px]">
        <PlaceholderPane title={paneTitle} body={paneBody} className="h-full" />
      </section>
      <section className="min-w-0 flex-1 overflow-hidden bg-white">
        <PlaceholderPane title={workTitle} body={workBody} className="h-full" />
      </section>
    </>
  );
}

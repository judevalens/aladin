import { ChevronDown, ChevronLeft, ChevronRight, Ellipsis } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { AladinPanel } from "@/components/ui/aladin";
import type { BreadcrumbCrumb, BreadcrumbView, BrowserTreeRow } from "@/modules/workspace/domain";
import { cn } from "@/shared/lib/utils";

export function BrowserPaneView({
  loading,
  errorMessage,
  breadcrumb,
  rows,
  activeArtifactId,
  expandedFolderIds,
  onNavigateToScope,
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
  breadcrumb: BreadcrumbView;
  rows: BrowserTreeRow[];
  activeArtifactId: string | null;
  expandedFolderIds: string[];
  onNavigateToScope: (folderId: string | null) => void;
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
      <div className="border-b border-[#e7e5e4] px-3 py-2.5">
        <BrowserBreadcrumb breadcrumb={breadcrumb} onNavigateToScope={onNavigateToScope} />
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

function BrowserBreadcrumb({
  breadcrumb,
  onNavigateToScope,
}: {
  breadcrumb: BreadcrumbView;
  onNavigateToScope: (folderId: string | null) => void;
}) {
  const parentCrumb =
    breadcrumb.crumbs.length >= 2 ? breadcrumb.crumbs[breadcrumb.crumbs.length - 2] : null;
  return (
    <nav className="flex min-w-0 items-center text-[12px] text-[#78716c]">
      {parentCrumb ? (
        <button
          type="button"
          aria-label="Up one level"
          title={`Up to ${parentCrumb.label}`}
          onClick={() => onNavigateToScope(parentCrumb.id)}
          className="mr-1 flex h-5 w-5 items-center justify-center rounded text-[#78716c] hover:bg-[#ebebe8] hover:text-[#0a0a0a]"
        >
          <ChevronLeft className="h-3.5 w-3.5" strokeWidth={2} />
        </button>
      ) : null}
      {breadcrumb.visible.map((crumb, index) => {
        const isLast = index === breadcrumb.visible.length - 1;
        return (
          <div key={`${crumb.kind}-${crumb.id ?? index}`} className="flex min-w-0 items-center">
            <BreadcrumbSegment crumb={crumb} onNavigateToScope={onNavigateToScope} isCurrent={isLast} />
            {!isLast ? <span className="mx-1 text-[#d6d3d1]">/</span> : null}
          </div>
        );
      })}
    </nav>
  );
}

function BreadcrumbSegment({
  crumb,
  onNavigateToScope,
  isCurrent,
}: {
  crumb: BreadcrumbCrumb;
  onNavigateToScope: (folderId: string | null) => void;
  isCurrent: boolean;
}) {
  const hasDestinations = crumb.kind === "ellipsis" ? crumb.siblings.length > 0 : true;
  if (!hasDestinations) {
    return (
      <span
        className={cn(
          "max-w-[14ch] truncate px-1",
          isCurrent ? "font-medium text-[#0a0a0a]" : "text-[#78716c]",
        )}
      >
        {crumb.label}
      </span>
    );
  }
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          className={cn(
            "max-w-[14ch] truncate rounded px-1 py-0.5 hover:bg-[#ebebe8] hover:text-[#0a0a0a]",
            isCurrent ? "font-medium text-[#0a0a0a]" : "text-[#78716c]",
          )}
        >
          {crumb.label}
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-52">
        {crumb.kind !== "ellipsis" && !isCurrent ? (
          <>
            <DropdownMenuItem onClick={() => onNavigateToScope(crumb.id)}>
              Go to {crumb.kind === "root" ? "root" : crumb.label}
            </DropdownMenuItem>
            {crumb.siblings.length > 0 ? <DropdownMenuSeparator /> : null}
          </>
        ) : null}
        {crumb.siblings.map((sibling) => (
          <DropdownMenuItem key={sibling.id} onClick={() => onNavigateToScope(sibling.id)}>
            {sibling.label}
          </DropdownMenuItem>
        ))}
        {crumb.kind !== "ellipsis" && crumb.siblings.length === 0 && isCurrent ? (
          <DropdownMenuItem disabled>No sibling folders</DropdownMenuItem>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

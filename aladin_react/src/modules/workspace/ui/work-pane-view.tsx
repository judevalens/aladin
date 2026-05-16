import { LineChart, Search, SlidersHorizontal, Star } from "lucide-react";
import type { ReactNode } from "react";
import type { Artifact } from "@/shared/api/models";
import { cn } from "@/shared/lib/utils";

export function WorkPaneView({
  openArtifacts,
  activeArtifact,
  statusPath,
  inspectorOpen,
  onActivateArtifact,
  onCloseArtifact,
  onToggleInspector,
  children,
}: {
  openArtifacts: Artifact[];
  activeArtifact: Artifact | null;
  statusPath: string[];
  inspectorOpen: boolean;
  onActivateArtifact: (artifactId: string) => void;
  onCloseArtifact: (artifactId: string) => void;
  onToggleInspector: () => void;
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
      {activeArtifact ? (
        <WorkPaneStatusBar
          path={statusPath}
          inspectorOpen={inspectorOpen}
          onToggleInspector={onToggleInspector}
        />
      ) : null}
      <div className="min-h-0 flex-1 bg-white">{children}</div>
    </section>
  );
}

function WorkPaneStatusBar({
  path,
  inspectorOpen,
  onToggleInspector,
}: {
  path: string[];
  inspectorOpen: boolean;
  onToggleInspector: () => void;
}) {
  return (
    <div className="flex items-center gap-3 border-b border-[#e7e5e4] bg-[#fafaf9] px-3.5 py-1.5">
      <div className="min-w-0 flex-1 truncate text-[12px] text-[#78716c]">
        {path.length > 0 ? path.join(" / ") : ""}
      </div>
      <div className="flex items-center gap-0.5">
        <StatusUtilityIcon ariaLabel="Search document" onClick={() => undefined}>
          <Search className="h-[15px] w-[15px]" strokeWidth={1.75} />
        </StatusUtilityIcon>
        <StatusUtilityIcon ariaLabel="Favorite document" onClick={() => undefined}>
          <Star className="h-[15px] w-[15px]" strokeWidth={1.75} />
        </StatusUtilityIcon>
        <StatusUtilityIcon ariaLabel="Open graph context" onClick={() => undefined}>
          <LineChart className="h-[15px] w-[15px]" strokeWidth={1.75} />
        </StatusUtilityIcon>
        <StatusUtilityIcon
          ariaLabel="Toggle inspector"
          isActive={inspectorOpen}
          onClick={onToggleInspector}
        >
          <SlidersHorizontal className="h-[15px] w-[15px]" strokeWidth={1.75} />
        </StatusUtilityIcon>
      </div>
    </div>
  );
}

function StatusUtilityIcon({
  ariaLabel,
  isActive = false,
  onClick,
  children,
}: {
  ariaLabel: string;
  isActive?: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      aria-label={ariaLabel}
      title={ariaLabel}
      onClick={onClick}
      className={cn(
        "flex h-6 w-6 items-center justify-center rounded transition-colors",
        isActive
          ? "bg-[#0a0a0a] text-white hover:bg-[#1f1f1f]"
          : "text-[#78716c] hover:bg-[#ebebe8] hover:text-[#0a0a0a]",
      )}
    >
      {children}
    </button>
  );
}

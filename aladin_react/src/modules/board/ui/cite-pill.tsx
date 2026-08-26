import { useBoardHost } from "../domain/board-host";
import { useBoardPaper } from "../domain/board-paper";
import { DOCK_PATHS, DockIcon } from "./dock-icons";

/**
 * Paper's provenance, top-left: the exercise this worksheet was spawned from. Tapping it
 * is the wormhole home — the source opens at the cited page.
 */
export function CitePill() {
  const paper = useBoardPaper();
  const host = useBoardHost();
  const cite = paper.cite;
  if (!cite) return null;
  const label = cite.title ? `${cite.title} · p. ${cite.page}` : `p. ${cite.page}`;
  const canOpen = Boolean(host.onOpenArtifact);
  return (
    <button
      type="button"
      disabled={!canOpen}
      onClick={() => host.onOpenArtifact?.(cite.artifactId, { page: cite.page })}
      className="board-island board-island--pill board-edge-top pointer-events-auto absolute left-5.5 flex h-11 max-w-[46vw] items-center gap-2 pl-3 pr-4 text-board-label text-ink-3 hover:text-ink disabled:hover:text-ink-3"
      title={canOpen ? "Open the source at this page" : undefined}
    >
      <DockIcon d={DOCK_PATHS.file} size={15} strokeWidth={1.9} />
      <span className="truncate">from {label}</span>
    </button>
  );
}

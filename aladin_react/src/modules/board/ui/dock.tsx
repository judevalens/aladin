import {
  BOARD_INK_SWATCHES,
  BOARD_WEIGHTS,
  type BoardTool,
  type BoardWeightIndex,
  type PencilSubTool,
} from "../domain/board-tools";
import type { BoardInkColor } from "../domain/board-theme";
import { DOCK_PATHS, DockIcon } from "./dock-icons";

/**
 * The bottom-center tool dock — pure render, all state and effects live in BoardChrome.
 * Geometry per the handoff: 6px padding, 52px buttons at radius 15, 1×28px dividers,
 * active = amber-soft bg + amber icon.
 */
export function Dock({
  tool,
  subTool,
  inkColor,
  weight,
  insertOpen,
  onPickTool,
  onPickSubTool,
  onPickColor,
  onPickWeight,
  onToggleInsert,
  onAddInk,
  onAddTask,
  onAddCard,
}: {
  tool: BoardTool;
  subTool: PencilSubTool;
  inkColor: BoardInkColor;
  weight: BoardWeightIndex;
  insertOpen: boolean;
  onPickTool: (tool: BoardTool) => void;
  onPickSubTool: (subTool: PencilSubTool) => void;
  onPickColor: (color: BoardInkColor) => void;
  onPickWeight: (weight: BoardWeightIndex) => void;
  onToggleInsert: () => void;
  onAddInk: () => void;
  onAddTask: () => void;
  onAddCard: () => void;
}) {
  const mainTools: { id: BoardTool; label: string; d: string }[] = [
    { id: "select", label: "Select", d: DOCK_PATHS.select },
    { id: "pencil", label: "Pencil", d: DOCK_PATHS.pencil },
    { id: "arrow", label: "Arrow", d: DOCK_PATHS.arrow },
  ];
  const subTools: { id: PencilSubTool; label: string; d: string }[] = [
    { id: "pen", label: "Pen", d: DOCK_PATHS.pen },
    { id: "highlighter", label: "Highlighter", d: DOCK_PATHS.highlighter },
    { id: "eraser", label: "Eraser", d: DOCK_PATHS.eraser },
    { id: "lasso", label: "Lasso", d: DOCK_PATHS.lasso },
  ];

  return (
    <div className="board-glass-dock pointer-events-auto absolute bottom-[calc(22px+var(--host-bottom-inset,0px))] left-1/2 flex -translate-x-1/2 items-center gap-1 p-1.5">
      <DockButton
        label="Insert from workspace"
        active={insertOpen}
        onClick={onToggleInsert}
      >
        <DockIcon d={DOCK_PATHS.insert} size={22} strokeWidth={1.9} />
      </DockButton>
      <Divider />
      {mainTools.map((t) => (
        <DockButton key={t.id} label={t.label} active={tool === t.id} onClick={() => onPickTool(t.id)}>
          <DockIcon d={t.d} />
        </DockButton>
      ))}
      {tool === "select" ? (
        <>
          <Divider />
          <TextButton label="Ink" onClick={onAddInk} />
          <TextButton label="Task" onClick={onAddTask} />
          <TextButton label="Card" onClick={onAddCard} />
        </>
      ) : null}
      {tool === "pencil" ? (
        <>
          <Divider />
          {subTools.map((t) => (
            <DockButton
              key={t.id}
              label={t.label}
              active={subTool === t.id}
              narrow
              onClick={() => onPickSubTool(t.id)}
            >
              <DockIcon d={t.d} size={20} />
            </DockButton>
          ))}
          <Divider />
          {BOARD_INK_SWATCHES.map((swatch) => (
            <button
              key={swatch.id}
              type="button"
              aria-label={swatch.label}
              title={swatch.label}
              onClick={() => onPickColor(swatch.id)}
              className="grid h-[52px] w-[34px] place-items-center"
            >
              <span
                className="h-[19px] w-[19px] rounded-full transition-shadow"
                style={{
                  background: swatch.cssVar,
                  boxShadow:
                    inkColor === swatch.id
                      ? "0 0 0 2.5px var(--bg), 0 0 0 4.5px var(--ink-2)"
                      : "0 0 0 1px rgb(var(--line))",
                }}
              />
            </button>
          ))}
          <Divider />
          {BOARD_WEIGHTS.map((w, index) => (
            <button
              key={w.size}
              type="button"
              aria-label="Stroke width"
              onClick={() => onPickWeight(index as BoardWeightIndex)}
              className={`grid h-[52px] w-9 place-items-center rounded-[13px] hover:bg-hover ${
                weight === index ? "bg-sel" : ""
              }`}
            >
              <span
                className={`rounded-full ${weight === index ? "bg-ink" : "bg-ink-3"}`}
                style={{ width: w.dotPx, height: w.dotPx }}
              />
            </button>
          ))}
        </>
      ) : null}
    </div>
  );
}

function DockButton({
  label,
  active,
  narrow,
  onClick,
  children,
}: {
  label: string;
  active: boolean;
  narrow?: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      onClick={onClick}
      className={`grid h-[52px] place-items-center rounded-[15px] transition-colors ${
        narrow ? "w-12" : "w-[52px]"
      } ${active ? "bg-amber-soft text-amber" : "text-ink-3 hover:bg-hover"}`}
    >
      {children}
    </button>
  );
}

/** Task/Card arrive with their shapes (P2); until then the buttons render disabled. */
function TextButton({ label, onClick }: { label: string; onClick: (() => void) | undefined }) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={!onClick}
      className="h-[52px] rounded-[15px] px-[15px] text-[15px] text-ink-2 hover:bg-hover hover:text-ink disabled:cursor-default disabled:text-ink-4 disabled:hover:bg-transparent"
    >
      {label}
    </button>
  );
}

function Divider() {
  return <span className="mx-1.5 h-7 w-px bg-line" />;
}

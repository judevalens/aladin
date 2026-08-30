import { Check, Eraser, Frame, Hand, Highlighter, Lasso, LayoutGrid, Link2, Maximize, Minus, Moon, MousePointer2, Pencil, Plus, Redo2, SlidersHorizontal, StickyNote, Sun, Type, Undo2 } from "lucide-react";
import { BOARD_INK_SWATCHES, BOARD_WEIGHTS, type BoardTool, type BoardWeightIndex, type PencilSubTool } from "../domain/board-tools";
import type { BoardInkColor } from "../domain/board-theme";
import type { BoardAppearance } from "../domain/board-appearance";
import { BoardButton } from "./board-button";

export interface DockProps {
  tool: BoardTool;
  subTool: PencilSubTool;
  inkColor: BoardInkColor;
  weight: BoardWeightIndex;
  drawWithFinger: boolean;
  insertOpen: boolean;
  pencilMenuOpen: boolean;
  styleOpen: boolean;
  appearance: BoardAppearance;
  zoomPct: number;
  zoomLocked: boolean;
  canUndo: boolean;
  canRedo: boolean;
  onUndo: () => void;
  onRedo: () => void;
  onPickTool: (tool: BoardTool) => void;
  onPickSubTool: (tool: PencilSubTool) => void;
  onPickColor: (color: BoardInkColor) => void;
  onPickWeight: (weight: BoardWeightIndex) => void;
  onToggleDrawWithFinger: () => void;
  onToggleInsert: () => void;
  onToggleStyle: () => void;
  onToggleAppearance: () => void;
  onZoomIn: () => void;
  onZoomOut: () => void;
  onResetZoom: () => void;
  onFit: () => void;
  onAddNote: () => void;
}

/** Stationary creation rail; contextual controls are summoned and dismissed. */
export function Dock(props: DockProps) {
  const { tool, subTool, inkColor, weight, appearance } = props;
  return <>
    <div className="board-top-actions">
      <BoardButton label={appearance === "light" ? "Use dark board" : "Use light board"} icon={appearance === "light" ? Moon : Sun} onClick={props.onToggleAppearance} />
      <button type="button" className="rs-library-button" aria-expanded={props.insertOpen} onClick={props.onToggleInsert}><LayoutGrid size={16} /> Library</button>
    </div>
    <div className="rs-tool-rail rs-surface" role="toolbar" aria-label="Board tools">
      <BoardButton label="Select" icon={MousePointer2} active={tool === "select"} onClick={() => props.onPickTool("select")} />
      <BoardButton label="Pan" icon={Hand} active={tool === "hand"} onClick={() => props.onPickTool("hand")} />
      <span className="rs-separator" />
      <BoardButton label="Sticky note" icon={StickyNote} active={tool === "note"} onClick={props.onAddNote} />
      <BoardButton label="Text" icon={Type} active={tool === "text"} onClick={() => props.onPickTool("text")} />
      <div className="rs-tool-anchor"><BoardButton label="Pencil" icon={Pencil} active={tool === "pencil"} aria-expanded={props.pencilMenuOpen} onClick={() => props.onPickTool("pencil")} /><span className="rs-tool-dot" /></div>
      <BoardButton label="Connect" icon={Link2} active={tool === "arrow"} onClick={() => props.onPickTool("arrow")} />
      <BoardButton label="Frame" icon={Frame} active={tool === "frame"} onClick={() => props.onPickTool("frame")} />
      <span className="rs-separator" />
      <BoardButton label="Add to board" icon={Plus} active={props.insertOpen} onClick={props.onToggleInsert} />
    </div>
    {tool === "pencil" && props.pencilMenuOpen && !props.insertOpen && <div className="rs-pencil-popover rs-surface board-pencil-palette" role="toolbar" aria-label="Pencil tools">
      <div className="rs-inline-tools">
        {([{ id: "pen", label: "Pen", icon: Pencil }, { id: "highlighter", label: "Highlighter", icon: Highlighter }, { id: "eraser", label: "Eraser", icon: Eraser }, { id: "lasso", label: "Lasso", icon: Lasso }] as const).map(({ id, label, icon }) => <BoardButton key={id} label={label} icon={icon} active={subTool === id} onClick={() => props.onPickSubTool(id)} />)}
      </div>
      <div className="rs-swatches">
        {BOARD_INK_SWATCHES.map((swatch) => <button key={swatch.id} type="button" className="rs-swatch" aria-label={swatch.label} aria-pressed={inkColor === swatch.id} style={{ background: swatch.cssVar }} onClick={() => props.onPickColor(swatch.id)}>{inkColor === swatch.id && <Check size={12} />}</button>)}
        <BoardButton label="Stroke settings" icon={SlidersHorizontal} active={props.styleOpen} aria-expanded={props.styleOpen} onClick={props.onToggleStyle} />
      </div>
      {props.styleOpen && <div className="board-stroke-settings">
        <div className="rs-inline-tools">{BOARD_WEIGHTS.map((item, index) => <button key={item.size} type="button" className="rs-icon-button" aria-label={"Stroke weight " + (index + 1)} aria-pressed={weight === index} onClick={() => props.onPickWeight(index as BoardWeightIndex)}><span style={{ width: item.dotPx, height: item.dotPx, borderRadius: "50%", background: "currentColor" }} /></button>)}</div>
        <button type="button" className="board-finger-toggle" aria-pressed={props.drawWithFinger} onClick={props.onToggleDrawWithFinger}>Finger draws {props.drawWithFinger && <Check size={13} />}</button>
      </div>}
    </div>}
    <div className="rs-history rs-surface" role="toolbar" aria-label="History"><BoardButton label="Undo" icon={Undo2} disabled={!props.canUndo} onClick={props.onUndo} /><BoardButton label="Redo" icon={Redo2} disabled={!props.canRedo} onClick={props.onRedo} /></div>
    <div className="rs-zoom rs-surface" role="toolbar" aria-label="Canvas navigation">
      <BoardButton label="Zoom out" icon={Minus} disabled={props.zoomLocked} onClick={props.onZoomOut} />
      <button type="button" className="rs-zoom-level" aria-label="Reset zoom" disabled={props.zoomLocked} onClick={props.onResetZoom}>{props.zoomPct}%</button>
      <BoardButton label="Zoom in" icon={Plus} disabled={props.zoomLocked} onClick={props.onZoomIn} />
      <span className="rs-divider" /><BoardButton label="Fit board" icon={Maximize} disabled={props.zoomLocked} onClick={props.onFit} />
    </div>
  </>;
}

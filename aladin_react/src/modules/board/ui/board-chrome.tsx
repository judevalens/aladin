import { useCallback, useEffect, useRef, useState } from "react";
import {
  DefaultColorStyle,
  DefaultSizeStyle,
  createShapeId,
  toRichText,
  useEditor,
  useValue,
  type TLEventInfo,
  type VecLike,
} from "tldraw";

import { boardCameraOptions, pencilCameraOptions } from "../domain/board-camera";
import { useBoardContent, useBoardFolder, type PickerArtifact } from "../domain/board-content";
import { addCard, addDocWindow, addExcerpt, addTask, boardArtifactIds } from "../domain/board-objects";
import type { BoardInkColor } from "../domain/board-theme";
import {
  BOARD_WEIGHTS,
  PENCIL_HINTS,
  boardToolFromTldraw,
  tldrawToolId,
  type BoardTool,
  type BoardWeightIndex,
  type PencilSubTool,
} from "../domain/board-tools";
import { Dock } from "./dock";
import { DOCK_PATHS, DockIcon } from "./dock-icons";
import { HintPill } from "./hint-pill";
import { InsertPopover, type InsertRow } from "./insert-popover";
import { PickerPanel } from "./picker-panel";
import { SelectionBar } from "./selection-bar";
import { ZoomPill } from "./zoom-pill";

/**
 * The board's floating chrome, mounted via `components.InFrontOfTheCanvas` so it lives
 * inside the editor context. tldraw's `getCurrentToolId()` is the single source of truth
 * for which tool is active — the dock derives its lit state from it and only remembers
 * the last pencil sub-tool, ink color and weight.
 */
export function BoardChrome() {
  const editor = useEditor();
  const toolId = useValue("toolId", () => editor.getCurrentToolId(), [editor]);
  const { tool, subTool: activeSubTool } = boardToolFromTldraw(toolId);

  const [lastSubTool, setLastSubTool] = useState<PencilSubTool>("pen");
  const [inkColor, setInkColor] = useState<BoardInkColor>("learn");
  const [weight, setWeight] = useState<BoardWeightIndex>(1);
  const [pickerOpen, setPickerOpen] = useState(false);
  const [hold, setHold] = useState<{ page: VecLike; viewport: VecLike } | null>(null);
  const subTool = activeSubTool ?? lastSubTool;

  // Picker data: the board's folder siblings, refreshed each time the panel opens.
  const contentSource = useBoardContent();
  const folderId = useBoardFolder();
  const [pickerArtifacts, setPickerArtifacts] = useState<PickerArtifact[] | null>(null);
  useEffect(() => {
    if (!pickerOpen || !contentSource) return;
    let alive = true;
    contentSource
      .listFolderArtifacts(folderId)
      .then((rows) => {
        if (alive) setPickerArtifacts(rows);
      })
      .catch(() => {
        if (alive) setPickerArtifacts([]);
      });
    return () => {
      alive = false;
    };
  }, [pickerOpen, contentSource, folderId]);

  const applyInkStyles = useCallback(
    (color: BoardInkColor, weightIndex: BoardWeightIndex) => {
      editor.setStyleForNextShapes(DefaultColorStyle, color);
      editor.setStyleForNextShapes(DefaultSizeStyle, BOARD_WEIGHTS[weightIndex].size);
    },
    [editor],
  );

  // Rule 3 — the pencil owns the camera. Any entry into a pencil sub-tool (dock tap,
  // shortcut, whatever) snaps to 1:1 about the viewport center and clamps the zoom steps,
  // so pinch is a no-op while two-finger pan keeps working. Leaving pencil restores the
  // stepped camera. Styles ride the same seam: pencil asserts the ink color/weight, arrow
  // asserts the link style, so the two regimes cannot leak strokes into each other.
  const inkRef = useRef({ inkColor, weight });
  inkRef.current = { inkColor, weight };
  useEffect(() => {
    if (tool === "pencil") {
      const { w, h } = editor.getViewportScreenBounds();
      const center = editor.getViewportPageBounds().center;
      editor.setCamera(
        { x: w / 2 - center.x, y: h / 2 - center.y, z: 1 },
        { animation: { duration: 150 } },
      );
      editor.setCameraOptions(pencilCameraOptions);
      editor.setStyleForNextShapes(DefaultColorStyle, inkRef.current.inkColor);
      editor.setStyleForNextShapes(
        DefaultSizeStyle,
        BOARD_WEIGHTS[inkRef.current.weight].size,
      );
    } else {
      editor.setCameraOptions(boardCameraOptions);
      if (tool === "arrow") {
        editor.setStyleForNextShapes(DefaultColorStyle, "link");
        editor.setStyleForNextShapes(DefaultSizeStyle, "s");
      }
    }
  }, [tool, editor]);

  // Hold the plane ~400ms → insert popover anchored at that point; any tap on the plane
  // dismisses whatever floats (the handoff's "tap elsewhere dismisses").
  useEffect(() => {
    function onEvent(info: TLEventInfo) {
      if (info.name === "pointer_down") {
        setHold(null);
        setPickerOpen(false);
        return;
      }
      if (
        info.name === "long_press" &&
        "target" in info &&
        info.target === "canvas" &&
        editor.getCurrentToolId() === "select"
      ) {
        const page = { ...editor.inputs.currentPagePoint };
        const viewport = editor.pageToViewport(page);
        setHold({ page, viewport: { x: viewport.x, y: viewport.y } });
      }
    }
    editor.on("event", onEvent);
    return () => {
      editor.off("event", onEvent);
    };
  }, [editor]);

  const pickTool = (next: BoardTool) => {
    editor.setCurrentTool(tldrawToolId(next, subTool));
  };

  const pickSubTool = (next: PencilSubTool) => {
    setLastSubTool(next);
    editor.setCurrentTool(tldrawToolId("pencil", next));
  };

  const pickColor = (color: BoardInkColor) => {
    setInkColor(color);
    applyInkStyles(color, weight);
  };

  const pickWeight = (next: BoardWeightIndex) => {
    setWeight(next);
    applyInkStyles(inkColor, next);
  };

  // Select-mode "Ink": a Caveat text label, editing immediately. Real drawn ink is strokes
  // via the pencil — this is the heading/legend affordance.
  const addInk = (at?: VecLike) => {
    const center = at ?? editor.getViewportPageBounds().center;
    const id = createShapeId();
    editor.createShape({
      id,
      type: "text",
      x: center.x - 60,
      y: center.y - 24,
      props: { font: "draw", color: inkColor, size: "l", richText: toRichText("") },
    });
    editor.setCurrentTool("select");
    editor.select(id);
    editor.setEditingShape(id);
  };

  const pasteAsExcerpt = async () => {
    setPickerOpen(false);
    try {
      const text = (await navigator.clipboard.readText()).trim();
      if (text) addExcerpt(editor, { text });
    } catch {
      // Clipboard permission denied — ⌘V still lands through the external-content handler.
    }
  };

  const holdRows: InsertRow[] = hold
    ? [
        {
          key: "ink",
          icon: <DockIcon d={DOCK_PATHS.pencil} size={17} strokeWidth={1.8} />,
          title: "Ink",
          meta: "create",
          onPick: () => {
            addInk(hold.page);
            setHold(null);
          },
        },
        {
          key: "task",
          icon: <DockIcon d={DOCK_PATHS.select} size={17} strokeWidth={1.8} />,
          title: "Task",
          meta: "create",
          onPick: () => {
            addTask(editor, hold.page);
            setHold(null);
          },
        },
        {
          key: "card",
          icon: <DockIcon d={DOCK_PATHS.clipboard} size={17} strokeWidth={1.8} />,
          title: "Card",
          meta: "create",
          onPick: () => {
            addCard(editor, hold.page);
            setHold(null);
          },
        },
      ]
    : [];

  return (
    <div className="pointer-events-none absolute inset-0 font-display">
      <SelectionBar />
      <Dock
        tool={tool}
        subTool={subTool}
        inkColor={inkColor}
        weight={weight}
        insertOpen={pickerOpen}
        onPickTool={pickTool}
        onPickSubTool={pickSubTool}
        onPickColor={pickColor}
        onPickWeight={pickWeight}
        onToggleInsert={() => setPickerOpen((open) => !open)}
        onAddInk={() => addInk()}
        onAddTask={() => addTask(editor)}
        onAddCard={() => addCard(editor)}
      />
      {tool === "pencil" ? <HintPill text={PENCIL_HINTS[subTool]} /> : <ZoomPill />}
      {pickerOpen ? (
        <PickerPanel
          rows={(() => {
            const onBoard = boardArtifactIds(editor);
            return (pickerArtifacts ?? []).map((artifact): InsertRow => {
              const existing = onBoard.get(artifact.id);
              const iconPath =
                (DOCK_PATHS as Record<string, string>)[artifact.kind] ?? DOCK_PATHS.file;
              return {
                key: artifact.id,
                icon: <DockIcon d={iconPath} size={17} strokeWidth={1.75} />,
                title: artifact.title,
                meta: existing ? "on this board" : artifact.meta,
                metaTone: existing ? "amber" : undefined,
                onPick: existing
                  ? () => {
                      setPickerOpen(false);
                      editor.select(existing);
                      editor.zoomToSelection({ animation: { duration: 220 } });
                    }
                  : () => {
                      setPickerOpen(false);
                      addDocWindow(editor, {
                        artifactId: artifact.id,
                        artifactKind: artifact.kind,
                        title: artifact.title,
                      });
                    },
              };
            });
          })()}
          emptyNote={
            !contentSource
              ? "no workspace in this host"
              : pickerArtifacts === null
                ? "listing this folder…"
                : pickerArtifacts.length === 0
                  ? "nothing insertable in this folder yet"
                  : null
          }
          onPaste={() => void pasteAsExcerpt()}
          onClose={() => setPickerOpen(false)}
        />
      ) : null}
      {hold ? (
        <InsertPopover
          x={hold.viewport.x}
          y={hold.viewport.y}
          rows={holdRows}
          footer="held the board · same list as ⌘K"
        />
      ) : null}
    </div>
  );
}

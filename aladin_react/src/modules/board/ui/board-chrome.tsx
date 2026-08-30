import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  DefaultColorStyle,
  DefaultDashStyle,
  DefaultFontStyle,
  DefaultSizeStyle,
  createShapeId,
  toRichText,
  useEditor,
  useValue,
  type TLEventInfo,
  type VecLike,
} from "tldraw";

import { TAP_SLOP_PX, boardCameraOptions } from "../domain/board-camera";
import { useBoardAppearance } from "../domain/board-appearance";
import { useBoardContent, useBoardFolder, type PickerArtifact } from "../domain/board-content";
import { createTapTracker } from "../domain/board-gestures";
import { useBoardHost } from "../domain/board-host";
import { useBoardPaper } from "../domain/board-paper";
import { addCard, addDocWindow, addExcerpt, addLink, addStickyNote, addTask, boardArtifactIds } from "../domain/board-objects";
import { pastedUrl } from "../domain/board-links";
import { resolveLinkInto } from "../domain/board-link-flow";
import {
  browserPrefsStorage,
  loadBoardPrefs,
  saveBoardPrefs,
  type BoardToolPrefs,
} from "../domain/board-prefs";
import type { BoardInkColor } from "../domain/board-theme";
import { useBoardToasts } from "../domain/board-toasts";
import {
  BOARD_WEIGHTS,
  boardToolFromTldraw,
  isBoardToolId,
  tldrawToolId,
  type BoardTool,
  type BoardWeightIndex,
  type PencilSubTool,
} from "../domain/board-tools";
import { BackToContent } from "./back-to-content";
import { CitePill } from "./cite-pill";
import { BoardToastView } from "./board-toast";
import { Dock } from "./dock";
import { DOCK_PATHS, DockIcon } from "./dock-icons";
import { InsertPopover, type InsertRow } from "./insert-popover";
import { EmptyHint } from "./empty-hint";
import { PickerPanel, type PickerNote } from "./picker-panel";
import { SelectionBar } from "./selection-bar";
import { StatusPill } from "./status-pill";

/**
 * The board's floating chrome, mounted via `components.InFrontOfTheCanvas` so it lives
 * inside the editor context. tldraw's `getCurrentToolId()` is the single source of truth
 * for which tool is active — the dock derives its lit state from it and only remembers
 * the last pencil sub-tool, ink color and weight (persisted as the board prefs).
 */
export function BoardChrome() {
  const editor = useEditor();
  const host = useBoardHost();
  const toasts = useBoardToasts();
  const paper = useBoardPaper();
  const { appearance, toggle: toggleAppearance } = useBoardAppearance();
  const toolId = useValue("toolId", () => editor.getCurrentToolId(), [editor]);
  const { tool, subTool: activeSubTool } = boardToolFromTldraw(toolId);
  const canUndo = useValue("canUndo", () => editor.getCanUndo(), [editor]);
  const canRedo = useValue("canRedo", () => editor.getCanRedo(), [editor]);
  const viewport = useValue("viewport", () => editor.getViewportScreenBounds(), [editor]);
  const penMode = useValue("penMode", () => editor.getInstanceState().isPenMode, [editor]);
  const zoom = useValue("zoom", () => editor.getZoomLevel(), [editor]);

  // Unsupported stock tools return to selection; every active tool has a rail control.
  useEffect(() => {
    if (!isBoardToolId(toolId)) editor.setCurrentTool("select");
  }, [toolId, editor]);

  // ── Prefs: hydrated once, written on every change ──
  const storage = useMemo(browserPrefsStorage, []);
  const [prefs, setPrefs] = useState<BoardToolPrefs>(() => loadBoardPrefs(storage));
  useEffect(() => saveBoardPrefs(storage, prefs), [storage, prefs]);
  const { inkColor, weight, drawWithFinger } = prefs;
  const subTool = activeSubTool ?? prefs.subTool;

  const [pickerOpen, setPickerOpen] = useState(false);
  const [pencilMenuOpen, setPencilMenuOpen] = useState(false);
  const [styleOpen, setStyleOpen] = useState(false);
  const [hold, setHold] = useState<{ page: VecLike; viewport: VecLike } | null>(null);
  const [holdRing, setHoldRing] = useState<VecLike | null>(null);

  // Picker data: the board's folder siblings, refreshed each time the panel (or the hold
  // popover, which shares the list) opens; a query also reaches the whole workspace.
  const contentSource = useBoardContent();
  const folderId = useBoardFolder();
  const [query, setQuery] = useState("");
  const [folderRows, setFolderRows] = useState<PickerArtifact[] | null>(null);
  const [folderError, setFolderError] = useState(false);
  const [folderFetch, setFolderFetch] = useState(0);
  const listOpen = pickerOpen || hold !== null;
  useEffect(() => {
    if (!listOpen || !contentSource) return;
    let alive = true;
    setFolderError(false);
    contentSource
      .listFolderArtifacts(folderId)
      .then((rows) => {
        if (alive) setFolderRows(rows);
      })
      .catch(() => {
        if (alive) {
          setFolderRows(null);
          setFolderError(true);
        }
      });
    return () => {
      alive = false;
    };
  }, [listOpen, contentSource, folderId, folderFetch]);

  const [searchRows, setSearchRows] = useState<PickerArtifact[]>([]);
  useEffect(() => {
    const q = query.trim();
    if (!pickerOpen || !contentSource || q.length < 2) {
      setSearchRows([]);
      return;
    }
    let alive = true;
    const handle = window.setTimeout(() => {
      contentSource
        .searchArtifacts(q)
        .then((rows) => {
          if (alive) setSearchRows(rows);
        })
        .catch(() => {
          if (alive) setSearchRows([]);
        });
    }, 250);
    return () => {
      alive = false;
      window.clearTimeout(handle);
    };
  }, [pickerOpen, contentSource, query]);
  useEffect(() => {
    if (!pickerOpen) setQuery("");
  }, [pickerOpen]);

  // Only this active editor handles shortcuts; hidden keep-alive boards never react.
  useEffect(() => {
    const container = editor.getContainer();
    const onKey = (e: KeyboardEvent) => {
      if (!editor.getInstanceState().isFocused || e.defaultPrevented) return;
      if (e.key === "Escape") {
        setPencilMenuOpen(false); setPickerOpen(false); setStyleOpen(false);
        setHold(null); setHoldRing(null);
        editor.setCurrentTool("select").selectNone(); return;
      }
      if ((e.target as HTMLElement).closest("input, textarea, [contenteditable='true']")) return;
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k" && !e.defaultPrevented) {
        e.preventDefault();
        setPencilMenuOpen(false);
        setStyleOpen(false);
        setPickerOpen((open) => !open);
        return;
      }
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "z") {
        e.preventDefault(); if (e.shiftKey) editor.redo(); else editor.undo(); return;
      }
      if (e.metaKey || e.ctrlKey || e.altKey) return;
      const shortcuts: Record<string, string> = { v: "select", h: "hand", p: "draw", a: "arrow", t: "text", f: "frame" };
      if (shortcuts[e.key.toLowerCase()]) {
        setPencilMenuOpen(false); setPickerOpen(false); setStyleOpen(false);
        editor.setCurrentTool(shortcuts[e.key.toLowerCase()]);
      }
      if (e.key.toLowerCase() === "n") addStickyNote(editor);
      if (e.key === "Delete" || e.key === "Backspace") {
        e.preventDefault(); editor.markHistoryStoppingPoint("delete selection"); editor.deleteShapes(editor.getSelectedShapeIds());
      }
    };
    container.addEventListener("keydown", onKey);
    return () => container.removeEventListener("keydown", onKey);
  }, [editor]);

  const applyInkStyles = useCallback(
    (color: BoardInkColor, weightIndex: BoardWeightIndex) => {
      editor.setStyleForNextShapes(DefaultColorStyle, color);
      editor.setStyleForNextShapes(DefaultSizeStyle, BOARD_WEIGHTS[weightIndex].size);
    },
    [editor],
  );

  // Tools change styles, never the plane's camera. Paper retains its host constraints.
  const inkRef = useRef({ inkColor, weight });
  inkRef.current = { inkColor, weight };
  useEffect(() => {
    if (!paper.paged) editor.setCameraOptions(boardCameraOptions);
    editor.setStyleForNextShapes(DefaultDashStyle, "solid");
    if (tool === "pencil") {
      editor.setStyleForNextShapes(DefaultColorStyle, inkRef.current.inkColor);
      editor.setStyleForNextShapes(
        DefaultSizeStyle,
        BOARD_WEIGHTS[inkRef.current.weight].size,
      );
    } else {
      setStyleOpen(false);
      setPencilMenuOpen(false);
      editor.setStyleForNextShapes(DefaultColorStyle, tool === "arrow" ? "link" : tool === "note" ? "yellow" : "black");
      editor.setStyleForNextShapes(DefaultSizeStyle, "s");
      editor.setStyleForNextShapes(DefaultFontStyle, tool === "note" ? "draw" : "sans");
    }
  }, [tool, editor, paper.paged]);

  // ── Finger ≠ Pencil ──
  // tldraw flips `isPenMode` on by itself at the first direct-Pencil touch and then drops
  // EVERY finger event in EVERY tool — a finger could no longer select a card. The board
  // owns the flag instead: pen mode only inside the pencil tools, only once a Pencil has
  // been seen, and never when the user asked for a finger that draws. In select and arrow a
  // finger always works.
  const [penSeen, setPenSeen] = useState(false);
  const wantPenMode = tool === "pencil" && penSeen && !drawWithFinger;
  useEffect(() => {
    if (penMode !== wantPenMode) editor.updateInstanceState({ isPenMode: wantPenMode });
  }, [editor, penMode, wantPenMode]);

  // ── Events from the plane: hold ring, hold-to-insert, dismissals ──
  useEffect(() => {
    function onEvent(info: TLEventInfo) {
      if (info.name === "pointer_down") {
        if ("isPen" in info && info.isPen) setPenSeen(true);
        setHold(null);
        setPickerOpen(false);
        setPencilMenuOpen(false);
        setStyleOpen(false);
        const currentTool = editor.getCurrentToolId();
        if (
          currentTool === "select" &&
          "target" in info &&
          info.target === "canvas" &&
          !editor.inputs.getIsPanning()
        ) {
          const origin = editor.inputs.getOriginScreenPoint();
          setHoldRing({ x: origin.x, y: origin.y });
        }
        return;
      }
      if (info.name === "pointer_move") {
        if (holdRingRef.current) {
          const origin = editor.inputs.getOriginScreenPoint();
          const now = editor.inputs.getCurrentScreenPoint();
          if (Math.hypot(now.x - origin.x, now.y - origin.y) > TAP_SLOP_PX) setHoldRing(null);
        }
        return;
      }
      if (info.name === "pointer_up" || info.name === "cancel" || info.name === "complete") {
        setHoldRing(null);
        return;
      }
      if (
        info.name === "long_press" &&
        "target" in info &&
        info.target === "canvas" &&
        editor.getCurrentToolId() === "select"
      ) {
        // The point the press BEGAN at, not where the finger drifted to over 400ms.
        const page = editor.inputs.getOriginPagePoint().clone();
        const viewport = editor.pageToViewport(page);
        setHoldRing(null);
        setHold({ page, viewport: { x: viewport.x, y: viewport.y } });
      }
    }
    editor.on("event", onEvent);
    return () => {
      editor.off("event", onEvent);
    };
  }, [editor]);
  const holdRingRef = useRef(holdRing);
  holdRingRef.current = holdRing;

  // ── Multi-finger taps (undo / redo) and the one-finger pan in pen mode ──
  // Touch listeners on tldraw's container, parallel to its pointer handling: tldraw drops a
  // finger's pointer events while in pen mode, which is exactly when the finger should pan.
  //
  // THE PENCIL IS A TOUCH TOO. On iOS the Apple Pencil raises the same TouchEvents with
  // `touchType: "stylus"` — treat it as a finger and the pan follows the drawing tip, so the
  // canvas slides under the stroke and ink piles up at the pen ("the ink moves with the
  // pencil", found on device 2026-08-24). Stylus touches are therefore invisible here: they
  // never count toward taps, never pan, and an active stylus suspends the finger-pan (a
  // camera move mid-stroke would displace the ink being laid down).
  const wantPenModeRef = useRef(wantPenMode);
  wantPenModeRef.current = wantPenMode;
  useEffect(() => {
    const container = editor.getContainer();
    const taps = createTapTracker({
      onTap: (fingers) => {
        if (fingers === 2 && editor.getCanUndo()) {
          editor.undo();
          host.haptic?.("light");
        } else if (fingers === 3 && editor.getCanRedo()) {
          editor.redo();
          host.haptic?.("light");
        }
      },
    });
    let pan: { id: number; x: number; y: number } | null = null;
    const stylusIds = new Set<number>();

    const isStylus = (t: Touch) =>
      (t as Touch & { touchType?: string }).touchType === "stylus";
    const fingersOf = (list: TouchList) => Array.from(list).filter((t) => !isStylus(t));

    const onStart = (e: TouchEvent) => {
      for (const t of Array.from(e.changedTouches)) {
        if (isStylus(t)) stylusIds.add(t.identifier);
        else taps.start(t.identifier, t.clientX, t.clientY, e.timeStamp);
      }
      const fingers = fingersOf(e.touches);
      if (wantPenModeRef.current && stylusIds.size === 0 && fingers.length === 1) {
        pan = { id: fingers[0].identifier, x: fingers[0].clientX, y: fingers[0].clientY };
      } else {
        pan = null;
      }
    };
    const onMove = (e: TouchEvent) => {
      for (const t of Array.from(e.changedTouches)) {
        if (!isStylus(t)) taps.move(t.identifier, t.clientX, t.clientY);
      }
      const fingers = fingersOf(e.touches);
      if (
        pan &&
        stylusIds.size === 0 &&
        fingers.length === 1 &&
        fingers[0].identifier === pan.id
      ) {
        const t = fingers[0];
        const dx = t.clientX - pan.x;
        const dy = t.clientY - pan.y;
        pan = { id: pan.id, x: t.clientX, y: t.clientY };
        const cam = editor.getCamera();
        editor.stopCameraAnimation();
        editor.setCamera({ x: cam.x + dx / cam.z, y: cam.y + dy / cam.z, z: cam.z });
      } else if (fingers.length !== 1 || stylusIds.size > 0) {
        pan = null;
      }
    };
    const onEnd = (e: TouchEvent) => {
      for (const t of Array.from(e.changedTouches)) {
        if (stylusIds.has(t.identifier)) stylusIds.delete(t.identifier);
        else taps.end(t.identifier, e.timeStamp);
      }
      if (fingersOf(e.touches).length === 0) pan = null;
    };
    const onCancel = (e: TouchEvent) => {
      for (const t of Array.from(e.changedTouches)) stylusIds.delete(t.identifier);
      taps.cancel();
      pan = null;
    };
    container.addEventListener("touchstart", onStart, { passive: true });
    container.addEventListener("touchmove", onMove, { passive: true });
    container.addEventListener("touchend", onEnd, { passive: true });
    container.addEventListener("touchcancel", onCancel, { passive: true });
    return () => {
      container.removeEventListener("touchstart", onStart);
      container.removeEventListener("touchmove", onMove);
      container.removeEventListener("touchend", onEnd);
      container.removeEventListener("touchcancel", onCancel);
    };
  }, [editor, host]);

  const pickTool = (next: BoardTool) => {
    setHold(null); setHoldRing(null);
    if (next === "pencil" && tool === "pencil") {
      setPencilMenuOpen(!pencilMenuOpen);
      if (pencilMenuOpen) setStyleOpen(false);
      setPickerOpen(false);
      return;
    }
    if (next !== tool) host.haptic?.("select");
    setPickerOpen(false);
    setPencilMenuOpen(next === "pencil");
    editor.setCurrentTool(tldrawToolId(next, subTool));
  };

  const pickSubTool = (next: PencilSubTool) => {
    if (next !== subTool) host.haptic?.("select");
    setPrefs((p) => ({ ...p, subTool: next }));
    setPencilMenuOpen(false);
    setStyleOpen(false);
    editor.setCurrentTool(tldrawToolId("pencil", next));
  };

  const pickColor = (color: BoardInkColor) => {
    setPrefs((p) => ({ ...p, inkColor: color }));
    applyInkStyles(color, weight);
    host.haptic?.("select");
  };

  const pickWeight = (next: BoardWeightIndex) => {
    setPrefs((p) => ({ ...p, weight: next }));
    applyInkStyles(inkColor, next);
    host.haptic?.("select");
  };

  const toggleDrawWithFinger = () => {
    setPrefs((p) => ({ ...p, drawWithFinger: !p.drawWithFinger }));
    host.haptic?.("select");
  };

  const inserted = () => host.haptic?.("light");

  // Select-mode "Ink": a Caveat text label, editing immediately. Real drawn ink is strokes
  // via the pencil — this is the heading/legend affordance.
  const addInk = (at?: VecLike) => {
    const center = at ?? editor.getViewportPageBounds().center;
    const id = createShapeId();
    editor.createShape({
      id,
      type: "text",
      x: center.x,
      y: center.y,
      props: { font: "draw", color: inkColor, size: "l", richText: toRichText("") },
    });
    // Centre on the measured box, not a guess at the label's size.
    const bounds = editor.getShapePageBounds(id);
    if (bounds) {
      editor.updateShape({ id, type: "text", x: center.x - bounds.w / 2, y: center.y - bounds.h / 2 });
    }
    editor.setCurrentTool("select");
    editor.select(id);
    editor.setEditingShape(id);
    inserted();
  };

  const pasteAsExcerpt = async () => {
    setPickerOpen(false);
    try {
      const text = (await navigator.clipboard.readText()).trim();
      const url = pastedUrl(text);
      if (url) {
        const id = addLink(editor, { url });
        resolveLinkInto(editor, contentSource, id, url);
        inserted();
      } else if (text) {
        addExcerpt(editor, { text });
        inserted();
      } else {
        toasts.show({ text: "Nothing to paste — the clipboard is empty" });
      }
    } catch {
      // Clipboard permission denied — ⌘V still lands through the external-content handler.
      toasts.show({ text: "Allow paste for this site, or press ⌘V on the board" });
    }
  };

  const holdRows: InsertRow[] = hold
    ? [
        ...(folderRows ?? []).map((artifact): InsertRow => {
          const onBoard = boardArtifactIds(editor);
          const existing = onBoard.get(artifact.id);
          return {
            key: artifact.id,
            icon: (
              <DockIcon
                d={(DOCK_PATHS as Record<string, string>)[artifact.kind] ?? DOCK_PATHS.file}
                size={17}
                strokeWidth={1.75}
              />
            ),
            title: artifact.title,
            meta: existing ? "on this board" : artifact.meta,
            metaTone: existing ? "amber" : undefined,
            onPick: () => {
              setHold(null);
              if (existing) {
                editor.select(existing);
                editor.zoomToSelection({ animation: { duration: 220 } });
                return;
              }
              addDocWindow(editor, {
                artifactId: artifact.id,
                artifactKind: artifact.kind,
                title: artifact.title,
                at: hold.page,
              });
              inserted();
            },
          };
        }),
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
          icon: <DockIcon d={DOCK_PATHS.task} size={17} strokeWidth={1.8} />,
          title: "Task",
          meta: "create",
          onPick: () => {
            addTask(editor, hold.page);
            inserted();
            setHold(null);
          },
        },
        {
          key: "card",
          icon: <DockIcon d={DOCK_PATHS.card} size={17} strokeWidth={1.8} />,
          title: "Card",
          meta: "create",
          onPick: () => {
            addCard(editor, hold.page);
            inserted();
            setHold(null);
          },
        },
      ]
    : [];

  // Picker rows — memoised: `boardArtifactIds` walks every shape, and the chrome re-renders
  // on every tool and zoom tick.
  const pickerRows = useMemo<InsertRow[]>(() => {
    if (!pickerOpen) return [];
    const onBoard = boardArtifactIds(editor);
    const q = query.trim().toLowerCase();
    const inFolder = (folderRows ?? []).filter(
      (a) => !q || a.title.toLowerCase().includes(q) || a.meta.toLowerCase().includes(q),
    );
    const folderIds = new Set((folderRows ?? []).map((a) => a.id));
    const elsewhere = searchRows.filter((a) => !folderIds.has(a.id));
    return [...inFolder, ...elsewhere].map((artifact): InsertRow => {
      const existing = onBoard.get(artifact.id);
      const iconPath = (DOCK_PATHS as Record<string, string>)[artifact.kind] ?? DOCK_PATHS.file;
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
              inserted();
            },
      };
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps -- `inserted` is a stable host call
  }, [pickerOpen, folderRows, searchRows, query, editor]);

  const pickerNote: PickerNote = !contentSource
    ? { kind: "info", text: "no workspace in this host" }
    : folderError
      ? {
          kind: "error",
          text: "couldn't list this folder",
          onRetry: () => setFolderFetch((n) => n + 1),
        }
      : folderRows === null
        ? { kind: "info", text: "listing this folder…" }
        : pickerRows.length === 0
          ? {
              kind: "info",
              text: query.trim()
                ? query.trim().length < 2
                  ? "keep typing to search everywhere"
                  : "nothing matches — here or anywhere"
                : "nothing insertable in this folder yet",
            }
          : { kind: "none" };

  return (
    <div
      onPointerDown={(event) => event.stopPropagation()}
      className="board-chrome rs-chrome pointer-events-none absolute inset-0"
    >
      <SelectionBar hidden={pickerOpen || pencilMenuOpen || hold !== null} />
      {paper.paged ? <CitePill /> : <BackToContent />}
      {paper.paged ? null : <EmptyHint />}
      <Dock
        tool={tool}
        subTool={subTool}
        inkColor={inkColor}
        weight={weight}
        drawWithFinger={drawWithFinger}
        insertOpen={pickerOpen}
        pencilMenuOpen={pencilMenuOpen}
        styleOpen={styleOpen}
        zoomPct={Math.round(zoom * 100)}
        zoomLocked={paper.paged}
        appearance={appearance}
        onToggleAppearance={toggleAppearance}
        canUndo={canUndo}
        canRedo={canRedo}
        onUndo={() => editor.undo()}
        onRedo={() => editor.redo()}
        onPickTool={pickTool}
        onPickSubTool={pickSubTool}
        onPickColor={pickColor}
        onPickWeight={pickWeight}
        onToggleDrawWithFinger={toggleDrawWithFinger}
        onToggleInsert={() => {
          setHold(null); setHoldRing(null);
          setPencilMenuOpen(false);
          setStyleOpen(false);
          setPickerOpen((open) => !open);
        }}
        onToggleStyle={() => {
          setPickerOpen(false);
          setStyleOpen((open) => !open);
        }}
        onZoomIn={() => editor.zoomIn()}
        onZoomOut={() => editor.zoomOut()}
        onResetZoom={() => editor.resetZoom()}
        onFit={() => { const bounds = editor.getCurrentPageBounds(); if (bounds) editor.zoomToBounds(bounds, { inset: 90, animation: { duration: 250 } }); }}
        onAddNote={() => { setPickerOpen(false); setPencilMenuOpen(false); setHold(null); setHoldRing(null); addStickyNote(editor); inserted(); }}
      />
      <StatusPill />
      <BoardToastView />
      {pickerOpen ? (
        <PickerPanel
          query={query}
          onQueryChange={setQuery}
          rows={pickerRows}
          note={pickerNote}
          onPaste={() => void pasteAsExcerpt()}
          onClose={() => setPickerOpen(false)}
          onAddLink={(url) => { setPickerOpen(false); const id = addLink(editor, { url }); resolveLinkInto(editor, contentSource, id, url); inserted(); }}
          onAddTask={() => { setPickerOpen(false); addTask(editor); inserted(); }}
          onAddCard={() => { setPickerOpen(false); addCard(editor); inserted(); }}
        />
      ) : null}
      {holdRing && !hold ? (
        <div className="board-hold-ring" style={{ left: holdRing.x, top: holdRing.y }} />
      ) : null}
      {hold ? (
        <InsertPopover
          x={hold.viewport.x}
          y={hold.viewport.y}
          viewportWidth={viewport.w}
          viewportHeight={viewport.h}
          rows={holdRows}
          footer="held the board · lands here · ⌘K opens the same list"
        />
      ) : null}
    </div>
  );
}

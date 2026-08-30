import { useCallback, useEffect, useState, type ReactNode } from "react";
import { ArrowUpRight, Check, ChevronRight, Copy, Eraser, Frame, Hand, Highlighter, LayoutGrid, Link2, Maximize, Minus, Moon, MousePointer2, NotebookPen, Pencil, Plus, Redo2, Search, StickyNote, Sun, Trash2, Type, Undo2, X, type LucideIcon } from "lucide-react";
import { DefaultColorStyle, DefaultDashStyle, DefaultFontStyle, DefaultSizeStyle, Tldraw, createShapeId, toRichText, useEditor, useValue, type Editor, type TLComponents, type TLDefaultColorStyle, type TLShapeId } from "tldraw";
import { buildBoardStudioTheme, useSavedBoardAppearance, useBoardThemeSync } from "../domain/board-appearance";
import { KIND_DETAILS, RESEARCH_DEFAULTS, ResearchObjectView, ResearchShapeUtil, SAMPLE_OBJECTS, type ResearchProps, type ResearchShape } from "./research-object";
import "tldraw/tldraw.css";
import "../ui/board-studio.css";

const SHAPES = [ResearchShapeUtil];
const COMPONENTS: TLComponents = { InFrontOfTheCanvas: DesignChrome };
const THEMES = { default: buildBoardStudioTheme() };

/** An intentionally isolated, local-only UI experiment. Nothing touches real boards. */
export function BoardDesignSpike() {
  const { appearance } = useSavedBoardAppearance();
  const [editor, setEditor] = useState<Editor | null>(null);
  useBoardThemeSync(editor, appearance);
  const onMount = useCallback((mounted: Editor) => { setEditor(mounted); return mountBoard(mounted); }, []);
  return <main className="research-studio" data-appearance={appearance} aria-label="Research board design spike">
      <Tldraw hideUi shapeUtils={SHAPES} components={COMPONENTS} themes={THEMES} onMount={onMount} />
  </main>;
}

function mountBoard(editor: Editor) {
  editor.setStyleForNextShapes(DefaultFontStyle, "sans");
  editor.setStyleForNextShapes(DefaultSizeStyle, "s");
  editor.setStyleForNextShapes(DefaultDashStyle, "solid");
  editor.setCameraOptions({ wheelBehavior: "pan" });
  seedResearchBoard(editor);
  editor.registerExternalContentHandler("url", ({ url, point }) => {
    let source = "Saved link";
    try { source = new URL(url).hostname; } catch { /* Keep a readable fallback. */ }
    addResearchObject(editor, { ...RESEARCH_DEFAULTS, kind: "link", title: source, body: url, source, tint: "neutral" }, point);
  });
  editor.registerExternalContentHandler("text", ({ text, point }) => {
    addResearchObject(editor, { ...RESEARCH_DEFAULTS, title: "Captured thought", body: text }, point);
  });
  const fit = requestAnimationFrame(() => editor.zoomToBounds({ x: -145, y: -95, w: 1375, h: 910 }, { inset: 24, targetZoom: 1 }));
  return () => cancelAnimationFrame(fit);
}

export function seedResearchBoard(editor: Editor) {
  if (editor.getCurrentPageShapeIds().size > 0) return;
  const frameId = createShapeId("design-working-notes");
  editor.createShape({ id: frameId, type: "frame", x: 373, y: 131, props: { w: 368, h: 570, name: "WORKING NOTES" } });
  for (const object of SAMPLE_OBJECTS) editor.createShape<ResearchShape>({ id: createShapeId(`design-${object.id}`), type: "research-spike-object", x: object.x, y: object.y, props: object.props });
  editor.reparentShapes([createShapeId("design-method"), createShapeId("design-question")], frameId);
  const text = (id: string, value: string, x: number, y: number, size: "s" | "m" | "l" | "xl", font: "sans" | "draw" = "sans", color: TLDefaultColorStyle = "black") => editor.createShape({ id: createShapeId(id), type: "text", x, y, props: { richText: toRichText(value), size, font, color, autoSize: true } });
  text("design-title", "Following the spread", 0, -8, "xl");
  text("design-subtitle", "A place to collect, connect, and think out loud.", 2, 51, "s", "sans", "grey");
  text("design-heading-reading", "READING ROOM", 0, 108, "s", "sans", "grey");
  text("design-heading-experiments", "TRY IT OUT", 829, 125, "s", "sans", "grey");
  text("design-handwriting", "test the relationship,\nnot just the backtest.", 443, 719, "m", "draw", "blue");
  const connect = (from: string, to: string, label: string, bend = 0) => {
    const id = createShapeId(`design-${from}-${to}`);
    editor.createShape({ id, type: "arrow", props: { start: { x: 0, y: 0 }, end: { x: 100, y: 0 }, richText: toRichText(label), color: "grey", size: "s", dash: "solid", font: "sans", bend } });
    editor.createBindings([
      { type: "arrow", fromId: id, toId: createShapeId(`design-${from}`), props: { terminal: "start", normalizedAnchor: { x: 1, y: 0.5 }, isExact: false, isPrecise: true } },
      { type: "arrow", fromId: id, toId: createShapeId(`design-${to}`), props: { terminal: "end", normalizedAnchor: { x: 0, y: 0.5 }, isExact: false, isPrecise: true } },
    ]);
  };
  connect("paper", "method", "unpack");
  connect("method", "spread", "explore");
  connect("question", "repo", "test", 25);
  editor.selectNone();
  editor.clearHistory();
}

function addResearchObject(editor: Editor, props: ResearchProps, at?: { x: number; y: number }) {
  const center = at ?? editor.getViewportPageBounds().center;
  const id = createShapeId();
  editor.markHistoryStoppingPoint("add research object");
  editor.createShape<ResearchShape>({ id, type: "research-spike-object", x: center.x - props.w / 2, y: center.y - props.h / 2, props });
  editor.setCurrentTool("select").select(id);
  return id;
}

function IconButton({ label, icon: Icon, active, children, ...props }: { label: string; icon: LucideIcon; active?: boolean; children?: ReactNode } & React.ButtonHTMLAttributes<HTMLButtonElement>) {
  return <button type="button" className="rs-icon-button" aria-label={label} aria-pressed={active} {...props}><Icon size={19} strokeWidth={1.65} />{children}</button>;
}

function DesignChrome() {
  const editor = useEditor();
  const { appearance, toggle: toggleTheme } = useSavedBoardAppearance();
  const tool = useValue("design-tool", () => editor.getCurrentToolId(), [editor]);
  const zoom = useValue("design-zoom", () => Math.round(editor.getZoomLevel() * 100), [editor]);
  const canUndo = useValue("design-undo", () => editor.getCanUndo(), [editor]);
  const canRedo = useValue("design-redo", () => editor.getCanRedo(), [editor]);
  const selected = useValue("design-selection", () => editor.getSelectedShapes(), [editor]);
  const selectionBounds = useValue("design-selection-bounds", () => {
    const bounds = editor.getSelectionPageBounds();
    if (!bounds) return null;
    return editor.pageToViewport({ x: bounds.midX, y: bounds.minY });
  }, [editor]);
  const idle = useValue("design-idle", () => editor.isIn("select.idle"), [editor]);
  const [palette, setPalette] = useState(false);
  const [library, setLibrary] = useState(false);
  const [detailsId, setDetailsId] = useState<TLShapeId | null>(null);
  const [color, setColor] = useState<TLDefaultColorStyle>("blue");
  const [search, setSearch] = useState("");
  const [link, setLink] = useState("");
  const [linkError, setLinkError] = useState("");
  const details = useValue("design-details", () => detailsId ? editor.getShape<ResearchShape>(detailsId) : undefined, [editor, detailsId]);
  const isInk = ["draw", "highlight", "eraser"].includes(tool);

  useEffect(() => {
    const dismiss = (event: { name: string }) => {
      if (event.name === "pointer_down") { setPalette(false); setLibrary(false); setDetailsId(null); }
    };
    editor.on("event", dismiss);
    return () => { editor.off("event", dismiss); };
  }, [editor]);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") { setPalette(false); setLibrary(false); setDetailsId(null); editor.setCurrentTool("select").selectNone(); return; }
      if ((event.target as HTMLElement).closest("input, textarea, [contenteditable='true']")) return;
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "z") {
        event.preventDefault();
        if (event.shiftKey) editor.redo(); else editor.undo();
        return;
      }
      if (event.metaKey || event.ctrlKey || event.altKey) return;
      if (event.key === "Delete" || event.key === "Backspace") {
        event.preventDefault(); editor.markHistoryStoppingPoint("delete selection"); editor.deleteShapes(editor.getSelectedShapeIds()); return;
      }
      const shortcuts: Record<string, string> = { v: "select", h: "hand", p: "draw", a: "arrow", t: "text", f: "frame" };
      if (shortcuts[event.key.toLowerCase()]) { editor.setCurrentTool(shortcuts[event.key.toLowerCase()]); setPalette(false); }
      if (event.key.toLowerCase() === "n") addResearchObject(editor, { ...RESEARCH_DEFAULTS, title: "New thought" });
    };
    const container = editor.getContainer();
    container.addEventListener("keydown", onKey);
    return () => container.removeEventListener("keydown", onKey);
  }, [editor]);

  function pickTool(next: string) {
    setLibrary(false); setDetailsId(null);
    setPalette(next === "draw" ? !palette : false);
    if (next === "draw") editor.setStyleForNextShapes(DefaultColorStyle, color);
    else editor.setStyleForNextShapes(DefaultColorStyle, "black");
    editor.setCurrentTool(next);
  }

  const add = (props: ResearchProps) => { addResearchObject(editor, props); setLibrary(false); };
  const selectedObject = selected.length === 1 && selected[0].type === "research-spike-object" ? selected[0] as ResearchShape : null;

  return <div className="rs-chrome" onPointerDown={(event) => event.stopPropagation()}>
    <header className="rs-header">
      <div className="rs-board-identity"><span className="rs-brand-mark">a<span>·</span></span><span className="rs-header-divider" /><span className="rs-workspace-name">Research</span><ChevronRight size={13} /><span className="rs-board-name">Pairs trading</span></div>
      <div className="rs-header-actions"><span className="rs-spike-label">DESIGN SPIKE</span><IconButton label={appearance === "light" ? "Use dark board" : "Use light board"} icon={appearance === "light" ? Moon : Sun} onClick={toggleTheme} /><button className="rs-library-button" aria-expanded={library} onClick={() => { setLibrary(!library); setPalette(false); setDetailsId(null); }}><LayoutGrid size={16} /> Library</button></div>
    </header>

    <div className="rs-tool-rail rs-surface" role="toolbar" aria-label="Creation tools">
      <IconButton label="Select" icon={MousePointer2} active={tool === "select"} onClick={() => pickTool("select")} />
      <IconButton label="Pan" icon={Hand} active={tool === "hand"} onClick={() => pickTool("hand")} />
      <span className="rs-separator" />
      <IconButton label="Sticky note" icon={StickyNote} onClick={() => { setPalette(false); add({ ...RESEARCH_DEFAULTS, title: "New thought" }); }} />
      <IconButton label="Text" icon={Type} active={tool === "text"} onClick={() => pickTool("text")} />
      <div className="rs-tool-anchor"><IconButton label="Pencil" icon={Pencil} active={isInk} aria-expanded={palette} onClick={() => pickTool("draw")} /><span className="rs-tool-dot" /></div>
      <IconButton label="Connect" icon={Link2} active={tool === "arrow"} onClick={() => pickTool("arrow")} />
      <IconButton label="Frame" icon={Frame} active={tool === "frame"} onClick={() => pickTool("frame")} />
      <span className="rs-separator" />
      <IconButton label="Add to board" icon={Plus} active={library} onClick={() => { setLibrary(!library); setPalette(false); setDetailsId(null); }} />
    </div>

    {palette && <div className="rs-pencil-popover rs-surface" role="toolbar" aria-label="Pencil options">
      <div className="rs-inline-tools">{([{ id: "draw", label: "Pen", icon: Pencil }, { id: "highlight", label: "Highlighter", icon: Highlighter }, { id: "eraser", label: "Eraser", icon: Eraser }]).map(({ id, label, icon }) => <IconButton key={id} label={label} icon={icon} active={tool === id} onClick={() => { editor.setCurrentTool(id); setPalette(false); }} />)}</div>
      <div className="rs-swatches">{(["black", "blue", "green", "violet", "orange"] as const).map((value) => <button key={value} aria-label={`${value} ink`} aria-pressed={color === value} className={`rs-swatch rs-ink--${value}`} onClick={() => { setColor(value); editor.setStyleForNextShapes(DefaultColorStyle, value); }}>{color === value && <Check size={12} />}</button>)}</div>
    </div>}

    {library && <aside className="rs-library rs-surface" aria-label="Board library">
      <div className="rs-panel-heading"><div><h2>Add to your thinking</h2><p>Bring the useful pieces together.</p></div><IconButton label="Close library" icon={X} onClick={() => setLibrary(false)} /></div>
      <label className="rs-search"><Search size={16} /><input aria-label="Find in library" placeholder="Find a source, note, instrument…" value={search} onChange={(event) => setSearch(event.target.value)} /></label>
      <div className="rs-section-label">FROM YOUR WORKSPACE <span>Sample objects</span></div>
      <div className="rs-library-list">{SAMPLE_OBJECTS.filter((item) => item.props.kind !== "note" && `${item.props.title} ${item.props.source}`.toLowerCase().includes(search.toLowerCase())).map(({ id, props }) => {
        const KindIcon = KIND_DETAILS[props.kind].icon;
        return <button key={id} className="rs-library-item" onClick={() => add({ ...props })}><span className={`rs-library-item-icon rs-kind--${props.kind}`}><KindIcon size={19} strokeWidth={1.5} /></span><span><strong>{props.title}</strong><small>{KIND_DETAILS[props.kind].label} · {props.source.split("/")[0]}</small></span><Plus size={15} /></button>;
      })}{!SAMPLE_OBJECTS.some((item) => item.props.kind !== "note" && `${item.props.title} ${item.props.source}`.toLowerCase().includes(search.toLowerCase())) && <p className="rs-empty">No matching objects.</p>}</div>
      <form className="rs-link-form" onSubmit={(event) => {
        event.preventDefault();
        try { const url = new URL(link); if (!["http:", "https:"].includes(url.protocol)) throw new Error(); add({ ...RESEARCH_DEFAULTS, kind: "link", title: url.hostname, body: url.href, source: url.hostname, tint: "neutral" }); setLink(""); setLinkError(""); }
        catch { setLinkError("Use a complete http or https link."); }
      }}><label htmlFor="rs-url">Or drop in a link</label><div><input id="rs-url" placeholder="https://…" value={link} onChange={(event) => setLink(event.target.value)} /><button type="submit" aria-label="Add link"><ArrowUpRight size={18} /></button></div>{linkError && <p role="alert">{linkError}</p>}</form>
    </aside>}

    {selected.length > 0 && idle && selectionBounds && !library && !details && <div className="rs-selection rs-surface" role="toolbar" aria-label="Object actions" style={{ left: `clamp(190px, ${selectionBounds.x}px, calc(100% - 190px))`, top: Math.max(70, selectionBounds.y - 54) }}>
      {selectedObject && <button className="rs-selection-edit" onClick={() => setDetailsId(selectedObject.id)}><NotebookPen size={15} />{selectedObject.props.kind === "note" ? "Edit note" : "Open"}</button>}
      {selectedObject && <><span className="rs-divider" />{(["neutral", "butter", "sage", "lilac"] as const).map((tint) => <button key={tint} className={`rs-tint-dot rs-tint--${tint}`} aria-label={`${tint} card`} aria-pressed={selectedObject.props.tint === tint} onClick={() => { editor.markHistoryStoppingPoint("change card colour"); editor.updateShape<ResearchShape>({ id: selectedObject.id, type: "research-spike-object", props: { tint } }); }} />)}</>}
      {selected.length > 1 && <button className="rs-selection-edit" onClick={() => editor.groupShapes(editor.getSelectedShapeIds())}>Group {selected.length}</button>}
      {selected.length === 1 && selected[0].type === "group" && <button className="rs-selection-edit" onClick={() => editor.ungroupShapes(editor.getSelectedShapeIds())}>Ungroup</button>}
      <span className="rs-divider" /><IconButton label="Duplicate selection" icon={Copy} onClick={() => editor.duplicateShapes(editor.getSelectedShapeIds(), { x: 24, y: 24 })} /><IconButton label="Delete selection" icon={Trash2} onClick={() => editor.deleteShapes(editor.getSelectedShapeIds())} />
    </div>}

    {details && <ObjectDetails key={details.id} shape={details} close={() => setDetailsId(null)} />}

    <div className="rs-history rs-surface" role="toolbar" aria-label="History"><IconButton label="Undo" icon={Undo2} disabled={!canUndo} onClick={() => editor.undo()} /><IconButton label="Redo" icon={Redo2} disabled={!canRedo} onClick={() => editor.redo()} /></div>
    <div className="rs-prototype-note">Sample board<span>·</span>Session-only edits</div>
    <div className="rs-zoom rs-surface" role="toolbar" aria-label="Canvas navigation"><IconButton label="Zoom out" icon={Minus} onClick={() => editor.zoomOut()} /><button className="rs-zoom-level" aria-label="Reset zoom" onClick={() => editor.resetZoom()}>{zoom}%</button><IconButton label="Zoom in" icon={Plus} onClick={() => editor.zoomIn()} /><span className="rs-divider" /><IconButton label="Fit board" icon={Maximize} onClick={() => { const bounds = editor.getCurrentPageBounds(); if (bounds) editor.zoomToBounds(bounds, { inset: 100, animation: { duration: 250 } }); }} /></div>
  </div>;
}

function ObjectDetails({ shape, close }: { shape: ResearchShape; close: () => void }) {
  const editor = useEditor();
  const [title, setTitle] = useState(shape.props.title);
  const [body, setBody] = useState(shape.props.body);
  return <aside className="rs-detail rs-surface" aria-label="Object detail">
    <div className="rs-panel-heading"><div><h2>{KIND_DETAILS[shape.props.kind].label}</h2><p>{shape.props.source}</p></div><IconButton label="Close detail" icon={X} onClick={close} /></div>
    <div className="rs-detail-preview"><ResearchObjectView props={{ ...shape.props, title, body }} /></div>
    <form onSubmit={(event) => { event.preventDefault(); editor.markHistoryStoppingPoint("edit research object"); editor.updateShape<ResearchShape>({ id: shape.id, type: shape.type, props: { title, body } }); close(); }}>
      <label>Title<input autoFocus value={title} onChange={(event) => setTitle(event.target.value)} /></label>
      <label>Notes<textarea rows={6} value={body} onChange={(event) => setBody(event.target.value)} placeholder="What do you want to remember?" /></label>
      <button type="submit" className="rs-save">Save to board <Check size={15} /></button>
    </form>
  </aside>;
}

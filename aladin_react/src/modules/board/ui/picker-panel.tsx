import { useEffect, useRef, useState } from "react";
import { ArrowUpRight, Clipboard, Plus, Search, X } from "lucide-react";
import type { InsertRow } from "./insert-popover";
import { BoardButton } from "./board-button";

export type PickerNote =
  | { kind: "none" }
  | { kind: "info"; text: string }
  | { kind: "error"; text: string; onRetry: () => void };

/** Approved library layout, using real workspace lookup and URL unfurling. */
export function PickerPanel({ query, onQueryChange, rows, note, onPaste, onClose, onAddLink, onAddTask, onAddCard }: {
  query: string;
  onQueryChange: (value: string) => void;
  rows: InsertRow[];
  note: PickerNote;
  onPaste: () => void;
  onClose: () => void;
  onAddLink: (url: string) => void;
  onAddTask: () => void;
  onAddCard: () => void;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [url, setUrl] = useState("");
  const [error, setError] = useState("");
  useEffect(() => { if (window.matchMedia?.("(pointer: fine)").matches) inputRef.current?.focus(); }, []);
  return <aside className="rs-library rs-surface board-library" aria-label="Board library" onKeyDown={(event) => { if (event.key === "Escape") onClose(); event.stopPropagation(); }}>
    <div className="rs-panel-heading"><div><h2>Add to your thinking</h2><p>Bring the useful pieces together.</p></div><BoardButton label="Close library" icon={X} onClick={onClose} /></div>
    <label className="rs-search"><Search size={16} /><input ref={inputRef} type="search" aria-label="Search your workspace" placeholder="Find a source, note, instrument…" value={query} onChange={(event) => onQueryChange(event.target.value)} onKeyDown={(event) => { if (event.key === "Enter") rows[0]?.onPick(); }} /></label>
    <div className="rs-section-label">FROM YOUR WORKSPACE<span>This folder · then everywhere</span></div>
    <div className="rs-library-list">{rows.map((row) => <button type="button" key={row.key} className="rs-library-item" onClick={row.onPick}><span className="rs-library-item-icon">{row.icon}</span><span><strong>{row.title}</strong><small>{row.meta}</small></span><Plus size={15} /></button>)}</div>
    {note.kind !== "none" && <div className="rs-empty" role={note.kind === "error" ? "alert" : "status"}>{note.text}{note.kind === "error" && <button className="rs-library-button" type="button" onClick={note.onRetry}>Retry</button>}</div>}
    <form className="rs-link-form" onSubmit={(event) => {
      event.preventDefault();
      try { const parsed = new URL(url.trim()); if (!["http:", "https:"].includes(parsed.protocol)) throw new Error(); onAddLink(parsed.href); }
      catch { setError("Use a complete http or https link."); }
    }}><label htmlFor="board-library-url">Or drop in a link</label><div><input id="board-library-url" placeholder="https://…" value={url} onChange={(event) => { setUrl(event.target.value); setError(""); }} /><button type="submit" aria-label="Add link"><ArrowUpRight size={18} /></button></div>{error && <p role="alert">{error}</p>}</form>
    <button type="button" className="board-library-paste" onClick={onPaste}><Clipboard size={15} /> Paste from clipboard</button>
    <div className="board-library-extra"><button type="button" onClick={onAddTask}>Add task</button><button type="button" onClick={onAddCard}>Add two-sided card</button></div>
  </aside>;
}

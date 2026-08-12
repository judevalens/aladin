import { useMemo, useRef, useState } from "react";
import { FileWarning, Loader2, Minus, Plus, ScanLine } from "lucide-react";

import { cn } from "@/lib/utils";
import { ScrollArea } from "@/components/ui/scroll-area";
import { useArtifactResource } from "@/modules/artifacts/hooks/use-artifact-resource";
import { useDocument, useDocumentOutline } from "@/modules/documents/hooks/use-document";
import type { OutlineEntry } from "@/modules/documents/hooks/use-document";
import { PdfView } from "@/modules/documents/ui/pdf-view";
import type { DocumentPage, DocumentStatus, IngestedDocument } from "@/repos/documents/document-repo";
import type { Artifact } from "@/shared/api/models";

/**
 * The document viewer (design/INGESTION_PRD.md §6): the PAGE, with the ingested structure
 * as navigation over it.
 *
 * **It shows the actual PDF, not the extracted text.** That distinction is the whole
 * design: §13f measured that 60% of a real paper's content lives in tables, figures and
 * equations, and flattening those to reading order destroys them — the p211 sweep table
 * becomes ~150 orphaned floats with every row label gone. The extracted layer exists for
 * machines (search, retrieval, citation). A human should be handed the page, which renders
 * all of it correctly and for free.
 *
 * Text remains available as a mode, because copying clean prose is a real thing to want.
 * It is not the default, and its cost is only paid when asked for — `withText` stays false
 * until you switch, so opening a book no longer pulls a book.
 */
export function DocumentViewerUI({ artifact }: { artifact: Artifact }) {
  const [wantsText, setWantsText] = useState(false);
  const { document, loading, error } = useDocument(artifact.id, wantsText);
  const { url, loading: resourceLoading } = useArtifactResource(artifact);

  // Authored structure beats inferred structure (§5), so the recovered tree is a FALLBACK.
  // It is also the common case: the MIT thesis carries zero bookmarks across 280 pages.
  const authored = document?.sections ?? [];
  const recovered = useDocumentOutline(artifact.id, document?.status === "ready" && authored.length === 0);
  const outline: OutlineEntry[] =
    authored.length > 0
      ? authored.map((section) => ({ title: section.title, depth: section.level, page: section.page }))
      : recovered;

  if (loading && !document) {
    return <Notice icon={Loader2} title="Reading…" body="Extracting text from this document." spin />;
  }
  if (error) {
    return <Notice icon={FileWarning} title="Couldn't load the document" body={error} tone="against" />;
  }
  if (!document) return null;
  if (document.status !== "ready") return <StatusNotice document={document} />;

  return (
    <DocumentReader
      title={artifact.title}
      pageCount={document.pageCount}
      outline={outline}
      outlineRecovered={authored.length === 0 && outline.length > 0}
      url={url}
      resourceLoading={resourceLoading}
      pages={document.pages}
      onWantsText={setWantsText}
    />
  );
}

export interface DocumentReaderProps {
  title: string;
  pageCount: number;
  outline: OutlineEntry[];
  /** Recovered by segmentation rather than carried by the file — worth saying out loud. */
  outlineRecovered: boolean;
  url: string | null;
  resourceLoading: boolean;
  pages?: DocumentPage[];
  /** Text is fetched lazily, so the reader tells its owner when it's actually wanted. */
  onWantsText?: (wanted: boolean) => void;
}

/**
 * The reader chrome. Presentational — no fetching, no app context — so it can be mounted in
 * `src/harness.tsx` and looked at, which is the only way to judge whether it reads as
 * finished. Everything it needs arrives as props.
 */
export function DocumentReader({
  title,
  pageCount,
  outline,
  outlineRecovered,
  url,
  resourceLoading,
  pages,
  onWantsText,
}: DocumentReaderProps) {
  const [mode, setMode] = useState<"page" | "text">("page");
  const [zoom, setZoom] = useState(1);
  const [targetPage, setTargetPage] = useState<number | null>(null);
  const [currentPage, setCurrentPage] = useState(1);
  const pagesRef = useRef<HTMLDivElement | null>(null);

  const switchTo = (next: "page" | "text") => {
    setMode(next);
    onWantsText?.(next === "text");
  };

  // The outline entry you're inside: the last one that started at or before this page.
  // Without it the sidebar is a list of links; with it, it's a position.
  const activeIndex = useMemo(() => {
    let found = -1;
    outline.forEach((entry, index) => {
      if (entry.page <= currentPage) found = index;
    });
    return found;
  }, [outline, currentPage]);

  const jumpTo = (page: number) => {
    if (mode === "text") {
      pagesRef.current?.querySelector(`[data-page="${page}"]`)?.scrollIntoView({ behavior: "smooth", block: "start" });
      setCurrentPage(page);
      return;
    }
    setTargetPage(page);
    setCurrentPage(page);
  };

  return (
    // One separator, not two. An outer `gap` *plus* the nav's border gave a strip of a
    // third background between two surfaces and made the left gutter wider than the right;
    // the breathing room belongs to the stage, symmetrically, on the stage's own colour.
    <div className="flex h-full min-h-0">
      {outline.length > 0 ? (
        <nav className="flex w-64 shrink-0 flex-col border-r border-line bg-panel">
          <div className="flex items-center gap-2 px-4 py-2.5">
            <h2 className="font-mono text-[10px] uppercase tracking-[0.7px] text-ink-4">Contents</h2>
            <span className="font-mono text-[10px] tabular-nums text-ink-4">{outline.length}</span>
            {outlineRecovered ? (
              <span
                title="This file carries no outline of its own — segmentation recovered one."
                className="ml-auto rounded-chip bg-[rgb(var(--amber-soft))] px-1.5 py-px font-mono text-[9px] uppercase tracking-[0.5px] text-amber"
              >
                Recovered
              </span>
            ) : null}
          </div>
          <ScrollArea className="min-h-0 flex-1">
            <ul className="px-2 pb-3">
              {outline.map((entry, index) => {
                const active = index === activeIndex;
                return (
                  <li key={`${entry.page}-${index}`}>
                    <button
                      type="button"
                      onClick={() => jumpTo(entry.page)}
                      style={{ paddingLeft: 8 + Math.max(0, entry.depth) * 11 }}
                      className={cn(
                        "group flex w-full items-baseline gap-2 rounded-md py-[5px] pr-2 text-left text-[12.5px] leading-snug transition-colors",
                        active
                          ? "bg-[rgb(var(--amber-soft))] text-amber"
                          : "text-ink-3 hover:bg-[rgb(var(--hover))] hover:text-ink-2",
                      )}
                    >
                      <span className="min-w-0 flex-1 truncate">{entry.title}</span>
                      <span
                        className={cn(
                          "shrink-0 font-mono text-[10px] tabular-nums",
                          active ? "text-amber" : "text-ink-4",
                        )}
                      >
                        {entry.page}
                      </span>
                    </button>
                  </li>
                );
              })}
            </ul>
          </ScrollArea>
        </nav>
      ) : null}

      <div className="flex min-w-0 flex-1 flex-col bg-rail">
        <header className="flex h-10 shrink-0 items-center gap-3 border-b border-line bg-panel pl-4 pr-3">
          <h1 className="min-w-0 flex-1 truncate font-display text-[13px] text-ink" title={title}>
            {title}
          </h1>

          <span className="shrink-0 whitespace-nowrap font-mono text-[10.5px] tabular-nums text-ink-4">
            {mode === "page" ? `${currentPage} / ${pageCount}` : `${pageCount} pp`}
          </span>

          {mode === "page" ? (
            <div className="flex shrink-0 items-center gap-0.5">
              <IconButton label="Zoom out" onClick={() => setZoom((z) => Math.max(0.5, +(z - 0.15).toFixed(2)))}>
                <Minus className="size-3" strokeWidth={2} />
              </IconButton>
              <span className="w-9 text-center font-mono text-[10.5px] tabular-nums text-ink-4">
                {Math.round(zoom * 100)}%
              </span>
              <IconButton label="Zoom in" onClick={() => setZoom((z) => Math.min(2.5, +(z + 0.15).toFixed(2)))}>
                <Plus className="size-3" strokeWidth={2} />
              </IconButton>
            </div>
          ) : null}

          <div className="flex shrink-0 items-center gap-px rounded-chip bg-field p-[3px]">
            <ModeButton active={mode === "page"} onClick={() => switchTo("page")}>
              Page
            </ModeButton>
            <ModeButton active={mode === "text"} onClick={() => switchTo("text")}>
              Text
            </ModeButton>
          </div>
        </header>

        {mode === "page" ? (
          url ? (
            <PdfView
              url={url}
              targetPage={targetPage}
              onVisiblePageChange={setCurrentPage}
              zoom={zoom}
              className="min-h-0 flex-1"
            />
          ) : resourceLoading ? (
            <Notice icon={Loader2} title="Loading…" body="Fetching the file." spin />
          ) : (
            <Notice
              icon={FileWarning}
              title="Couldn't load the file"
              body="The document was read, but its bytes couldn't be fetched. Text mode still works."
              tone="against"
            />
          )
        ) : (
          <ScrollArea className="min-h-0 flex-1">
            <div ref={pagesRef} className="mx-auto w-full max-w-[52rem] px-8 py-7">
              {/* §13f: this mode is honest about what it is. Tables and equations do not
                  survive the flattening, so it is for copying prose, not for reading a
                  paper — Page mode is for that. */}
              <p className="mb-6 border-l-2 border-line-2 pl-3 text-[12px] leading-relaxed text-ink-4">
                Extracted text — what the machine layer reads. Tables, figures and equations don&apos;t
                survive this view; switch to Page for those.
              </p>
              {(pages ?? []).map((page) => (
                <section key={page.page} data-page={page.page} className="scroll-mt-6 pb-7">
                  <div className="mb-2 select-none font-mono text-[10px] uppercase tracking-wide text-ink-4">
                    p{page.page}
                  </div>
                  {page.text ? (
                    <p className="whitespace-pre-wrap text-[13.5px] leading-relaxed text-ink-2">{page.text}</p>
                  ) : (
                    <p className="text-[13px] italic text-ink-4">No text on this page.</p>
                  )}
                </section>
              ))}
            </div>
          </ScrollArea>
        )}
      </div>
    </div>
  );
}

function ModeButton({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "rounded-chip px-2 py-[3px] font-mono text-[10px] uppercase tracking-[0.5px] transition-colors",
        active ? "bg-raise text-ink" : "text-ink-4 hover:text-ink-2",
      )}
    >
      {children}
    </button>
  );
}

function IconButton({
  label,
  onClick,
  children,
}: {
  label: string;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      title={label}
      className="flex size-6 items-center justify-center rounded-md text-ink-4 transition-colors hover:bg-[rgb(var(--hover))] hover:text-ink-2"
    >
      {children}
    </button>
  );
}

/**
 * §4: every non-ready state says what happened and what to do about it. A scanned book
 * is the case that matters — it isn't broken, it just needs OCR first, and saying so is
 * the difference between a next action and a dead end.
 */
function StatusNotice({ document }: { document: IngestedDocument }) {
  const copy: Record<Exclude<DocumentStatus, "ready">, { title: string; body: string }> = {
    pending: { title: "Queued", body: "This document is waiting to be read." },
    ingesting: { title: "Reading…", body: "Extracting text and structure." },
    unsupported: {
      title: "No text to extract",
      body:
        document.error ||
        "This PDF has no text layer — it's almost certainly a scan. Run it through OCR and re-upload.",
    },
    failed: { title: "Couldn't read this file", body: document.error || "Extraction failed." },
  };
  const { title, body } = copy[document.status as Exclude<DocumentStatus, "ready">];
  const working = document.status === "pending" || document.status === "ingesting";

  return (
    <Notice
      icon={working ? Loader2 : document.status === "unsupported" ? ScanLine : FileWarning}
      title={title}
      body={body}
      spin={working}
      tone={document.status === "failed" ? "against" : "muted"}
    />
  );
}

function Notice({
  icon: Icon,
  title,
  body,
  spin = false,
  tone = "muted",
}: {
  icon: React.ComponentType<{ className?: string; strokeWidth?: number }>;
  title: string;
  body: string;
  spin?: boolean;
  tone?: "muted" | "against";
}) {
  return (
    <div className="flex h-full items-center justify-center p-8">
      <div className="max-w-md text-center">
        <Icon
          className={cn("mx-auto mb-3 size-6", tone === "against" ? "text-against" : "text-ink-4", spin && "animate-spin")}
          strokeWidth={1.5}
        />
        <p className="font-display text-[15px] text-ink">{title}</p>
        <p className="mt-1.5 text-[13px] leading-relaxed text-ink-4">{body}</p>
      </div>
    </div>
  );
}

/**
 * Chooses how a file artifact opens.
 *
 * A PDF we've read renders as a document; anything else keeps the existing file card
 * (download, preview, metadata). The status-only fetch here is cheap by design — it
 * never pulls the text, which is why `withText` exists.
 */
export function FileArtifactPaneUI({
  artifact,
  fallback,
}: {
  artifact: Artifact;
  fallback: React.ReactNode;
}) {
  const { document, loading } = useDocument(artifact.id, false);

  if (document && document.status === "ready") {
    return <DocumentViewerUI artifact={artifact} />;
  }

  // A PDF with NO document row at all is the state that used to lie: it rendered
  // identically to a file nothing will ever read. Extraction happens in a background
  // worker, so "nothing here yet" and "nothing is coming" look the same from the client
  // — say which, because the fix (start the worker) is otherwise invisible.
  const unread = !loading && !document && looksLikePDF(artifact);

  return (
    <div className="flex h-full min-h-0 flex-col">
      {unread ? (
        <p className="border-b border-line bg-panel px-6 py-2 text-[12.5px] text-ink-4">
          Not read yet — no text or outline has been extracted. Extraction runs in the
          background worker; if this doesn&apos;t change, the worker isn&apos;t running.
        </p>
      ) : null}
      {document && document.status !== "ready" ? (
        <div className="border-b border-line bg-panel">
          <StatusLine document={document} />
        </div>
      ) : null}
      <div className="min-h-0 flex-1">{fallback}</div>
    </div>
  );
}

/** Compact form of the status notice, for sitting above the file card. */
function StatusLine({ document }: { document: IngestedDocument }) {
  const working = document.status === "pending" || document.status === "ingesting";
  return (
    <p
      className={cn(
        "px-6 py-2 text-[12.5px]",
        document.status === "failed" ? "text-against" : working ? "text-ink-4" : "text-amber",
      )}
    >
      {working
        ? "Reading this document…"
        : document.error || `Ingestion status: ${document.status}`}
    </p>
  );
}

// Uploads keep their original filename as the title, and the stored resource keeps the
// extension, so either is a good enough signal for a status hint.
function looksLikePDF(artifact: Artifact): boolean {
  return /\.pdf(\?|$)/i.test(artifact.resourceUrl ?? "") || /\.pdf$/i.test(artifact.title);
}

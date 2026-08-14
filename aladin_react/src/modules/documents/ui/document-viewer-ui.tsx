import { useMemo, useState } from "react";
import { Eyebrow } from "@/components/ui/eyebrow";
import { Icon } from "@/components/ui/icon";
import { FileWarning, Loader2, Minus, PanelLeft, Plus, ScanLine } from "lucide-react";

import { cn } from "@/lib/utils";
import { ScrollArea } from "@/components/ui/scroll-area";
import { useArtifactResource } from "@/modules/artifacts/hooks/use-artifact-resource";
import { useDocument, useDocumentOutline } from "@/modules/documents/hooks/use-document";
import type { OutlineEntry } from "@/modules/documents/hooks/use-document";
import { PdfView } from "@/modules/documents/ui/pdf-view";
import type { DocumentStatus, IngestedDocument } from "@/repos/documents/document-repo";
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
 * There was a Page/Text segmented toggle in the header. It was removed: it read as chrome
 * bolted onto the title bar, and the extracted-text view it revealed is a machine artifact —
 * tables and equations do not survive the flattening. `useDocument` keeps its `withText`
 * parameter (other callers pass false too), so bringing a text view back is a UI change only.
 */
export function DocumentViewerUI({ artifact }: { artifact: Artifact }) {
  const { document, loading, error } = useDocument(artifact.id, false);
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
}: DocumentReaderProps) {
  const [outlineOpen, setOutlineOpen] = useState(true);
  const [zoom, setZoom] = useState(1);
  const [targetPage, setTargetPage] = useState<number | null>(null);
  const [currentPage, setCurrentPage] = useState(1);

  // The outline entry you're inside: the last one that started at or before this page.
  // Without it the sidebar is a list of links; with it, it's a position.
  const activeIndex = useMemo(() => {
    let found = -1;
    outline.forEach((entry, index) => {
      if (entry.page <= currentPage) found = index;
    });
    return found;
  }, [outline, currentPage]);

  const hasOutline = outline.length > 0;

  const jumpTo = (page: number) => {
    setTargetPage(page);
    setCurrentPage(page);
  };

  return (
    // One separator, not two. An outer `gap` *plus* the nav's border gave a strip of a
    // third background between two surfaces and made the left gutter wider than the right;
    // the breathing room belongs to the stage, symmetrically, on the stage's own colour.
    <div className="flex h-full min-h-0">
      {hasOutline && outlineOpen ? (
        // min-h-0 is what makes the list scroll rather than push the nav taller than the
        // reader: without it this flex child floors at its content height.
        <nav className="flex h-full min-h-0 w-64 shrink-0 flex-col border-r border-line bg-panel">
          <div className="flex shrink-0 items-center gap-2 px-4 py-2.5">
            <Eyebrow as="h2" className="text-ink-4">Contents</Eyebrow>
            <span className="font-mono text-meta tabular-nums text-ink-4">{outline.length}</span>
            {outlineRecovered ? (
              <span
                title="This file carries no outline of its own — segmentation recovered one."
                className="ml-auto rounded-chip bg-[rgb(var(--amber-soft))] px-1.5 py-px font-mono text-meta uppercase tracking-[0.5px] text-amber"
              >
                Recovered
              </span>
            ) : null}
          </div>
          {/* `type="auto"` because Radix defaults to "hover": with the default there is no
              scrollbar element in the DOM at all until the pointer is already inside, so a
              46-entry outline looked like it simply didn't scroll.

              The `document-outline-scroll` CSS rule is the other half. Radix gives its viewport a child with
              an inline `display: table`, which sizes to CONTENT — 490px inside a 255px pane
              here. `truncate` never engages against a table box, so long titles ran under the
              border and took the page numbers off screen with them. */}
          <ScrollArea
            type="auto"
            className="document-outline-scroll min-h-0 flex-1"
          >
            {/* A 1.5px gap so consecutive highlighted rows read as separate targets — with
                the rows flush, the active row's amber block merged into its neighbour's
                hover block and the two looked like one taller row. */}
            <ul className="space-y-0.5 px-2 pb-3">
              {outline.map((entry, index) => {
                const active = index === activeIndex;
                return (
                  <li key={`${entry.page}-${index}`}>
                    <button
                      type="button"
                      onClick={() => jumpTo(entry.page)}
                      style={{ paddingLeft: 8 + Math.max(0, entry.depth) * 11 }}
                      className={cn(
                        "group flex w-full items-baseline gap-2 rounded-control py-1 pr-2 text-left text-small leading-snug transition-colors",
                        active
                          ? "bg-[rgb(var(--amber-soft))] text-amber"
                          : "text-ink-3 hover:bg-[rgb(var(--hover))] hover:text-ink-2",
                      )}
                    >
                      <span className="min-w-0 flex-1 truncate">{entry.title}</span>
                      <span
                        className={cn(
                          "shrink-0 font-mono text-meta tabular-nums",
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
        <header className="flex h-10 shrink-0 items-center gap-3 border-b border-line bg-panel pl-2 pr-3">
          {/* One control, always in the same place, rather than a chevron inside the panel
              plus a second button to bring it back once the panel is gone. */}
          {hasOutline ? (
            <IconButton
              label={outlineOpen ? "Hide contents" : `Show contents (${outline.length})`}
              onClick={() => setOutlineOpen((open) => !open)}
              active={outlineOpen}
            >
              <Icon as={PanelLeft} size="inline" />
            </IconButton>
          ) : null}
          <h1 className="min-w-0 flex-1 truncate font-display text-body text-ink" title={title}>
            {title}
          </h1>

          <span className="shrink-0 whitespace-nowrap font-mono text-meta tabular-nums text-ink-4">
            {currentPage} / {pageCount}
          </span>

          <div className="flex shrink-0 items-center gap-0.5">
            <IconButton label="Zoom out" onClick={() => setZoom((z) => Math.max(0.5, +(z - 0.15).toFixed(2)))}>
              <Icon as={Minus} size="inline" mark />
            </IconButton>
            <span className="w-9 text-center font-mono text-meta tabular-nums text-ink-4">
              {Math.round(zoom * 100)}%
            </span>
            <IconButton label="Zoom in" onClick={() => setZoom((z) => Math.min(2.5, +(z + 0.15).toFixed(2)))}>
              <Icon as={Plus} size="inline" mark />
            </IconButton>
          </div>
        </header>

        {url ? (
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
            body="The document was read, but its bytes couldn't be fetched."
            tone="against"
          />
        )}
      </div>
    </div>
  );
}

function IconButton({
  label,
  onClick,
  children,
  /** A toggle rather than an action — reads as pressed, and announces as one. */
  active,
}: {
  label: string;
  onClick: () => void;
  children: React.ReactNode;
  active?: boolean;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      aria-pressed={active}
      title={label}
      className={cn(
        "flex size-6 items-center justify-center rounded-control transition-colors hover:bg-[rgb(var(--hover))] hover:text-ink-2",
        active ? "text-ink-2" : "text-ink-4",
      )}
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
  // `Glyph`, not `Icon` — this file imports the <Icon> primitive, and a prop named `Icon`
  // would shadow it inside this component only, which is the worst kind of trap.
  icon: Glyph,
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
        {/* Empty-state illustration at 24px — deliberately off the <Icon> scale (§5 rule 9's
            carve-out), so its raw strokeWidth is intentional. */}
        <Glyph
          className={cn("mx-auto mb-3 size-6", tone === "against" ? "text-against" : "text-ink-4", spin && "animate-spin")}
          strokeWidth={1.5}
        />
        <p className="font-display text-lead text-ink">{title}</p>
        <p className="mt-1.5 text-body leading-relaxed text-ink-4">{body}</p>
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
        <p className="border-b border-line bg-panel px-6 py-2 text-small text-ink-4">
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
        "px-6 py-2 text-small",
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

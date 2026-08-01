import { useRef } from "react";
import { FileWarning, Loader2, ScanLine } from "lucide-react";

import { cn } from "@/lib/utils";
import { ScrollArea } from "@/components/ui/scroll-area";
import { useDocument } from "@/modules/documents/hooks/use-document";
import type { DocumentStatus, IngestedDocument } from "@/repos/documents/document-repo";

/**
 * The document viewer (design/INGESTION_PRD.md §6): outline sidebar, text pane,
 * jump-to-section.
 *
 * It renders for a file artifact that has been ingested. Everything it shows is a fact
 * recovered from the file — text and structure. There is no summary, no extracted
 * topics, no "key points": §2 says ingestion extracts and does not interpret, and a
 * surface that quietly adds interpretation would drag it back.
 */
export function DocumentViewerUI({ artifactId }: { artifactId: string }) {
  const { document, loading, error } = useDocument(artifactId, true);
  const pagesRef = useRef<HTMLDivElement | null>(null);

  if (loading && !document) {
    return <Notice icon={Loader2} title="Reading…" body="Extracting text from this document." spin />;
  }
  if (error) {
    return <Notice icon={FileWarning} title="Couldn't load the document" body={error} tone="against" />;
  }
  if (!document) return null;

  if (document.status !== "ready") {
    return <StatusNotice document={document} />;
  }

  const jumpTo = (page: number) => {
    pagesRef.current
      ?.querySelector(`[data-page="${page}"]`)
      ?.scrollIntoView({ behavior: "smooth", block: "start" });
  };

  return (
    <div className="flex h-full min-h-0">
      {/* Outline — only when the document actually carries one. §5: we read the
          document's own bookmarks and never invent a structure, so no outline means
          no sidebar rather than a fabricated one. */}
      {document.sections.length > 0 ? (
        <nav className="w-64 shrink-0 border-r border-line bg-panel">
          <ScrollArea className="h-full">
            <div className="px-3 py-3">
              <h2 className="mb-2 px-1 font-mono text-[10.5px] uppercase tracking-[0.6px] text-ink-4">
                Contents · {document.sections.length}
              </h2>
              <ul>
                {document.sections.map((section, index) => (
                  <li key={`${section.page}-${index}`}>
                    <button
                      type="button"
                      onClick={() => jumpTo(section.page)}
                      style={{ paddingLeft: 6 + section.level * 12 }}
                      className="flex w-full items-baseline gap-2 rounded-md py-1 pr-2 text-left text-[12.5px] text-ink-3 transition-colors hover:bg-[rgb(var(--hover))] hover:text-ink"
                    >
                      <span className="min-w-0 flex-1 truncate">{section.title}</span>
                      <span className="shrink-0 font-mono text-[10px] text-ink-4">{section.page}</span>
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          </ScrollArea>
        </nav>
      ) : null}

      <ScrollArea className="min-w-0 flex-1">
        <div ref={pagesRef} className="mx-auto w-full max-w-[52rem] px-8 py-7">
          <header className="mb-5 font-mono text-[10.5px] uppercase tracking-[0.5px] text-ink-4">
            {document.pageCount} page{document.pageCount === 1 ? "" : "s"}
            {document.sections.length === 0 ? " · no outline in this file" : ""}
          </header>
          {(document.pages ?? []).map((page) => (
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
    </div>
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
  artifactId,
  fallback,
}: {
  artifactId: string;
  fallback: React.ReactNode;
}) {
  const { document, loading } = useDocument(artifactId, false);

  // While we don't know yet, show the file card rather than a flash of empty chrome —
  // it's the correct view for most files and the honest one for an unread PDF.
  if (loading && !document) return <>{fallback}</>;
  if (!document) return <>{fallback}</>;

  return <DocumentViewerUI artifactId={artifactId} />;
}

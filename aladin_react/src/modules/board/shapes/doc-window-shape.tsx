import { useEffect, useSyncExternalStore } from "react";
import { BaseBoxShapeUtil, HTMLContainer } from "tldraw";
import type { TLIndicatorPath } from "tldraw";

import { useBoardContent, type DocPageContent } from "../domain/board-content";
import { DOC_WINDOW_DEFAULTS, docWindowProps, type DocWindowShape } from "./shape-types";
import { roundedIndicator, tappable } from "./shape-shared";

const KIND_ICONS: Record<string, string> = {
  file: "M14 3v5h5M19 8v11a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h7z",
  note: "M4 4h16v16H4zM8 9h8M8 13h8M8 17h5",
  link: "M10 14a5 5 0 0 0 7 0l3-3a5 5 0 0 0-7-7l-1.5 1.5M14 10a5 5 0 0 0-7 0l-3 3a5 5 0 0 0 7 7l1.5-1.5",
};

/**
 * A live window onto a workspace artifact. The page is THIS window's own — two windows
 * on one PDF are two reading positions, one document (product rule 1). The body resolves
 * read-live through BoardContentContext; the shape stores no content.
 */
export class DocWindowShapeUtil extends BaseBoxShapeUtil<DocWindowShape> {
  static override type = "aladin-doc" as const;
  static override props = docWindowProps;

  override getDefaultProps(): DocWindowShape["props"] {
    return { ...DOC_WINDOW_DEFAULTS };
  }

  override getIndicatorPath(shape: DocWindowShape): TLIndicatorPath {
    return roundedIndicator(shape.props.w, shape.props.h);
  }

  override component(shape: DocWindowShape) {
    const source = useBoardContent();
    const { artifactId, page } = shape.props;
    const content = useSyncExternalStore<DocPageContent | null>(
      (onChange) => (source ? source.subscribe(artifactId, page, onChange) : () => {}),
      () => (source ? source.get(artifactId, page) : null),
    );
    const pageCount =
      content?.state === "ready" ? content.pageCount : shape.props.pageCount;

    // Persist the resolved page count into the window's props (once per change), so the
    // selection bar and a cold reload agree with the live footer.
    useEffect(() => {
      if (content?.state === "ready" && content.pageCount !== shape.props.pageCount) {
        this.editor.updateShape({
          id: shape.id,
          type: shape.type,
          props: { pageCount: content.pageCount },
        });
      }
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [content?.state === "ready" ? content.pageCount : null, shape.props.pageCount]);

    const setPage = (next: number) => {
      const clamped = Math.max(1, Math.min(pageCount, next));
      if (clamped === page) return;
      this.editor.updateShape({
        id: shape.id,
        type: shape.type,
        props: { page: clamped, pageCount },
      });
    };

    const mono: React.CSSProperties = {
      fontFamily: "'JetBrains Mono', ui-monospace, monospace",
      fontSize: 10,
      color: "var(--ink-4)",
    };

    return (
      <HTMLContainer>
        <div
          style={{
            display: "flex",
            flexDirection: "column",
            width: "100%",
            height: "100%",
            borderRadius: 18,
            background: "var(--card)",
            border: "1px solid rgb(var(--line))",
            overflow: "hidden",
            fontFamily: "'Space Grotesk Variable', 'Space Grotesk', system-ui, sans-serif",
          }}
        >
          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: 10,
              padding: "13px 16px",
              borderBottom: "1px solid rgb(var(--line-2))",
            }}
          >
            <svg
              width="17"
              height="17"
              viewBox="0 0 24 24"
              fill="none"
              stroke="var(--ink-3)"
              strokeWidth="1.75"
              strokeLinecap="round"
              strokeLinejoin="round"
              style={{ flexShrink: 0 }}
            >
              <path d={KIND_ICONS[shape.props.artifactKind] ?? KIND_ICONS.file} />
            </svg>
            <span
              style={{
                minWidth: 0,
                fontSize: 14.5,
                color: "var(--ink-2)",
                overflow: "hidden",
                textOverflow: "ellipsis",
                whiteSpace: "nowrap",
              }}
            >
              {shape.props.title}
            </span>
            <span
              style={{
                flexShrink: 0,
                display: "flex",
                alignItems: "center",
                gap: 6,
                marginLeft: "auto",
                padding: "3px 9px",
                borderRadius: 8,
                background: "rgb(var(--amber-soft))",
                fontFamily: "'JetBrains Mono', ui-monospace, monospace",
                fontSize: 10,
                color: "var(--amber)",
              }}
            >
              <span
                style={{
                  width: 6,
                  height: 6,
                  borderRadius: 999,
                  background: "var(--amber)",
                }}
              />
              live
            </span>
          </div>
          <div style={{ flex: 1, minHeight: 0, padding: "14px 16px", overflow: "hidden" }}>
            {content?.state === "ready" ? (
              <>
                <div
                  style={{
                    ...mono,
                    letterSpacing: "0.08em",
                    textTransform: "uppercase",
                  }}
                >
                  {content.sourceLine}
                </div>
                <p
                  style={{
                    margin: "9px 0 0",
                    fontFamily: "Georgia, 'Times New Roman', serif",
                    fontSize: 15,
                    lineHeight: 1.65,
                    color: "var(--ink-2)",
                  }}
                >
                  {content.excerpt}
                </p>
              </>
            ) : (
              <div style={{ ...mono, fontSize: 11, color: "var(--ink-4)" }}>
                {content?.state === "missing"
                  ? "the artifact is gone — the window stays"
                  : "live window — resolving…"}
              </div>
            )}
          </div>
          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: 12,
              padding: "0 16px 13px",
              ...mono,
            }}
          >
            <span {...tappable(this.editor, () => setPage(page - 1))}>◀</span>
            <span>
              page {page} / {pageCount}
            </span>
            <span {...tappable(this.editor, () => setPage(page + 1))}>▶</span>
            <span style={{ marginLeft: "auto" }}>this window's own page</span>
          </div>
        </div>
      </HTMLContainer>
    );
  }
}

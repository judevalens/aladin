import { renderPlaintextFromRichText } from "tldraw";
import type { Editor, TLShape } from "tldraw";

import type { CardProps, DocWindowProps, ExcerptProps, LinkProps, TaskProps } from "../shapes/shape-types";

/** What the selection bar shows for one selected object. */
export interface SelectionSummary {
  title: string;
  meta: string;
  /** The green `cited` chip — the object carries a source location. */
  cited: boolean;
  /** Artifact behind the object, when there is one. */
  artifactId: string | null;
  /** Where in the artifact — the wormhole's page. Null when the object has no anchor. */
  page: number | null;
  /** Label for the open action — the handoff: doc/voice open in folder, others open source. */
  openLabel: "Open in folder" | "Open source" | null;
}

export function describeShape(editor: Editor, shape: TLShape): SelectionSummary {
  switch (shape.type) {
    case "aladin-doc": {
      const props = shape.props as DocWindowProps;
      return {
        title: props.title || "Live window",
        meta: `page ${props.page} / ${props.pageCount} · live window, two-way`,
        cited: false,
        artifactId: props.artifactId || null,
        page: props.page || null,
        openLabel: props.artifactId ? "Open in folder" : null,
      };
    }
    case "aladin-excerpt": {
      const props = shape.props as ExcerptProps;
      return {
        title: props.text || "Excerpt",
        meta: props.page != null ? `${props.sourceTitle} · p. ${props.page}` : props.sourceTitle,
        cited: props.page != null,
        artifactId: props.sourceArtifactId,
        page: props.page,
        openLabel: props.sourceArtifactId ? "Open source" : null,
      };
    }
    case "aladin-task": {
      const props = shape.props as TaskProps;
      return {
        title: props.text || "Task",
        meta: props.checked ? "done" : props.meta,
        cited: false,
        artifactId: null,
        page: null,
        openLabel: null,
      };
    }
    case "aladin-card": {
      const props = shape.props as CardProps;
      return {
        title: props.front || "Card",
        meta: props.cite,
        cited: false,
        artifactId: null,
        page: null,
        openLabel: null,
      };
    }
    case "aladin-link": {
      const props = shape.props as LinkProps;
      return {
        title: props.title || props.url || "Link",
        meta: props.status === "pending" ? "fetching preview…" : props.domain || "external link",
        cited: false,
        artifactId: null,
        page: null,
        openLabel: null, // the shape's own ↗ opens the URL; Open verbs are for artifacts
      };
    }
    case "text": {
      const richText = (shape.props as { richText?: Parameters<typeof renderPlaintextFromRichText>[1] })
        .richText;
      const text = richText ? renderPlaintextFromRichText(editor, richText) : "";
      return {
        title: text.trim() || "Ink",
        meta: "ink · your handwriting is the legend",
        cited: false,
        artifactId: null,
        page: null,
        openLabel: null,
      };
    }
    case "note":
      return { title: renderPlaintextFromRichText(editor, shape.props.richText) || "Note", meta: "sticky note", cited: false, artifactId: null, page: null, openLabel: null };
    case "draw":
    case "highlight":
      return { title: "Ink", meta: "pencil stroke", cited: false, artifactId: null, page: null, openLabel: null };
    case "arrow":
      return { title: "Link", meta: "connects two objects", cited: false, artifactId: null, page: null, openLabel: null };
    default:
      return { title: "Object", meta: shape.type, cited: false, artifactId: null, page: null, openLabel: null };
  }
}

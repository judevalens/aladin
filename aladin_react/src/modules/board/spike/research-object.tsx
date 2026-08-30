import { BaseBoxShapeUtil, HTMLContainer, T, type RecordProps, type TLBaseShape } from "tldraw";
import { ArrowUpRight, FileText, GitBranch, Globe, MessageCircle, NotebookPen, Play, SquareDashed, Waves } from "lucide-react";

export type ResearchKind = "paper" | "discussion" | "video" | "document" | "repository" | "instrument" | "note" | "link";
export interface ResearchProps {
  w: number;
  h: number;
  kind: ResearchKind;
  title: string;
  body: string;
  source: string;
  tint: "neutral" | "butter" | "sage" | "lilac";
}
export type ResearchShape = TLBaseShape<"research-spike-object", ResearchProps>;

declare module "@tldraw/tlschema" {
  interface TLGlobalShapePropsMap {
    "research-spike-object": ResearchProps;
  }
}

export const RESEARCH_PROPS: RecordProps<ResearchShape> = {
  w: T.nonZeroNumber, h: T.nonZeroNumber,
  kind: T.literalEnum("paper", "discussion", "video", "document", "repository", "instrument", "note", "link"),
  title: T.string, body: T.string, source: T.string,
  tint: T.literalEnum("neutral", "butter", "sage", "lilac"),
};

export const KIND_DETAILS = {
  paper: { label: "Paper", icon: FileText },
  discussion: { label: "Discussion", icon: MessageCircle },
  video: { label: "Video", icon: Play },
  document: { label: "Note", icon: NotebookPen },
  repository: { label: "Repository", icon: GitBranch },
  instrument: { label: "Aladin instrument", icon: Waves },
  note: { label: "Sticky note", icon: SquareDashed },
  link: { label: "Link", icon: Globe },
} satisfies Record<ResearchKind, { label: string; icon: typeof FileText }>;

export const RESEARCH_DEFAULTS: ResearchProps = {
  w: 300, h: 190, kind: "note", title: "A thought to follow", body: "", source: "Your note", tint: "butter",
};

export class ResearchShapeUtil extends BaseBoxShapeUtil<ResearchShape> {
  static override type = "research-spike-object" as const;
  static override props = RESEARCH_PROPS;
  override getDefaultProps() { return { ...RESEARCH_DEFAULTS }; }
  override canEdit() { return false; }
  override getIndicatorPath(shape: ResearchShape) {
    const path = new Path2D();
    path.roundRect(0, 0, shape.props.w, shape.props.h, 10);
    return path;
  }
  override component(shape: ResearchShape) {
    return <HTMLContainer><ResearchObjectView props={shape.props} /></HTMLContainer>;
  }
}

/** All source text and instrument values in this spike are illustrative fixtures. */
export function ResearchObjectView({ props }: { props: ResearchProps }) {
  const { kind, title, body, source, tint } = props;
  const KindIcon = KIND_DETAILS[kind].icon;
  return (
    <article className={`rs-object rs-object--${kind} rs-tint--${tint}`} aria-label={`${KIND_DETAILS[kind].label}: ${title}`}>
      {kind === "video" && <div className="rs-video-art"><span className="rs-video-lines" /><span className="rs-play"><Play size={20} fill="currentColor" /></span><span className="rs-duration">18:42</span></div>}
      <div className="rs-object-content">
        <div className="rs-object-meta"><span className={`rs-kind-icon rs-kind--${kind}`}><KindIcon size={14} strokeWidth={1.7} /></span><span>{source || KIND_DETAILS[kind].label}</span>{kind !== "note" && <ArrowUpRight size={13} className="rs-object-arrow" />}</div>
        <h2>{title}</h2>
        {kind === "instrument" ? <SpreadPreview /> : <p>{body}</p>}
        {kind === "paper" && <div className="rs-object-footer"><span>PDF</span><span>Foundational reading</span></div>}
        {kind === "repository" && <div className="rs-object-footer"><i className="rs-language-dot" />Python<span className="rs-repo-branch"><GitBranch size={12} /> research</span></div>}
        {kind === "document" && <div className="rs-document-lines" aria-hidden="true"><span /><span /><span /></div>}
        {kind === "note" && <div className="rs-note-corner" aria-hidden="true" />}
      </div>
    </article>
  );
}

function SpreadPreview() {
  return <div className="rs-spread">
    <div className="rs-spread-legend"><span><i />Spread</span><span>Illustrative data</span></div>
    <svg viewBox="0 0 284 108" role="img" aria-label="Illustrative spread fluctuating around its mean">
      <rect x="0" y="25" width="284" height="58" rx="3" fill="currentColor" opacity=".055" />
      <path d="M0 25H284M0 83H284" stroke="currentColor" strokeOpacity=".18" strokeDasharray="3 4" fill="none" />
      <path d="M0 54H284" stroke="currentColor" strokeOpacity=".3" fill="none" />
      <path d="M1 58L10 60L18 48L27 54L36 30L45 18L54 38L62 44L71 40L80 65L89 72L98 60L107 54L116 62L125 86L134 78L143 92L152 70L161 66L170 42L179 48L188 33L197 17L206 23L215 41L224 46L233 37L242 51L251 60L260 52L270 59L283 54" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" />
      <circle cx="283" cy="54" r="3.2" fill="currentColor" />
    </svg>
    <div className="rs-spread-footer"><span>Lookback <b>90 days</b></span><span>z-score <b>+0.12</b></span></div>
  </div>;
}

export const SAMPLE_OBJECTS: Array<{ id: string; x: number; y: number; props: ResearchProps }> = [
  { id: "paper", x: 0, y: 143, props: { w: 300, h: 220, kind: "paper", title: "Pairs trading & relative value", body: "The original idea. Formation windows, pair selection, and what to read next.", source: "Reading list · PDF", tint: "neutral" } },
  { id: "discussion", x: 18, y: 377, props: { w: 282, h: 185, kind: "discussion", title: "What survives transaction costs?", body: "Keep the practical questions next to the theory.", source: "reddit.com / r/algotrading", tint: "neutral" } },
  { id: "video", x: -10, y: 588, props: { w: 306, h: 110, kind: "video", title: "When two prices move together", body: "", source: "YouTube · saved video", tint: "neutral" } },
  { id: "method", x: 405, y: 161, props: { w: 304, h: 235, kind: "document", title: "The method, in my own words", body: "Find a relationship. Watch the spread. Understand why it might return — and why it might not.", source: "My notes / Pairs trading", tint: "neutral" } },
  { id: "question", x: 425, y: 436, props: { w: 268, h: 256, kind: "note", title: "What would make this fail?", body: "Regime changes?\nBorrow costs?\nA relationship that\nwas never real?", source: "A question to keep open", tint: "butter" } },
  { id: "spread", x: 829, y: 160, props: { w: 328, h: 255, kind: "instrument", title: "Explore the spread", body: "A place for a native Aladin instrument. This chart is an illustrative preview, not a live market feed.", source: "Aladin / Spread explorer", tint: "neutral" } },
  { id: "repo", x: 849, y: 482, props: { w: 302, h: 208, kind: "repository", title: "pairs-research / notebook", body: "Formation windows, pair selection, and a first walk-forward experiment.", source: "Code / research notebook", tint: "neutral" } },
];

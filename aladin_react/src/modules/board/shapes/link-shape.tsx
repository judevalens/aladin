import { useState } from "react";
import { BaseBoxShapeUtil, HTMLContainer } from "tldraw";
import type { TLIndicatorPath } from "tldraw";

import { DOCK_PATHS, DockIcon } from "../ui/dock-icons";
import { LINK_DEFAULTS, linkProps, type LinkShape } from "./shape-types";
import { boardObjectClass, roundedIndicator, tappable } from "./shape-shared";
import { boardSourceUrl, useOpenBoardSource } from "../domain/board-source";

/**
 * An external link with its unfurled preview — the board-native replacement for tldraw's
 * stock bookmark (which renders off-register and never unfurls). Self-contained: the
 * props ARE the preview; nothing re-fetches at render time.
 */
export class LinkShapeUtil extends BaseBoxShapeUtil<LinkShape> {
  static override type = "aladin-link" as const;
  static override props = linkProps;

  override getDefaultProps(): LinkShape["props"] {
    return { ...LINK_DEFAULTS };
  }

  override canEdit() {
    return false;
  }

  override getIndicatorPath(shape: LinkShape): TLIndicatorPath {
    return roundedIndicator(shape.props.w, shape.props.h);
  }

  override component(shape: LinkShape) {
    const openSource = useOpenBoardSource();
    const { url, title, description, domain, image, favicon, status } = shape.props;

    return (
      <HTMLContainer>
        <article aria-label={"Link: " + (title || domain || url)} className={boardObjectClass(shape) + " rs-object board-link-object"}>
          {image ? <LinkThumbnail key={image} url={image} /> : null}
          <div className="rs-object-content">
            <div className="rs-object-meta">
              {favicon ? (
                <img
                  src={favicon}
                  alt=""
                  draggable={false}
                  className="h-4 w-4 shrink-0 rounded-[3px]"
                  onError={(e) => {
                    e.currentTarget.style.display = "none";
                  }}
                />
              ) : null}
              <span>
                {domain || "link"}
              </span>
              <button
                type="button"
                disabled={!boardSourceUrl(url)}
                aria-label="Open link"
                {...tappable(this.editor, () => openSource(url))}
                className="board-source-open"
              >
                <DockIcon d={DOCK_PATHS.open} size={15} strokeWidth={2} />
              </button>
            </div>
            <h2 className="line-clamp-2">
              {status === "pending" && !title ? (
                <span className="text-ink-4">fetching preview…</span>
              ) : (
                title || url
              )}
            </h2>
            {description ? (
              <p className="line-clamp-3">
                {description}
              </p>
            ) : null}
          </div>
        </article>
      </HTMLContainer>
    );
  }
}

function LinkThumbnail({ url }: { url: string }) {
  const [failed, setFailed] = useState(false);
  if (failed) return null;
  return <div className="h-[124px] w-full shrink-0 overflow-hidden border-b border-line bg-field">
    <img src={url} alt="" draggable={false} className="h-full w-full object-cover" onError={() => setFailed(true)} />
  </div>;
}

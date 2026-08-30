import { BaseBoxShapeUtil, HTMLContainer } from "tldraw";
import type { TLIndicatorPath } from "tldraw";

import { DOCK_PATHS, DockIcon } from "../ui/dock-icons";
import { LINK_DEFAULTS, linkProps, type LinkShape } from "./shape-types";
import { boardObjectClass, roundedIndicator, tappable } from "./shape-shared";

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
    const { url, title, description, domain, image, favicon, status } = shape.props;

    return (
      <HTMLContainer>
        <article aria-label={"Link: " + (title || domain || url)} className={boardObjectClass(shape) + " rs-object board-link-object"}>
          {image ? (
            <div className="h-[124px] w-full shrink-0 overflow-hidden border-b border-line bg-field">
              {/* External URL by design — previews aren't workspace assets. draggable=false
                  keeps the browser's native image-drag from fighting the canvas. */}
              <img
                src={image}
                alt=""
                draggable={false}
                className="h-full w-full object-cover"
                onError={(e) => {
                  (e.currentTarget.parentElement as HTMLElement).style.display = "none";
                }}
              />
            </div>
          ) : null}
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
              <a
                href={/^https?:\/\//i.test(url) ? url : undefined}
                target="_blank"
                rel="noreferrer noopener"
                aria-label="Open link"
                {...tappable(this.editor, () => {})}
                className="board-source-open"
              >
                <DockIcon d={DOCK_PATHS.open} size={15} strokeWidth={2} />
              </a>
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

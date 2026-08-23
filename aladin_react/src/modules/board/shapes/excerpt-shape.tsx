import { BaseBoxShapeUtil, HTMLContainer, useValue } from "tldraw";
import type { TLIndicatorPath } from "tldraw";

import { EXCERPT_DEFAULTS, excerptProps, type ExcerptShape } from "./shape-types";
import { ShapeTextArea, roundedIndicator } from "./shape-shared";

/** A frozen quote: serif italic + `frozen` chip + citation. Double-tap edits the text. */
export class ExcerptShapeUtil extends BaseBoxShapeUtil<ExcerptShape> {
  static override type = "aladin-excerpt" as const;
  static override props = excerptProps;

  override getDefaultProps(): ExcerptShape["props"] {
    return { ...EXCERPT_DEFAULTS };
  }

  override canEdit() {
    return true;
  }

  override getIndicatorPath(shape: ExcerptShape): TLIndicatorPath {
    return roundedIndicator(shape.props.w, shape.props.h);
  }

  override component(shape: ExcerptShape) {
    const isEditing = useValue(
      "excerpt-editing",
      () => this.editor.getEditingShapeId() === shape.id,
      [shape.id],
    );
    const cite =
      shape.props.page != null
        ? `${shape.props.sourceTitle} · p. ${shape.props.page}`
        : shape.props.sourceTitle;

    return (
      <HTMLContainer>
        <div className="board-object px-4.5 py-4">
          <div className="font-serif text-board-quote italic text-ink">
            {isEditing ? (
              <ShapeTextArea
                editor={this.editor}
                value={shape.props.text}
                onChange={(next) =>
                  this.editor.updateShape({
                    id: shape.id,
                    type: shape.type,
                    props: { text: next },
                  })
                }
              />
            ) : (
              <div>{shape.props.text}</div>
            )}
          </div>
          <div className="mt-3.5 flex items-center gap-2">
            <span className="rounded-chip bg-field px-2 py-0.5 font-mono text-board-meta text-ink-3">
              frozen
            </span>
            <span className="truncate font-mono text-board-meta text-ink-4">{cite}</span>
          </div>
        </div>
      </HTMLContainer>
    );
  }
}

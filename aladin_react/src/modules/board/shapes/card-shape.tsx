import { BaseBoxShapeUtil, HTMLContainer, useValue } from "tldraw";
import type { TLIndicatorPath } from "tldraw";

import { CARD_DEFAULTS, cardProps, type CardShape } from "./shape-types";
import { ShapeTextArea, boardObjectClass, roundedIndicator } from "./shape-shared";

/**
 * Flashcard. The first tap selects (like every object); a tap on the SELECTED card flips
 * it (ShapeUtil.onClick — a click that was not a drag). Without that order you could never
 * select a card to move or remove it without answering it. Double-tap edits the visible
 * face. No scheduler, no due counts — product rule 5.
 */
export class CardShapeUtil extends BaseBoxShapeUtil<CardShape> {
  static override type = "aladin-card" as const;
  static override props = cardProps;

  override getDefaultProps(): CardShape["props"] {
    return { ...CARD_DEFAULTS };
  }

  override canEdit() {
    return true;
  }

  override onClick(shape: CardShape) {
    // Flipping while editing would yank the textarea out from under the caret.
    if (this.editor.getEditingShapeId() === shape.id) return;
    // Not yet selected: let the click select it (returning nothing leaves the click alone).
    if (!this.editor.getSelectedShapeIds().includes(shape.id)) return;
    return {
      id: shape.id,
      type: shape.type,
      props: { flipped: !shape.props.flipped },
    };
  }

  override getIndicatorPath(shape: CardShape): TLIndicatorPath {
    return roundedIndicator(shape.props.w, shape.props.h);
  }

  override component(shape: CardShape) {
    const isEditing = useValue(
      "card-editing",
      () => this.editor.getEditingShapeId() === shape.id,
      [shape.id],
    );
    const { flipped } = shape.props;
    const face = flipped ? "back" : "front";
    const text = flipped ? shape.props.back : shape.props.front;
    const faceClass = flipped ? "text-board-body text-ink-2" : "text-board-face text-ink";

    return (
      <HTMLContainer>
        <div className={boardObjectClass(shape) + " board-object--card flex flex-col px-4.5 py-4"}>
          <div className="flex items-center gap-2">
            <span className="text-board-learn font-mono text-board-meta uppercase tracking-wider">
              {face}
            </span>
            <span className="ml-auto font-mono text-board-meta text-ink-4">
              {flipped ? "tap again to hide" : "tap again to answer"}
            </span>
          </div>
          <div className={`mt-3 min-h-0 flex-1 ${faceClass}`}>
            {isEditing ? (
              <ShapeTextArea
                editor={this.editor}
                value={text}
                onChange={(next) =>
                  this.editor.updateShape({
                    id: shape.id,
                    type: shape.type,
                    props: flipped ? { back: next } : { front: next },
                  })
                }
                onNeedHeight={(needed) => {
                    if (needed > shape.props.h + 1) {
                      this.editor.updateShape({ id: shape.id, type: shape.type, props: { h: needed } });
                    }
                  }}
              />
            ) : (
              <div>{text}</div>
            )}
          </div>
          <div className="pt-3 font-mono text-board-meta text-ink-4">{shape.props.cite}</div>
        </div>
      </HTMLContainer>
    );
  }
}

import { BaseBoxShapeUtil, HTMLContainer, useValue } from "tldraw";
import type { TLIndicatorPath } from "tldraw";

import { CARD_DEFAULTS, cardProps, type CardShape } from "./shape-types";
import { ShapeTextArea, roundedIndicator } from "./shape-shared";

/**
 * Flashcard. Tap anywhere flips (ShapeUtil.onClick — a click that was not a drag);
 * double-tap edits the VISIBLE face. No scheduler, no due counts — product rule 5.
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
    const textStyle: React.CSSProperties = flipped
      ? { fontSize: 16, lineHeight: 1.4, color: "var(--ink-2)" }
      : { fontSize: 19, lineHeight: 1.4, color: "var(--ink)" };

    return (
      <HTMLContainer>
        <div
          style={{
            display: "flex",
            flexDirection: "column",
            width: "100%",
            height: "100%",
            padding: "16px 18px",
            borderRadius: 18,
            background: "var(--raise)",
            border: "2px solid rgb(var(--board-learn-line))",
            fontFamily: "'Space Grotesk Variable', 'Space Grotesk', system-ui, sans-serif",
          }}
        >
          <div style={{ display: "flex", alignItems: "center", gap: 9 }}>
            <span
              style={{
                fontFamily: "'JetBrains Mono', ui-monospace, monospace",
                fontSize: 10,
                letterSpacing: "0.07em",
                textTransform: "uppercase",
                color: "var(--board-learn)",
              }}
            >
              {face}
            </span>
            <span
              style={{
                marginLeft: "auto",
                fontFamily: "'JetBrains Mono', ui-monospace, monospace",
                fontSize: 10,
                color: "var(--ink-4)",
              }}
            >
              {flipped ? "tap to hide" : "tap to answer"}
            </span>
          </div>
          <div style={{ marginTop: 12, flex: 1, minHeight: 0 }}>
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
                style={{ ...textStyle, fontFamily: "inherit", height: "100%" }}
              />
            ) : (
              <div style={textStyle}>{text}</div>
            )}
          </div>
          <div
            style={{
              paddingTop: 12,
              fontFamily: "'JetBrains Mono', ui-monospace, monospace",
              fontSize: 10,
              color: "var(--ink-4)",
            }}
          >
            {shape.props.cite}
          </div>
        </div>
      </HTMLContainer>
    );
  }
}

import { BaseBoxShapeUtil, HTMLContainer, useValue } from "tldraw";
import type { TLIndicatorPath } from "tldraw";

import { EXCERPT_DEFAULTS, excerptProps, type ExcerptShape } from "./shape-types";
import { ShapeTextArea, roundedIndicator } from "./shape-shared";

/** A frozen quote: Georgia italic + `frozen` chip + citation. Double-tap edits the text. */
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
    const textStyle: React.CSSProperties = {
      fontFamily: "Georgia, 'Times New Roman', serif",
      fontStyle: "italic",
      fontSize: 20,
      lineHeight: 1.3,
      color: "var(--ink)",
    };
    const cite =
      shape.props.page != null
        ? `${shape.props.sourceTitle} · p. ${shape.props.page}`
        : shape.props.sourceTitle;

    return (
      <HTMLContainer>
        <div
          style={{
            width: "100%",
            height: "100%",
            padding: "16px 18px",
            borderRadius: 18,
            background: "var(--card)",
            border: "1px solid rgb(var(--line))",
          }}
        >
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
              style={textStyle}
            />
          ) : (
            <div style={textStyle}>{shape.props.text}</div>
          )}
          <div style={{ display: "flex", alignItems: "center", gap: 8, marginTop: 14 }}>
            <span
              style={{
                padding: "3px 9px",
                borderRadius: 8,
                background: "var(--field)",
                fontFamily: "'JetBrains Mono', ui-monospace, monospace",
                fontSize: 10,
                color: "var(--ink-3)",
              }}
            >
              frozen
            </span>
            <span
              style={{
                fontFamily: "'JetBrains Mono', ui-monospace, monospace",
                fontSize: 10.5,
                color: "var(--ink-4)",
                overflow: "hidden",
                textOverflow: "ellipsis",
                whiteSpace: "nowrap",
              }}
            >
              {cite}
            </span>
          </div>
        </div>
      </HTMLContainer>
    );
  }
}

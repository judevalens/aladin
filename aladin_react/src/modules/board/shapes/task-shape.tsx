import { BaseBoxShapeUtil, HTMLContainer, useValue } from "tldraw";
import type { TLIndicatorPath } from "tldraw";

import { TASK_DEFAULTS, taskProps, type TaskShape } from "./shape-types";
import { ShapeTextArea, roundedIndicator, tappable } from "./shape-shared";

/** Task: 30px checkbox + Caveat line. The checkbox is always tappable; double-tap edits. */
export class TaskShapeUtil extends BaseBoxShapeUtil<TaskShape> {
  static override type = "aladin-task" as const;
  static override props = taskProps;

  override getDefaultProps(): TaskShape["props"] {
    return { ...TASK_DEFAULTS };
  }

  override canEdit() {
    return true;
  }

  override getIndicatorPath(shape: TaskShape): TLIndicatorPath {
    return roundedIndicator(shape.props.w, shape.props.h);
  }

  override component(shape: TaskShape) {
    const isEditing = useValue(
      "task-editing",
      () => this.editor.getEditingShapeId() === shape.id,
      [shape.id],
    );
    const { checked } = shape.props;
    const textStyle: React.CSSProperties = {
      fontFamily: "'Caveat', cursive",
      fontSize: 27,
      lineHeight: 1.25,
      color: checked ? "var(--ink-4)" : "var(--amber)",
      textDecoration: checked ? "line-through" : "none",
    };

    return (
      <HTMLContainer>
        <div
          style={{
            display: "flex",
            alignItems: "flex-start",
            gap: 14,
            width: "100%",
            height: "100%",
            padding: "16px 18px",
            borderRadius: 18,
            background: "var(--card)",
            border: "1px solid rgb(var(--line))",
          }}
        >
          <span
            role="checkbox"
            aria-checked={checked}
            {...tappable(this.editor, () =>
              this.editor.updateShape({
                id: shape.id,
                type: shape.type,
                props: { checked: !checked },
              }),
            )}
            style={{
              display: "grid",
              placeItems: "center",
              flexShrink: 0,
              width: 30,
              height: 30,
              borderRadius: 9,
              border: `2px solid ${checked ? "var(--amber)" : "rgba(255,255,255,.24)"}`,
              background: checked ? "var(--amber)" : "transparent",
              pointerEvents: "all",
              cursor: "pointer",
            }}
          >
            <svg
              width="17"
              height="17"
              viewBox="0 0 24 24"
              fill="none"
              stroke="var(--on-amber)"
              strokeWidth="3"
              strokeLinecap="round"
              opacity={checked ? 1 : 0}
            >
              <path d="M5 13l4 4L19 7" />
            </svg>
          </span>
          <div style={{ minWidth: 0, flex: 1 }}>
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
                style={{ ...textStyle, textDecoration: "none" }}
              />
            ) : (
              <div style={textStyle}>{shape.props.text}</div>
            )}
            <div
              style={{
                marginTop: 6,
                fontFamily: "'JetBrains Mono', ui-monospace, monospace",
                fontSize: 10,
                letterSpacing: "0.05em",
                textTransform: "uppercase",
                color: "var(--ink-4)",
              }}
            >
              {shape.props.meta}
            </div>
          </div>
        </div>
      </HTMLContainer>
    );
  }
}

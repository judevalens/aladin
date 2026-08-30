import { BaseBoxShapeUtil, HTMLContainer, useValue } from "tldraw";
import type { TLIndicatorPath } from "tldraw";

import { DOCK_PATHS, DockIcon } from "../ui/dock-icons";
import { TASK_DEFAULTS, taskProps, type TaskShape } from "./shape-types";
import { ShapeTextArea, boardObjectClass, roundedIndicator, tappable } from "./shape-shared";

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

    return (
      <HTMLContainer>
        <div className={boardObjectClass(shape) + " flex items-start gap-3.5 px-4.5 py-4"}>
          {/* 44pt hit box around the 30px mark — a finger's target, the design's size. */}
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
            className="-m-[7px] grid h-11 w-11 shrink-0 place-items-center"
          >
            <span
              className={`board-tile grid h-[30px] w-[30px] place-items-center rounded-control border-2 ${
                checked ? "border-amber bg-amber text-on-amber" : "border-ink-4 bg-transparent"
              }`}
            >
              <span className={checked ? "opacity-100" : "opacity-0"}>
                <DockIcon d={DOCK_PATHS.check} size={17} strokeWidth={3} />
              </span>
            </span>
          </span>
          <div className="min-w-0 flex-1">
            <div
              className={`font-hand text-board-hand ${
                checked ? "text-ink-4 line-through" : "text-amber"
              }`}
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
                  onNeedHeight={(needed) => {
                      if (needed > shape.props.h + 1) {
                        this.editor.updateShape({ id: shape.id, type: shape.type, props: { h: needed } });
                      }
                    }}
                  className="no-underline"
                />
              ) : (
                <div>{shape.props.text}</div>
              )}
            </div>
            <div className="mt-1.5 font-mono text-board-meta uppercase tracking-wider text-ink-4">
              {shape.props.meta}
            </div>
          </div>
        </div>
      </HTMLContainer>
    );
  }
}

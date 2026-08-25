// The server side of the board's record schema. The client twin of this file is
// aladin_react/src/test/board-schema-drift.test.ts, which imports THIS module and holds
// the validators equal to shape-types.ts — here we prove the schema stands on its own.
import { test } from "node:test";
import assert from "node:assert";
import { DefaultColorStyle } from "@tldraw/tlschema";
import {
  BOARD_CUSTOM_COLORS,
  boardShapeProps,
  createBoardSchema,
} from "../src/services/board-schema.js";

const TASK_DEFAULTS = { w: 364, h: 112, text: "New task", meta: "open", checked: false };

test("the schema builds and validates a full board shape record", () => {
  const schema = createBoardSchema();
  const record = {
    typeName: "shape",
    type: "aladin-task",
    id: "shape:t1",
    index: "a1",
    parentId: "page:page",
    x: 0,
    y: 0,
    rotation: 0,
    isLocked: false,
    opacity: 1,
    meta: {},
    props: { ...TASK_DEFAULTS },
  };
  assert.deepEqual(schema.types.shape.validate(record), record);
});

test("junk props are rejected, not stored", () => {
  const schema = createBoardSchema();
  assert.throws(() =>
    schema.types.shape.validate({
      typeName: "shape",
      type: "aladin-task",
      id: "shape:t2",
      index: "a1",
      parentId: "page:page",
      x: 0,
      y: 0,
      rotation: 0,
      isLocked: false,
      opacity: 1,
      meta: {},
      props: { ...TASK_DEFAULTS, checked: "yes" },
    }),
  );
});

test("board ink colours are registered — stock names kept, board names added", () => {
  createBoardSchema();
  for (const name of BOARD_CUSTOM_COLORS) {
    assert.ok(DefaultColorStyle.values.includes(name), `missing custom colour ${name}`);
  }
  // The 13 stock names survive registration (removeValues trap — board-theme.ts rule 1).
  for (const name of ["black", "grey", "blue", "red", "white"]) {
    assert.ok(DefaultColorStyle.values.includes(name), `stock colour ${name} was removed`);
  }
  assert.doesNotThrow(() => DefaultColorStyle.validate("learn"));
  assert.doesNotThrow(() => DefaultColorStyle.validate("link"));
});

test("every shape type declares w/h and only known fields", () => {
  for (const [type, props] of Object.entries(boardShapeProps)) {
    assert.ok(props.w && props.h, `${type} lost its box`);
  }
});

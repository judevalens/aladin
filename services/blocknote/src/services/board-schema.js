// The board's record schema, server-side — the same four shapes the web client's
// `aladin_react/src/modules/board/shapes/shape-types.ts` defines, expressed without React
// so the sync room can validate every record it stores.
//
// TWO RULES, both load-bearing:
//
// 1. **These validators must never drift from the client's.** A record the client accepts
//    and the server rejects is INVALID_RECORD and a dropped edit; the reverse persists junk.
//    `aladin_react/src/test/board-schema-drift.test.ts` imports THIS file and holds the two
//    sides equal — change shapes there and here in the same commit.
//
// 2. **No migrations are defined — deliberately, on BOTH sides.** tldraw derives a
//    migration-sequence id from the shape type; a sequence that exists on one side only
//    makes the serialized schemas incomparable and the connection is refused. Identical
//    omission is identically comparable. When a shape's props first change shape, add the
//    sequence on both sides in the same commit.

import { T } from "@tldraw/validate";
import {
  DefaultColorStyle,
  createTLSchema,
  defaultBindingSchemas,
  defaultShapeSchemas,
  registerColorsFromThemes,
} from "@tldraw/tlschema";

/** The board's registered ink colours beyond tldraw's stock set (see board-theme.ts). */
export const BOARD_CUSTOM_COLORS = ["learn", "amber", "against", "link"];

/** Prop validators, mirroring shape-types.ts field for field. */
export const boardShapeProps = {
  "aladin-doc": {
    w: T.nonZeroNumber,
    h: T.nonZeroNumber,
    artifactId: T.string,
    artifactKind: T.string,
    title: T.string,
    page: T.number,
    pageCount: T.number,
    frozen: T.boolean,
  },
  "aladin-excerpt": {
    w: T.nonZeroNumber,
    h: T.nonZeroNumber,
    text: T.string,
    sourceArtifactId: T.string.nullable(),
    sourceTitle: T.string,
    page: T.number.nullable(),
  },
  "aladin-task": {
    w: T.nonZeroNumber,
    h: T.nonZeroNumber,
    text: T.string,
    meta: T.string,
    checked: T.boolean,
  },
  "aladin-card": {
    w: T.nonZeroNumber,
    h: T.nonZeroNumber,
    front: T.string,
    back: T.string,
    cite: T.string,
    flipped: T.boolean,
  },
};

/**
 * Registers the board's colour names into tldraw's global colour style.
 *
 * `registerColorsFromThemes` REMOVES any registered colour absent from the given themes
 * (the client learned this the hard way — board-theme.ts rule 1), so the definitions here
 * are the stock names currently registered PLUS the board's own. The palette VALUES are
 * irrelevant server-side — validation cares about names — so empty objects suffice.
 */
export function registerBoardColors() {
  const names = new Set([...DefaultColorStyle.values, ...BOARD_CUSTOM_COLORS]);
  const palette = Object.fromEntries([...names].map((name) => [name, {}]));
  registerColorsFromThemes({
    default: { id: "default", colors: { light: palette, dark: palette } },
  });
}

/** The room's schema: tldraw's defaults + the four board shapes, board colours registered. */
export function createBoardSchema() {
  registerBoardColors();
  return createTLSchema({
    shapes: {
      ...defaultShapeSchemas,
      "aladin-doc": { props: boardShapeProps["aladin-doc"] },
      "aladin-excerpt": { props: boardShapeProps["aladin-excerpt"] },
      "aladin-task": { props: boardShapeProps["aladin-task"] },
      "aladin-card": { props: boardShapeProps["aladin-card"] },
    },
    bindings: defaultBindingSchemas,
  });
}

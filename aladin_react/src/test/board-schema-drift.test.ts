/**
 * The drift gate between the board's TWO schema definitions: the client's shape-types.ts
 * (TypeScript, feeds useSync) and the room server's board-schema.js (plain JS in
 * services/blocknote, feeds TLSocketRoom). A record one side accepts and the other
 * rejects is INVALID_RECORD — a dropped edit — so the validators must move together, in
 * the same commit, forever.
 */
import { describe, expect, it } from "vitest";

// eslint-disable-next-line import/no-relative-packages -- the drift gate IS the point
import {
  BOARD_CUSTOM_COLORS,
  boardShapeProps as serverProps,
} from "../../../services/blocknote/src/services/board-schema.js";
import { BOARD_INK_COLORS } from "@/modules/board/domain/board-theme";
import {
  CARD_DEFAULTS,
  DOC_WINDOW_DEFAULTS,
  EXCERPT_DEFAULTS,
  TASK_DEFAULTS,
  cardProps,
  docWindowProps,
  excerptProps,
  taskProps,
} from "@/modules/board/shapes/shape-types";

type Validators = Record<string, { validate(value: unknown): unknown }>;

const clientProps: Record<string, Validators> = {
  "aladin-doc": docWindowProps as unknown as Validators,
  "aladin-excerpt": excerptProps as unknown as Validators,
  "aladin-task": taskProps as unknown as Validators,
  "aladin-card": cardProps as unknown as Validators,
};

const defaults: Record<string, Record<string, unknown>> = {
  "aladin-doc": DOC_WINDOW_DEFAULTS as unknown as Record<string, unknown>,
  "aladin-excerpt": EXCERPT_DEFAULTS as unknown as Record<string, unknown>,
  "aladin-task": TASK_DEFAULTS as unknown as Record<string, unknown>,
  "aladin-card": CARD_DEFAULTS as unknown as Record<string, unknown>,
};

/** Per-field probes: values a correct validator must accept/reject identically. */
const PROBES: unknown[] = [0, 1, -2, "", "text", true, false, null, undefined, {}, [1]];

function accepts(validator: { validate(value: unknown): unknown }, value: unknown): boolean {
  try {
    validator.validate(value);
    return true;
  } catch {
    return false;
  }
}

describe("board schema drift (client shape-types.ts vs server board-schema.js)", () => {
  it("defines the same shape types", () => {
    expect(Object.keys(serverProps).sort()).toEqual(Object.keys(clientProps).sort());
  });

  it("defines the same prop fields per type", () => {
    for (const type of Object.keys(clientProps)) {
      expect(Object.keys(serverProps[type]).sort(), type).toEqual(
        Object.keys(clientProps[type]).sort(),
      );
    }
  });

  it("accepts and rejects identically, field by field", () => {
    for (const [type, fields] of Object.entries(clientProps)) {
      for (const [field, clientValidator] of Object.entries(fields)) {
        const serverValidator = (serverProps as Record<string, Validators>)[type][field];
        for (const probe of PROBES) {
          expect(
            accepts(serverValidator, probe),
            `${type}.${field} disagrees on ${JSON.stringify(probe) ?? "undefined"}`,
          ).toBe(accepts(clientValidator, probe));
        }
        // The shape's own default must pass BOTH.
        const good = defaults[type][field];
        expect(accepts(clientValidator, good), `${type}.${field} client default`).toBe(true);
        expect(accepts(serverValidator, good), `${type}.${field} server default`).toBe(true);
      }
    }
  });

  it("the server registers every ink colour the dock offers", () => {
    for (const color of BOARD_INK_COLORS) {
      expect(BOARD_CUSTOM_COLORS, `dock colour ${color}`).toContain(color);
    }
  });
});

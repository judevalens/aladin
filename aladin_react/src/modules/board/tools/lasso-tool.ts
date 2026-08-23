import { SelectTool, StateNode } from "tldraw";
import type { TLStateNodeConstructor } from "tldraw";

/**
 * The dock's Lasso: tldraw has no standalone lasso tool — scribble-select is the
 * SelectTool's `scribble_brushing` child state, normally entered from `brushing` only
 * while Alt is held (Brushing.onEnter). No key exists on an iPad, so this tool is the
 * SelectTool with one change: entering `brushing` immediately forwards to
 * `scribble_brushing`.
 *
 * The child states are not exported, so the brushing constructor is taken from
 * `SelectTool.children()` at module load and subclassed. `ScribbleBrushing.onKeyUp`
 * can bounce back to `brushing` (alt released) — the forward here turns that into a
 * no-op loop back into scribble, which is exactly the wanted behavior.
 */

const stockChildren = SelectTool.children();
const BrushingBase = stockChildren.find((c) => c.id === "brushing") as
  | (typeof StateNode & TLStateNodeConstructor)
  | undefined;

if (!BrushingBase) {
  throw new Error("tldraw SelectTool no longer has a 'brushing' child state");
}

class LassoBrushing extends BrushingBase {
  static override id = "brushing";

  override onEnter(info: unknown) {
    this.parent.transition("scribble_brushing", info);
  }
}

export class BoardLassoTool extends SelectTool {
  static override id = "lasso";

  static override children(): TLStateNodeConstructor[] {
    return stockChildren.map((c) => (c.id === "brushing" ? LassoBrushing : c));
  }
}

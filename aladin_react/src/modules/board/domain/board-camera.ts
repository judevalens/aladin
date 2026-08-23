import type { TLCameraOptions } from "tldraw";

/**
 * The two camera regimes of the board.
 *
 * Default: pinch and the zoom pill walk these steps. tldraw clamps ALL zoom — pinch
 * included — to `zoomSteps[0]..last`, so the range is the board's whole reading range:
 * 10% shows a wall of windows as thumbnails, 400% makes a PDF excerpt legible under a
 * pencil. The wheel pans (trackpad pinch still zooms — tldraw treats ctrl+wheel as zoom
 * regardless of wheelBehavior).
 *
 * Pencil: the handoff's rule 3 — "Pencil owns the camera". Clamping `zoomSteps` to [1]
 * makes pinch-zoom a no-op while **two-finger pan keeps working**; `isLocked` would kill
 * the pan too, which is why it is never used here. Ink is therefore always laid down at
 * world scale 1:1.
 */
export const BOARD_ZOOM_STEPS = [0.1, 0.25, 0.5, 0.75, 1, 1.5, 2, 4];

export const boardCameraOptions: Partial<TLCameraOptions> = {
  zoomSteps: BOARD_ZOOM_STEPS,
  wheelBehavior: "pan",
};

export const pencilCameraOptions: Partial<TLCameraOptions> = {
  zoomSteps: [1],
  wheelBehavior: "pan",
};

/** Screen-space pointer travel beyond which a press is a drag, not a hold or a tap. */
export const TAP_SLOP_PX = 8;

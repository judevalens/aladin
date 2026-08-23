import type { TLCameraOptions } from "tldraw";

/**
 * The two camera regimes of the board.
 *
 * Default: the zoom pill walks these steps (50–150%); the wheel pans (trackpad pinch still
 * zooms — tldraw treats ctrl+wheel as zoom regardless of wheelBehavior).
 *
 * Pencil: the handoff's rule 3 — "Pencil owns the camera". Clamping `zoomSteps` to [1]
 * makes pinch-zoom a no-op while **two-finger pan keeps working**; `isLocked` would kill
 * the pan too, which is why it is never used here. Ink is therefore always laid down at
 * world scale 1:1.
 */
export const BOARD_ZOOM_STEPS = [0.5, 0.75, 1, 1.25, 1.5];

export const boardCameraOptions: Partial<TLCameraOptions> = {
  zoomSteps: BOARD_ZOOM_STEPS,
  wheelBehavior: "pan",
};

export const pencilCameraOptions: Partial<TLCameraOptions> = {
  zoomSteps: [1],
  wheelBehavior: "pan",
};

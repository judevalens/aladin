import { TAP_SLOP_PX } from "./board-camera";

/**
 * Multi-finger taps — two fingers = undo, three = redo. The muscle memory every iPad
 * drawing app trained (Procreate, Concepts); users will try it before they find a button.
 *
 * Pure: fed touch starts/moves/ends with ids, screen points and times, it decides when a
 * gesture was a clean tap of N fingers. A tap is: every finger down within the window,
 * none travelled more than the slop, all lifted within `maxDurationMs` of the first touch.
 * Three-finger SWIPES are left alone (they are the system's undo/redo) — movement cancels.
 */
export interface TapTrackerOptions {
  onTap: (fingers: 2 | 3) => void;
  maxDurationMs?: number;
  slopPx?: number;
}

export interface TapTracker {
  start(id: number, x: number, y: number, t: number): void;
  move(id: number, x: number, y: number): void;
  end(id: number, t: number): void;
  cancel(): void;
}

export function createTapTracker(options: TapTrackerOptions): TapTracker {
  const maxDuration = options.maxDurationMs ?? 250;
  const slop = options.slopPx ?? TAP_SLOP_PX;
  const origins = new Map<number, { x: number; y: number }>();
  let startedAt = 0;
  let maxFingers = 0;
  let spoiled = false;

  function reset() {
    origins.clear();
    startedAt = 0;
    maxFingers = 0;
    spoiled = false;
  }

  return {
    start(id, x, y, t) {
      if (origins.size === 0) {
        reset();
        startedAt = t;
      }
      origins.set(id, { x, y });
      maxFingers = Math.max(maxFingers, origins.size);
      if (maxFingers > 3) spoiled = true;
    },
    move(id, x, y) {
      const origin = origins.get(id);
      if (!origin) return;
      if (Math.hypot(x - origin.x, y - origin.y) > slop) spoiled = true;
    },
    end(id, t) {
      if (!origins.has(id)) return;
      origins.delete(id);
      if (origins.size > 0) return;
      const fingers = maxFingers;
      const clean = !spoiled && t - startedAt <= maxDuration;
      reset();
      if (clean && (fingers === 2 || fingers === 3)) options.onTap(fingers);
    },
    cancel() {
      reset();
    },
  };
}

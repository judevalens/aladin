/**
 * The board's outbound save machine — pure, timer-injectable, no editor and no DOM.
 *
 * Rules it enforces (each one a real bug when missing):
 * - Nothing saves until `arm()` — a failed LOAD must never let the next edit PATCH an
 *   empty snapshot over the server's board.
 * - Edits coalesce behind a debounce; a failed save keeps the board dirty and retries on a
 *   backoff ladder (1s · 2s · 4s … capped) — a failed save is never dropped.
 * - An edit during a pending retry does not reset the backoff; the retry saves the newest
 *   snapshot when it fires. A success that raced a newer edit saves again at once.
 * - `flush()` (pane hidden / page hidden / unmount) saves immediately if dirty.
 */

export type BoardSaveState = "saved" | "dirty" | "saving" | "error";

export interface BoardSaverTimers {
  set: (fn: () => void, ms: number) => unknown;
  clear: (handle: unknown) => void;
}

export interface BoardSaverOptions {
  /** Perform one save of the CURRENT snapshot; reject on failure. */
  save: () => Promise<void>;
  onState?: (state: BoardSaveState, error?: unknown) => void;
  debounceMs?: number;
  retryBaseMs?: number;
  retryCapMs?: number;
  timers?: BoardSaverTimers;
}

export interface BoardSaver {
  /** The load succeeded — edits now count. */
  arm(): void;
  /** An edit happened. No-op until armed. */
  markDirty(): void;
  /** Save now if anything is pending (hide / unmount). */
  flush(): void;
  /** Stop all timers and callbacks. */
  dispose(): void;
  readonly armed: boolean;
  readonly dirty: boolean;
  readonly state: BoardSaveState;
}

const DEFAULT_TIMERS: BoardSaverTimers = {
  set: (fn, ms) => setTimeout(fn, ms),
  clear: (handle) => clearTimeout(handle as ReturnType<typeof setTimeout>),
};

export function createBoardSaver(options: BoardSaverOptions): BoardSaver {
  const debounceMs = options.debounceMs ?? 700;
  const retryBaseMs = options.retryBaseMs ?? 1000;
  const retryCapMs = options.retryCapMs ?? 30_000;
  const timers = options.timers ?? DEFAULT_TIMERS;

  let armed = false;
  let dirty = false;
  let saving = false;
  let disposed = false;
  let retries = 0;
  let state: BoardSaveState = "saved";
  let debounceHandle: unknown = null;
  let retryHandle: unknown = null;

  function setState(next: BoardSaveState, error?: unknown) {
    state = next;
    if (!disposed) options.onState?.(next, error);
  }

  function clearDebounce() {
    if (debounceHandle !== null) {
      timers.clear(debounceHandle);
      debounceHandle = null;
    }
  }

  function clearRetry() {
    if (retryHandle !== null) {
      timers.clear(retryHandle);
      retryHandle = null;
    }
  }

  async function saveNow() {
    if (disposed || !armed || saving || !dirty) return;
    saving = true;
    dirty = false;
    setState("saving");
    try {
      await options.save();
      saving = false;
      retries = 0;
      if (disposed) return;
      if (dirty) {
        void saveNow();
      } else {
        setState("saved");
      }
    } catch (error) {
      saving = false;
      dirty = true;
      if (disposed) return;
      setState("error", error);
      const delay = Math.min(retryCapMs, retryBaseMs * 2 ** retries);
      retries += 1;
      clearRetry();
      retryHandle = timers.set(() => {
        retryHandle = null;
        void saveNow();
      }, delay);
    }
  }

  return {
    arm() {
      armed = true;
    },
    markDirty() {
      if (disposed || !armed) return;
      dirty = true;
      if (state !== "error") setState("dirty");
      // A backoff already waiting keeps its slot — it saves the newest snapshot when it fires.
      if (retryHandle !== null) return;
      clearDebounce();
      debounceHandle = timers.set(() => {
        debounceHandle = null;
        void saveNow();
      }, debounceMs);
    },
    flush() {
      if (disposed || !dirty) return;
      clearDebounce();
      clearRetry();
      void saveNow();
    },
    dispose() {
      disposed = true;
      clearDebounce();
      clearRetry();
    },
    get armed() {
      return armed;
    },
    get dirty() {
      return dirty;
    },
    get state() {
      return state;
    },
  };
}

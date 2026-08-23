import { createContext, useContext, useSyncExternalStore } from "react";

/**
 * The board's transient messages — "Removed from board · Undo", "allow paste, or use ⌘V".
 * One at a time (the newest replaces), auto-dismissed, with at most one action. A tiny
 * external store so shapes, the chrome and the pane can all raise one without threading
 * callbacks; the chrome renders it with useSyncExternalStore.
 */
export interface BoardToast {
  id: number;
  text: string;
  action?: { label: string; onPress: () => void };
}

export interface BoardToastStore {
  /** Show a toast; returns its id. */
  show(toast: Omit<BoardToast, "id">, durationMs?: number): number;
  dismiss(id?: number): void;
  subscribe(onChange: () => void): () => void;
  get(): BoardToast | null;
}

export interface ToastTimers {
  set: (fn: () => void, ms: number) => unknown;
  clear: (handle: unknown) => void;
}

const DEFAULT_TIMERS: ToastTimers = {
  set: (fn, ms) => setTimeout(fn, ms),
  clear: (handle) => clearTimeout(handle as ReturnType<typeof setTimeout>),
};

export function createToastStore(timers: ToastTimers = DEFAULT_TIMERS): BoardToastStore {
  let current: BoardToast | null = null;
  let handle: unknown = null;
  let nextId = 1;
  const listeners = new Set<() => void>();

  function emit() {
    for (const listener of listeners) listener();
  }

  function clearTimer() {
    if (handle !== null) {
      timers.clear(handle);
      handle = null;
    }
  }

  return {
    show(toast, durationMs = 4000) {
      clearTimer();
      const id = nextId++;
      current = { ...toast, id };
      handle = timers.set(() => {
        handle = null;
        if (current?.id === id) {
          current = null;
          emit();
        }
      }, durationMs);
      emit();
      return id;
    },
    dismiss(id) {
      if (!current || (id !== undefined && current.id !== id)) return;
      clearTimer();
      current = null;
      emit();
    },
    subscribe(onChange) {
      listeners.add(onChange);
      return () => {
        listeners.delete(onChange);
      };
    },
    get: () => current,
  };
}

const NOOP_STORE: BoardToastStore = {
  show: () => 0,
  dismiss: () => {},
  subscribe: () => () => {},
  get: () => null,
};

export const BoardToastContext = createContext<BoardToastStore>(NOOP_STORE);

export function useBoardToasts(): BoardToastStore {
  return useContext(BoardToastContext);
}

export function useCurrentToast(): BoardToast | null {
  const store = useBoardToasts();
  return useSyncExternalStore(store.subscribe, store.get, store.get);
}

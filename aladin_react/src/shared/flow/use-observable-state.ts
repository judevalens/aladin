import { useMemo, useSyncExternalStore } from "react";
import type { Observable } from "rxjs";
import { data, failed, loading, type Loadable } from "./loadable";
import { toError, type Result } from "./result";

interface ObservableStore<T> {
  subscribe: (onChange: () => void) => () => void;
  getSnapshot: () => Loadable<T>;
}

function isResult(value: unknown): value is Result<unknown> {
  if (typeof value !== "object" || value === null) return false;
  const r = value as { ok?: unknown; value?: unknown; error?: unknown };
  if (typeof r.ok !== "boolean") return false;
  return r.ok ? "value" in r : "error" in r;
}

function toLoadable<T>(value: unknown): Loadable<T> {
  if (isResult(value)) {
    return value.ok
      ? data(value.value as T)
      : failed<T>(value.error instanceof Error ? value.error : toError(value.error));
  }
  return data(value as T);
}

function createObservableStore<T>(observable$: Observable<unknown>): ObservableStore<T> {
  let current: Loadable<T> = loading<T>();

  const primer = observable$.subscribe({
    next: (value) => {
      current = toLoadable<T>(value);
    },
    error: (error) => {
      current = failed<T>(toError(error));
    },
  });
  primer.unsubscribe();

  return {
    getSnapshot: () => current,
    subscribe: (onChange) => {
      const subscription = observable$.subscribe({
        next: (value) => {
          current = toLoadable<T>(value);
          onChange();
        },
        error: (error) => {
          current = failed<T>(toError(error));
          onChange();
        },
      });
      return () => subscription.unsubscribe();
    },
  };
}

export function useObservableState<T>(
  observable$: Observable<Result<T>>,
): Loadable<T>;
export function useObservableState<T>(observable$: Observable<T>): Loadable<T>;
export function useObservableState<T>(observable$: Observable<T>): Loadable<T> {
  const store = useMemo(
    () => createObservableStore<T>(observable$),
    [observable$],
  );
  return useSyncExternalStore(store.subscribe, store.getSnapshot);
}

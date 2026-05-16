import { type DependencyList, useEffect, useRef, useState } from "react";
import type { Observable } from "rxjs";

export function useObservableState<T>(
  observe: () => Observable<T>,
  getSnapshot: () => T,
  dependencies: DependencyList,
) {
  const observeRef = useRef(observe);
  const getSnapshotRef = useRef(getSnapshot);
  observeRef.current = observe;
  getSnapshotRef.current = getSnapshot;

  const [value, setValue] = useState<T>(() => getSnapshot());

  useEffect(() => {
    setValue(getSnapshotRef.current());
    const subscription = observeRef.current().subscribe((next) => {
      setValue(next);
    });
    return () => {
      subscription.unsubscribe();
    };
  }, dependencies);

  return value;
}

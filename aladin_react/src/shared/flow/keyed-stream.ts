import {
  catchError,
  defer,
  filter,
  from,
  ignoreElements,
  map,
  merge,
  of,
  Subject,
  takeUntil,
  tap,
  type Observable,
} from "rxjs";
import { err, ok, toError, type Result } from "./result";

/**
 * Keyed event stream with per-subscription snapshot.
 *
 * - `observe(key)` fetches the current value via `fetch(key)` and pushes it
 *   into the shared subject. All matching subscribers — including the caller
 *   and anyone already listening on the same key — receive it via the live
 *   channel. The snapshot pipeline itself emits only on error.
 * - `push(value)` broadcasts an update to all matching subscribers.
 * - Errors from the snapshot fetch are emitted as `err` Results to the
 *   triggering subscriber only; the live channel never errors, so other
 *   subscribers are unaffected and everyone recovers on the next successful
 *   push.
 *
 * Per-key caching lives in the repo, not here — this stream only coordinates
 * fan-out, not storage. Concurrent `observe(key)` calls for the same key will
 * each call `fetch`; in-flight dedup is the repo's responsibility if needed.
 */
export class KeyedStream<K, T> {
  private readonly subject = new Subject<T>();
  private readonly observables = new Map<K, Observable<Result<T>>>();

  constructor(
    private readonly keyOf: (value: T) => K,
    private readonly fetch: (key: K) => Promise<T>,
  ) {}

  observe(key: K): Observable<Result<T>> {
    let cached = this.observables.get(key);
    if (cached) return cached;
    cached = defer(() => {
      const live = this.subject.pipe(
        filter((value) => this.keyOf(value) === key),
        map((value) => ok(value)),
      );
      const snapshot = from(this.fetch(key)).pipe(
        tap((value) => this.subject.next(value)),
        ignoreElements(),
        catchError((error) => of(err<T>(toError(error)))),
        takeUntil(live),
      );
      return merge(snapshot, live);
    });
    this.observables.set(key, cached);
    return cached;
  }

  push(value: T) {
    this.subject.next(value);
  }
}

import { defer, ReplaySubject, Subject, type Observable } from "rxjs";
import { err, ok, toError, type Result } from "./result";

/**
 * Singleton stream with lazy first-fetch on subscribe.
 *
 * - Subscribers receive the latest cached value via the default ReplaySubject(1).
 * - First subscription triggers `fetch()` once; concurrent calls share the
 *   in-flight promise.
 * - `refresh()` returns a Promise that rejects on failure so imperative callers
 *   (e.g. mutation chains) can `await` and catch. The error is also pushed into
 *   the stream as an `err` Result so reactive consumers see it.
 *
 * Pass a custom `subjectFactory` to swap the cache semantics
 * (`BehaviorSubject` for a seed value, plain `Subject` for fan-out only).
 */
export class LazyResultStream<T> {
  private readonly subject: Subject<Result<T>>;
  private readonly observable$: Observable<Result<T>>;
  private loading: Promise<void> | null = null;
  private loaded = false;

  constructor(
    private readonly fetch: () => Promise<T>,
    subjectFactory: () => Subject<Result<T>> = () =>
      new ReplaySubject<Result<T>>(1),
  ) {
    this.subject = subjectFactory();
    this.observable$ = defer(() => {
      if (!this.loaded && !this.loading) {
        void this.refresh().catch(() => {});
      }
      return this.subject.asObservable();
    });
  }

  observable(): Observable<Result<T>> {
    return this.observable$;
  }

  refresh(): Promise<void> {
    if (this.loading) return this.loading;
    this.loading = this.fetch()
      .then((value) => {
        this.loaded = true;
        this.subject.next(ok(value));
      })
      .catch((error) => {
        const e = toError(error);
        this.subject.next(err<T>(e));
        throw e;
      })
      .finally(() => {
        this.loading = null;
      });
    return this.loading;
  }

  push(value: T) {
    this.loaded = true;
    this.subject.next(ok(value));
  }
}

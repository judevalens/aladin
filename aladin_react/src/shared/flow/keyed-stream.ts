import { defer, finalize, ReplaySubject, type Observable } from "rxjs";
import { err, ok, toError, type Result } from "./result";

/**
 * Keyed read model: one stream per key that HOLDS ITS CURRENT VALUE.
 *
 * `observe(key)` is "give me what you have, then every update": a subscriber gets the retained
 * value synchronously on subscribe and never sees a gap. The first subscriber for a key triggers
 * `fetch(key)`; later ones ride the value already held. `push(value)` — the syncer's frames —
 * replaces it.
 *
 * **Why retention, and not just fan-out.** This used to be a plain `Subject` with a
 * fetch-per-subscription: the observable was a recipe, not a value, so every resubscribe went
 * back to "nothing yet" until a promise landed. That made SUBSCRIPTION LIFETIME the real state
 * container, and lifetime is decided by incidental things — a React memo dep, a component
 * remounting, an unrelated tab closing. Closing one work-pane tab rebuilt the whole artifact
 * cache stream, which reported every open artifact as missing for one frame and tore down the
 * panes reading them: a PDF's pdf.js document destroyed and re-fetched, a note's Yjs socket
 * reconnected, a shard's iframe rebuilt. Holding the value makes resubscribing free, so callers
 * are free to subscribe per consumer instead of hoarding one shared subscription.
 *
 * **Eviction.** Retention is bounded: keys with no subscribers are dropped least-recently-used
 * first, past `retainedKeys`. Live subscribers are never evicted. Eviction is by USE rather than
 * by a timer so it stays deterministic — and it deliberately does NOT drop a key the moment its
 * last subscriber leaves, because a teardown-then-resubscribe inside one React commit is exactly
 * the case this class exists to survive.
 *
 * A failed `fetch` is emitted as an `err` Result and leaves the key unresolved, so the next
 * subscriber retries rather than inheriting the failure forever.
 */
export class KeyedStream<K, T> {
  /** Insertion order IS the LRU order: `touch` re-inserts. */
  private readonly entries = new Map<K, Entry<T>>();

  constructor(
    private readonly keyOf: (value: T) => K,
    private readonly fetch: (key: K) => Promise<T>,
    private readonly retainedKeys = 64,
  ) {}

  observe(key: K): Observable<Result<T>> {
    return this.entry(key).observable;
  }

  push(value: T) {
    const key = this.keyOf(value);
    const entry = this.entry(key);
    entry.resolved = true;
    entry.subject.next(ok(value));
    this.evict();
  }

  /**
   * One entry per key, created once — the observable's IDENTITY is stable, which is what lets
   * `useObservableState` memoise on it and what makes two consumers of the same key share a
   * single fetch.
   */
  private entry(key: K): Entry<T> {
    const existing = this.entries.get(key);
    if (existing) {
      this.touch(key, existing);
      return existing;
    }
    const entry: Entry<T> = {
      subject: new ReplaySubject<Result<T>>(1),
      subscribers: 0,
      resolved: false,
      loading: null,
      observable: defer(() => {
        entry.subscribers += 1;
        this.touch(key, entry);
        if (!entry.resolved && !entry.loading) void this.load(key, entry);
        return entry.subject.asObservable().pipe(
          finalize(() => {
            entry.subscribers -= 1;
            this.evict();
          }),
        );
      }),
    };
    this.entries.set(key, entry);
    this.evict();
    return entry;
  }

  private load(key: K, entry: Entry<T>) {
    entry.loading = this.fetch(key)
      .then((value) => {
        entry.resolved = true;
        entry.subject.next(ok(value));
      })
      .catch((error: unknown) => {
        entry.subject.next(err<T>(toError(error)));
      })
      .finally(() => {
        entry.loading = null;
      });
    return entry.loading;
  }

  private touch(key: K, entry: Entry<T>) {
    this.entries.delete(key);
    this.entries.set(key, entry);
  }

  /** Drops unobserved keys, oldest first, until the retained set is back within its cap. */
  private evict() {
    if (this.entries.size <= this.retainedKeys) return;
    for (const [key, entry] of this.entries) {
      if (this.entries.size <= this.retainedKeys) return;
      if (entry.subscribers > 0) continue;
      entry.subject.complete();
      this.entries.delete(key);
    }
  }
}

interface Entry<T> {
  subject: ReplaySubject<Result<T>>;
  observable: Observable<Result<T>>;
  subscribers: number;
  /** A value has been held at least once — a later subscriber must not re-fetch. */
  resolved: boolean;
  loading: Promise<void> | null;
}

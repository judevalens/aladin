/**
 * The reading-position writer's gate + debounce (pure, fake-timer testable).
 *
 * The trap this exists for: the reader reports its visible page from an
 * IntersectionObserver that fires "page 1" at mount — BEFORE the synced position
 * has loaded or the restore jump has landed. A naive writer publishes `page: 1`
 * on every open and clobbers the position on every device. So:
 *
 *   - nothing is sent until `arm(restoredPage)` — the pane arms once the synced
 *     position has resolved (present or absent);
 *   - a page equal to the last known position is never sent, and CANCELS any
 *     pending send (the mount-observer's "1" schedules, the restore jump's
 *     arrival cancels);
 *   - sends debounce (trailing) — the interval is also a durability parameter:
 *     every report is an outbox row on the server, so never write per scroll tick;
 *   - `flush()` sends a still-pending page immediately (close/hide).
 */
export interface PositionReporter {
  /** Start reporting; `restoredPage` (null = none) seeds "last known". */
  arm(restoredPage: number | null): void;
  /** The visible page changed (observer tick, jump, outline click). */
  notePage(page: number): void;
  /** Send any pending page now (pane closing / app hiding). */
  flush(): void;
}

export function createPositionReporter(
  send: (page: number) => void,
  debounceMs = 2000,
): PositionReporter {
  let armed = false;
  let lastKnown: number | null = null;
  let pending: number | null = null;
  let timer: ReturnType<typeof setTimeout> | null = null;

  const cancel = () => {
    if (timer) {
      clearTimeout(timer);
      timer = null;
    }
    pending = null;
  };

  const fire = () => {
    timer = null;
    const page = pending;
    pending = null;
    if (page != null) {
      lastKnown = page;
      send(page);
    }
  };

  return {
    arm(restoredPage) {
      armed = true;
      lastKnown = restoredPage;
    },
    notePage(page) {
      if (!armed) return;
      if (page === lastKnown) {
        cancel();
        return;
      }
      pending = page;
      if (timer) clearTimeout(timer);
      timer = setTimeout(fire, debounceMs);
    },
    flush() {
      if (!timer && pending == null) return;
      if (timer) clearTimeout(timer);
      fire();
    },
  };
}

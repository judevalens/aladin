import { Channel, invoke } from "@tauri-apps/api/core";
import { Subject, type Observable } from "rxjs";
import type { NodeRow } from "@/repos/local-repo-types";
import type { ShardKvRow } from "@/repos/shard-kv/local-shard-kv-types";
import type { LocalWatchlist } from "@/repos/watchlist/local-watchlist-types";

export interface EntityDeletedEvent {
  id: string;
}

// Data-layer — the workspace data events emitted by the pull/live engines after
// applying a frame. Node events drive the tree (`nodes`); watchlist events drive
// the Markets switcher (`watchlists`).
export type DataEvent =
  | { type: "nodeUpserted"; payload: NodeRow }
  | { type: "nodeDeleted"; payload: EntityDeletedEvent }
  | { type: "watchlistUpserted"; payload: LocalWatchlist }
  | { type: "watchlistDeleted"; payload: EntityDeletedEvent }
  | { type: "shardKvUpserted"; payload: ShardKvRow }
  | { type: "shardKvDeleted"; payload: EntityDeletedEvent };

export interface DataEventsRepo {
  events(): Observable<DataEvent>;
  connect(): Promise<void>;
}

export function createDataEventsRepo(): DataEventsRepo {
  const subject = new Subject<DataEvent>();
  let connected = false;

  return {
    events() {
      return subject.asObservable();
    },
    async connect() {
      if (connected) return;
      connected = true;
      const channel = new Channel<DataEvent>((event) => {
        subject.next(event);
      });
      await invoke("sync_subscribe_data_events", { channel });
    },
  };
}

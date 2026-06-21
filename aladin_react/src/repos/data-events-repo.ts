import { Channel, invoke } from "@tauri-apps/api/core";
import { Subject, type Observable } from "rxjs";
import type { NodeRow } from "@/repos/local-repo-types";
import type { ClaimSignal } from "@/modules/signals/signal-types";

export interface EntityDeletedEvent {
  id: string;
}

// Data-layer — the workspace data events emitted by the pull/live engines after
// applying a frame. Node events drive the tree (`nodes`); signal events drive the
// Signals/claim feed (`signals`).
export type DataEvent =
  | { type: "nodeUpserted"; payload: NodeRow }
  | { type: "nodeDeleted"; payload: EntityDeletedEvent }
  | { type: "signalUpserted"; payload: ClaimSignal }
  | { type: "signalDeleted"; payload: EntityDeletedEvent };

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

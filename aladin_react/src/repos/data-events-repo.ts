import { Channel, invoke } from "@tauri-apps/api/core";
import { Subject, type Observable } from "rxjs";
import type {
  ArtifactRow,
  BrowserNodeRow,
  PageContentRow,
} from "@/repos/local-repo-types";

export interface EntityDeletedEvent {
  id: string;
}

export type DataEvent =
  | { type: "browserNodeCreated"; payload: BrowserNodeRow }
  | { type: "browserNodeUpdated"; payload: BrowserNodeRow }
  | { type: "browserNodeDeleted"; payload: EntityDeletedEvent }
  | { type: "artifactChanged"; payload: ArtifactRow }
  | { type: "artifactDeleted"; payload: EntityDeletedEvent }
  | { type: "pageContentChanged"; payload: PageContentRow };

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

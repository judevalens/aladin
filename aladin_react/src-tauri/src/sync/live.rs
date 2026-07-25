use crate::db::{Db, DbResult};
use crate::events::DataEventHub;
use crate::realtime::{
    decode_frame_payload, BackendEventPayload, BackendEventSubscription, EventSubscriber,
    PayloadRegistration, ValidatedBackendEvent,
};
use crate::sync::{engine, SyncConfig};

// Data-layer R1-C (client) — the live websocket path.
// Architecture: ~/.claude/plans/data-layer-offline-readable.md (§4).
//
// The server CDC drain publishes each committed frame over the workspace
// realtime stream (event kind `*.frame`). This subscriber decodes the frame and
// applies it DIRECTLY into the cache via the SAME generic engine the pull uses,
// under the per-entity seq guard. The cursor is NOT advanced here (live is
// best-effort, may drop/reorder/dup); the recovery pull (on connect + heartbeat)
// advances it and heals anything the live stream missed. Idempotent: a live
// apply and a later pull overlap safely (the seq guard dedups).

/// The single live event kind the server publishes for data frames.
const FRAME_EVENT_KIND: &str = "*.frame";

pub struct WorkspaceLiveSubscriber;

impl EventSubscriber for WorkspaceLiveSubscriber {
    fn name(&self) -> &'static str {
        "workspace-live"
    }

    fn subscriptions(&self) -> Vec<BackendEventSubscription> {
        vec![BackendEventSubscription {
            event_kind: Some(FRAME_EVENT_KIND.to_string()),
            stream: "workspace".to_string(),
            resource_kind: "*".to_string(),
            resource_id: "*".to_string(),
            qualifiers: None,
        }]
    }

    fn payload_registrations(&self) -> Vec<PayloadRegistration> {
        vec![PayloadRegistration {
            event_kind: FRAME_EVENT_KIND,
            decoder: decode_frame_payload,
        }]
    }

    fn handle(
        &self,
        db: &Db,
        events: &DataEventHub,
        _config: &SyncConfig,
        event: &ValidatedBackendEvent,
    ) -> DbResult<()> {
        let BackendEventPayload::Frame(frame) = &event.payload;
        let registry = engine::Registry::tree();
        // Apply the frame atomically; NO cursor advance (live is best-effort). The
        // events to dispatch come straight from the repos (via the handlers).
        let emit = db.with_tx(|tx| engine::apply_frame(tx, &registry, frame))?;
        if crate::realtime::ws_debug_enabled() {
            eprintln!(
                "[ws] live-applied frame id={}: {} entities -> {} data event(s) emitted to UI",
                event.envelope.event_id,
                frame.entities.len(),
                emit.len()
            );
        }
        for event in emit {
            events.emit(event);
        }
        Ok(())
    }
}

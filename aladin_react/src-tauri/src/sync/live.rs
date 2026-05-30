use crate::db::repo::nodes::{self, ApplyOutcome};
use crate::db::{Db, DbResult};
use crate::events::{DataEvent, DataEventHub, EntityDeletedEvent};
use crate::realtime::{
    decode_sync_change_payload, BackendEventPayload, BackendEventSubscription, EventSubscriber,
    PayloadRegistration, ValidatedBackendEvent,
};
use crate::sync::SyncConfig;

// Data-layer redesign, Phase C — the live websocket path (to spec).
// Plan: ~/.claude/plans/data-layer-sync-model.md ("Live websocket carrying
// per-field changes").
//
// The server echoes each change row it appends to the feed over the workspace
// realtime stream. This subscriber decodes the carried change and applies it
// DIRECTLY into `nodes` via the same seq-guarded, idempotent apply the pull
// uses — no pull round-trip. The cursor is NOT advanced here; the recovery pull
// (on connect + heartbeat) advances it and heals anything the live stream
// dropped. Because apply is per-(entity,field) newest-wins, a live apply and a
// later recovery pull overlap safely.

pub struct WorkspaceLiveSubscriber;

const CHANGE_KINDS: &[&str] = &[
    "folder.created",
    "folder.updated",
    "folder.deleted",
    "artifact.created",
    "artifact.updated",
    "artifact.deleted",
];

impl EventSubscriber for WorkspaceLiveSubscriber {
    fn name(&self) -> &'static str {
        "workspace-live"
    }

    fn subscriptions(&self) -> Vec<BackendEventSubscription> {
        CHANGE_KINDS
            .iter()
            .map(|kind| BackendEventSubscription {
                event_kind: Some((*kind).to_string()),
                stream: "workspace".to_string(),
                resource_kind: "*".to_string(),
                resource_id: "*".to_string(),
                qualifiers: None,
            })
            .collect()
    }

    fn payload_registrations(&self) -> Vec<PayloadRegistration> {
        CHANGE_KINDS
            .iter()
            .map(|kind| PayloadRegistration {
                event_kind: kind,
                decoder: decode_sync_change_payload,
            })
            .collect()
    }

    fn handle(
        &self,
        db: &Db,
        events: &DataEventHub,
        _config: &SyncConfig,
        event: &ValidatedBackendEvent,
    ) -> DbResult<()> {
        let BackendEventPayload::SyncChange(change) = &event.payload else {
            return Ok(());
        };
        let emit = db.with_tx(|tx| {
            let outcome = nodes::apply_change(tx, change)?;
            Ok(match outcome {
                ApplyOutcome::Upserted(id) => nodes::get_node(tx, &id)?.map(DataEvent::NodeUpserted),
                ApplyOutcome::Deleted(id) => {
                    Some(DataEvent::NodeDeleted(EntityDeletedEvent { id }))
                }
                ApplyOutcome::Skipped => None,
            })
        })?;
        if let Some(event) = emit {
            events.emit(event);
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    // The client must parse the EXACT JSON the Go server publishes (repo.Change
    // with camelCase tags + raw-JSON value). This is the wire-compat contract.
    #[test]
    fn decodes_server_change_payload() {
        let payload = json!({
            "seq": 7,
            "entityKind": "folder",
            "entityId": "f1",
            "op": "update",
            "field": "title",
            "value": "Docs",
            "mutationId": "client-abc:3"
        });
        match decode_sync_change_payload(&payload).expect("decode") {
            BackendEventPayload::SyncChange(c) => {
                assert_eq!(c.seq, 7);
                assert_eq!(c.entity_kind, "folder");
                assert_eq!(c.entity_id, "f1");
                assert_eq!(c.op, "update");
                assert_eq!(c.field.as_deref(), Some("title"));
                assert_eq!(c.value, Some(json!("Docs")));
            }
            other => panic!("wrong variant: {other:?}"),
        }
    }

    #[test]
    fn rejects_change_missing_identity() {
        let payload = json!({ "seq": 1, "entityKind": "folder", "entityId": "", "op": "create" });
        assert!(decode_sync_change_payload(&payload).is_err());
    }
}

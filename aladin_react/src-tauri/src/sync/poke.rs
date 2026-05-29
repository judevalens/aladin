use crate::db::{Db, DbResult};
use crate::events::DataEventHub;
use crate::realtime::{
    BackendEventSubscription, EventSubscriber, PayloadRegistration, ValidatedBackendEvent,
};
use crate::sync::SyncConfig;

// Data-layer redesign — the realtime "doorbell" subscription.
// Plan: ~/.claude/plans/data-layer-sync-model.md.
//
// The new model treats a realtime event as a POKE, not a data carrier: the
// websocket loop calls pull_now on any received event, and the cursor-based
// pull is what actually applies + converges. But the loop only RECEIVES events
// whose kind the client subscribed to. This subscriber exists solely to
// subscribe the websocket to the workspace tree event kinds (so the server's
// post-push poke arrives); it applies nothing. Without it the tree would
// converge only on the poll. (It replaces the old BrowserEventSubscriber, which
// applied event payloads directly — the race-prone path the redesign removed.)

pub struct WorkspacePokeSubscriber;

const POKE_KINDS: &[&str] = &[
    "folder.created",
    "folder.updated",
    "folder.deleted",
    "artifact.created",
    "artifact.updated",
    "artifact.deleted",
];

impl EventSubscriber for WorkspacePokeSubscriber {
    fn name(&self) -> &'static str {
        "workspace-poke"
    }

    fn subscriptions(&self) -> Vec<BackendEventSubscription> {
        POKE_KINDS
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
        // No decoders: we don't apply payloads. The event is recorded as IGNORED
        // and the loop's pull_now does the real work.
        Vec::new()
    }

    fn handle(
        &self,
        _db: &Db,
        _events: &DataEventHub,
        _config: &SyncConfig,
        _event: &ValidatedBackendEvent,
    ) -> DbResult<()> {
        Ok(())
    }
}

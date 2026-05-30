use serde::Deserialize;
use tauri::{ipc::Channel, State};

use crate::db::{Db, DbResult};
use crate::events::{DataEvent, DataEventHub};
use crate::sync::{SyncConfig, SyncHandle};

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SyncSessionInput {
    pub api_base_url: String,
    pub token: Option<String>,
}

#[tauri::command]
pub fn sync_set_session(sync: State<'_, SyncHandle>, input: Option<SyncSessionInput>) {
    sync.set(input.map(|value| SyncConfig {
        api_base_url: value.api_base_url,
        token: value.token,
    }));
}

#[tauri::command]
pub fn sync_subscribe_data_events(events: State<'_, DataEventHub>, channel: Channel<DataEvent>) {
    events.subscribe(channel);
}

/// Pushes pending intents then pulls + applies the change-feed delta into
/// `nodes`, on demand. Returns the number of changes applied.
#[tauri::command]
pub fn sync_pull_now(
    db: State<'_, Db>,
    events: State<'_, DataEventHub>,
    sync: State<'_, SyncHandle>,
) -> DbResult<usize> {
    sync.pull_now(&db, &events)
}

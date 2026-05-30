use std::sync::{Arc, Mutex};

use serde::Serialize;
use tauri::ipc::Channel;

use crate::db::repo::nodes::NodeRow;

/// Data-layer redesign — the only workspace data events. The pull/live engines
/// apply changes into `nodes` and emit these; the UI reads `nodes` and patches
/// its tree by id. (Page content rides Yjs/Hocuspocus, a separate channel.)
#[derive(Debug, Clone, Serialize)]
#[serde(tag = "type", content = "payload", rename_all = "camelCase")]
pub enum DataEvent {
    NodeUpserted(NodeRow),
    NodeDeleted(EntityDeletedEvent),
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct EntityDeletedEvent {
    pub id: String,
}

#[derive(Default, Clone)]
pub struct DataEventHub {
    subscribers: Arc<Mutex<Vec<Channel<DataEvent>>>>,
}

impl DataEventHub {
    pub fn subscribe(&self, channel: Channel<DataEvent>) {
        if let Ok(mut subscribers) = self.subscribers.lock() {
            subscribers.push(channel);
        }
    }

    pub fn emit(&self, event: DataEvent) {
        if let Ok(mut subscribers) = self.subscribers.lock() {
            subscribers.retain(|channel| channel.send(event.clone()).is_ok());
        }
    }
}

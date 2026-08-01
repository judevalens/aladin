use std::sync::{Arc, Mutex};

use serde::Serialize;
use tauri::ipc::Channel;

use crate::db::repo::nodes::NodeRow;
use crate::db::repo::watchlists::WatchlistRow;

/// Data-layer redesign — workspace data events. The pull/live engines apply frame
/// changes into the local cache and emit these; the UI reads the cache and patches
/// by id. Node events drive the tree; watchlist events drive the Markets switcher.
/// (Page content rides Yjs/Hocuspocus, a separate channel.)
#[derive(Debug, Clone, Serialize)]
#[serde(tag = "type", content = "payload", rename_all = "camelCase")]
pub enum DataEvent {
    NodeUpserted(NodeRow),
    NodeDeleted(EntityDeletedEvent),
    WatchlistUpserted(WatchlistRow),
    WatchlistDeleted(EntityDeletedEvent),
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

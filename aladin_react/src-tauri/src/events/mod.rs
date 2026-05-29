use std::sync::{Arc, Mutex};

use serde::Serialize;
use tauri::ipc::Channel;

use crate::db::repo::{
    artifacts::ArtifactRow, browser::BrowserNodeRow, nodes::NodeRow, page_content::PageContentRow,
};

#[derive(Debug, Clone, Serialize)]
#[serde(tag = "type", content = "payload", rename_all = "camelCase")]
pub enum DataEvent {
    BrowserNodeCreated(BrowserNodeRow),
    BrowserNodeUpdated(BrowserNodeRow),
    BrowserNodeDeleted(EntityDeletedEvent),
    ArtifactChanged(ArtifactRow),
    ArtifactDeleted(EntityDeletedEvent),
    PageContentChanged(PageContentRow),
    // Data-layer redesign, Phase A — the unified `nodes` model. The pull engine
    // emits these after applying a feed delta; the workspace UI reads `nodes`
    // and patches its tree by id. NodeUpserted carries the full current row.
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

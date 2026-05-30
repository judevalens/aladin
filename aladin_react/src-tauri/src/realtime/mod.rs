use std::{
    collections::{HashMap, HashSet},
    io::ErrorKind,
    net::TcpStream,
    sync::{Arc, Mutex},
    thread,
    time::Duration,
};

use rusqlite::params;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use tungstenite::{
    client::IntoClientRequest,
    connect,
    http::{header::AUTHORIZATION, HeaderName, HeaderValue},
    stream::MaybeTlsStream,
    Message, WebSocket,
};

use crate::{
    api::sync::SyncChange,
    db::{Db, DbResult},
    events::DataEventHub,
    sync::{SyncConfig, SyncHandle},
};

const RECONNECT_BASE_SECS: u64 = 1;
const RECONNECT_MAX_SECS: u64 = 30;
const SUBSCRIPTION_POLL_SECS: u64 = 1;

#[derive(Debug, Clone, Deserialize, Serialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct BackendEventSubscription {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub event_kind: Option<String>,
    pub stream: String,
    pub resource_kind: String,
    pub resource_id: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub qualifiers: Option<HashMap<String, String>>,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct BackendEventEnvelope {
    pub event_id: String,
    pub kind: String,
    pub subscription_key: BackendEventSubscription,
    pub occurred_at: String,
}

#[derive(Debug, Clone)]
pub struct ValidatedBackendEvent {
    pub envelope: BackendEventEnvelope,
    pub payload: BackendEventPayload,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ArtifactSnapshotPayload {
    pub id: String,
    #[serde(rename = "type")]
    pub kind: String,
    pub folder_id: Option<String>,
    pub title: String,
    #[serde(default)]
    pub content: Option<String>,
    pub summary: Option<String>,
    pub source_url: Option<String>,
    pub resource_url: Option<String>,
    pub metadata: Option<Value>,
    pub updated_at: Option<String>,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TreeNodeSnapshotPayload {
    pub node: TreeNodePayload,
    pub artifact: Option<ArtifactSnapshotPayload>,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TreeNodePayload {
    pub id: String,
    pub parent_id: Option<String>,
    pub kind: String,
    pub title: String,
    pub artifact_id: Option<String>,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PageSnapshotPayload {
    pub id: String,
    pub title: String,
    /// Post-M5: the Go backend publishes BlockNote blocks JSON in this
    /// field. We round-trip it opaquely.
    #[serde(default)]
    pub blocks: Value,
    pub revision: i64,
    pub updated_at: Option<String>,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct EntityDeletedPayload {
    pub id: String,
}

#[derive(Debug, Clone)]
pub enum BackendEventPayload {
    TreeNodeSnapshot(TreeNodeSnapshotPayload),
    PageSnapshot(PageSnapshotPayload),
    EntityDeleted(EntityDeletedPayload),
    // Data-layer redesign, Phase C — a per-field change row carried live over the
    // websocket, applied directly into `nodes` via the same seq-guarded apply.
    SyncChange(SyncChange),
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct BackendEventWire {
    pub event_id: String,
    #[serde(rename = "type")]
    pub kind: String,
    pub subscription_key: BackendEventSubscription,
    pub payload: Value,
    pub occurred_at: String,
}

#[derive(Debug, Deserialize)]
struct RealtimeServerMessage {
    #[serde(rename = "type")]
    message_type: String,
    event: Option<BackendEventWire>,
    message: Option<String>,
}

#[derive(Debug, Serialize)]
struct RealtimeSubscribeMessage {
    #[serde(rename = "type")]
    message_type: &'static str,
    subscriptions: Vec<BackendEventSubscription>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum BackendEventStatus {
    Received,
    Dispatched,
    Invalid,
    Ignored,
    Failed,
}

impl BackendEventStatus {
    fn as_str(self) -> &'static str {
        match self {
            Self::Received => "RECEIVED",
            Self::Dispatched => "DISPATCHED",
            Self::Invalid => "INVALID",
            Self::Ignored => "IGNORED",
            Self::Failed => "FAILED",
        }
    }
}

pub type PayloadDecoder = fn(&Value) -> Result<BackendEventPayload, String>;

#[derive(Clone, Copy)]
pub struct PayloadRegistration {
    pub event_kind: &'static str,
    pub decoder: PayloadDecoder,
}

pub trait EventSubscriber: Send + Sync {
    fn name(&self) -> &'static str;
    fn subscriptions(&self) -> Vec<BackendEventSubscription>;
    fn payload_registrations(&self) -> Vec<PayloadRegistration>;
    fn handle(
        &self,
        db: &Db,
        events: &DataEventHub,
        config: &SyncConfig,
        event: &ValidatedBackendEvent,
    ) -> DbResult<()>;
}

#[derive(Clone, Default)]
pub struct BackendEventProcessor {
    subscribers: Arc<Mutex<Vec<Arc<dyn EventSubscriber>>>>,
    decoders: Arc<Mutex<HashMap<String, PayloadDecoder>>>,
}

impl BackendEventProcessor {
    pub fn register_subscriber(&self, subscriber: Arc<dyn EventSubscriber>) {
        if let Ok(mut decoders) = self.decoders.lock() {
            for registration in subscriber.payload_registrations() {
                decoders
                    .entry(registration.event_kind.to_string())
                    .or_insert(registration.decoder);
            }
        }
        if let Ok(mut subscribers) = self.subscribers.lock() {
            subscribers.push(subscriber);
        }
    }

    pub fn subscriptions(&self) -> Vec<BackendEventSubscription> {
        let subscribers = self
            .subscribers
            .lock()
            .map(|subscribers| subscribers.clone())
            .unwrap_or_default();
        let mut seen = HashSet::new();
        let mut out = Vec::new();
        for subscriber in subscribers {
            for subscription in subscriber.subscriptions() {
                let key = subscription_key_hash(&subscription);
                if seen.insert(key) {
                    out.push(subscription);
                }
            }
        }
        out
    }

    fn process_event(
        &self,
        db: &Db,
        events: &DataEventHub,
        config: &SyncConfig,
        event: BackendEventWire,
    ) -> DbResult<()> {
        if !insert_received_event(db, &event)? {
            return Ok(());
        }

        let decoder = self
            .decoders
            .lock()
            .ok()
            .and_then(|decoders| decoders.get(&event.kind).copied());
        let Some(decoder) = decoder else {
            update_event_status(db, &event.event_id, BackendEventStatus::Ignored, None)?;
            return Ok(());
        };

        let payload = match decoder(&event.payload) {
            Ok(payload) => payload,
            Err(error) => {
                update_event_status(
                    db,
                    &event.event_id,
                    BackendEventStatus::Invalid,
                    Some(&error),
                )?;
                return Ok(());
            }
        };

        let validated = ValidatedBackendEvent {
            envelope: BackendEventEnvelope {
                event_id: event.event_id.clone(),
                kind: event.kind.clone(),
                subscription_key: event.subscription_key.clone(),
                occurred_at: event.occurred_at,
            },
            payload,
        };

        let subscribers = self
            .subscribers
            .lock()
            .map_err(|_| crate::db::DbError::NotInitialized)?
            .clone();
        let mut handled = false;
        for subscriber in subscribers {
            if !subscriber_handles(&subscriber, &validated.envelope) {
                continue;
            }
            handled = true;
            if let Err(error) = subscriber.handle(db, events, config, &validated) {
                let message = format!("{}: {error}", subscriber.name());
                update_event_status(
                    db,
                    &event.event_id,
                    BackendEventStatus::Failed,
                    Some(&message),
                )?;
                return Err(error);
            }
        }

        update_event_status(
            db,
            &event.event_id,
            if handled {
                BackendEventStatus::Dispatched
            } else {
                BackendEventStatus::Ignored
            },
            None,
        )
    }
}

pub fn run_websocket_loop(sync: SyncHandle, db: Db, events: DataEventHub) {
    let mut last_event_id: Option<String> = None;
    let mut backoff_secs = RECONNECT_BASE_SECS;
    loop {
        let Some(config) = sync.get() else {
            thread::sleep(Duration::from_secs(1));
            continue;
        };
        if !sync_config_ready(&config) {
            thread::sleep(Duration::from_secs(1));
            continue;
        }

        match connect_once(&sync, &db, &events, &config, &mut last_event_id) {
            Ok(()) => backoff_secs = RECONNECT_BASE_SECS,
            Err(error) => {
                eprintln!("backend websocket failed: {error}");
                thread::sleep(Duration::from_secs(backoff_secs));
                backoff_secs = (backoff_secs * 2).min(RECONNECT_MAX_SECS);
            }
        }
    }
}

fn connect_once(
    sync: &SyncHandle,
    db: &Db,
    events: &DataEventHub,
    config: &SyncConfig,
    last_event_id: &mut Option<String>,
) -> Result<(), String> {
    let mut request = websocket_url(&config.api_base_url)
        .into_client_request()
        .map_err(|error| error.to_string())?;
    if let Some(token) = config
        .token
        .as_ref()
        .filter(|token| !token.trim().is_empty())
    {
        let value =
            HeaderValue::from_str(&format!("Bearer {token}")).map_err(|error| error.to_string())?;
        request.headers_mut().insert(AUTHORIZATION, value);
    }
    if let Some(last_event_id) = last_event_id.as_ref() {
        let value = HeaderValue::from_str(last_event_id).map_err(|error| error.to_string())?;
        request
            .headers_mut()
            .insert(HeaderName::from_static("last-event-id"), value);
    }

    let (mut socket, _) = connect(request).map_err(|error| error.to_string())?;
    set_socket_read_timeout(&mut socket, Duration::from_secs(SUBSCRIPTION_POLL_SECS))?;
    let mut subscription_version = sync.subscription_version();
    send_subscribe(&mut socket, sync)?;
    // Recovery pull on (re)connect: catch up anything missed while the socket
    // was down (the cursor advances here, not on live events).
    let _ = sync.pull_now(db, events);

    loop {
        let message = match socket.read() {
            Ok(message) => message,
            Err(tungstenite::Error::Io(error))
                if matches!(error.kind(), ErrorKind::WouldBlock | ErrorKind::TimedOut) =>
            {
                maybe_send_subscription_update(&mut socket, sync, &mut subscription_version)?;
                continue;
            }
            Err(error) => return Err(error.to_string()),
        };
        let Message::Text(text) = message else {
            maybe_send_subscription_update(&mut socket, sync, &mut subscription_version)?;
            continue;
        };
        let parsed: RealtimeServerMessage =
            serde_json::from_str(&text).map_err(|error| error.to_string())?;
        match parsed.message_type.as_str() {
            "event" => {
                if let Some(event) = parsed.event {
                    let event_id = event.event_id.clone();
                    // Data-layer redesign, Phase C — the live change row is carried
                    // in the event payload and applied DIRECTLY here (via the
                    // WorkspaceLiveSubscriber → seq-guarded apply). No pull is
                    // triggered per event; the cursor advances only on the
                    // recovery pull (on connect + heartbeat).
                    sync.event_processor()
                        .process_event(db, events, config, event)
                        .map_err(|error| error.to_string())?;
                    *last_event_id = Some(event_id);
                }
            }
            "error" => {
                return Err(parsed
                    .message
                    .unwrap_or_else(|| "backend websocket error".to_string()));
            }
            _ => {}
        }
        maybe_send_subscription_update(&mut socket, sync, &mut subscription_version)?;
    }
}

fn maybe_send_subscription_update(
    socket: &mut WebSocket<MaybeTlsStream<TcpStream>>,
    sync: &SyncHandle,
    subscription_version: &mut u64,
) -> Result<(), String> {
    let current_version = sync.subscription_version();
    if current_version == *subscription_version {
        return Ok(());
    }
    send_subscribe(socket, sync)?;
    *subscription_version = current_version;
    Ok(())
}

fn send_subscribe(
    socket: &mut WebSocket<MaybeTlsStream<TcpStream>>,
    sync: &SyncHandle,
) -> Result<(), String> {
    let subscriptions = sync.event_processor().subscriptions();
    socket
        .send(Message::Text(
            serde_json::to_string(&RealtimeSubscribeMessage {
                message_type: "subscribe",
                subscriptions,
            })
            .map_err(|error| error.to_string())?,
        ))
        .map_err(|error| error.to_string())
}

fn set_socket_read_timeout(
    socket: &mut WebSocket<MaybeTlsStream<TcpStream>>,
    duration: Duration,
) -> Result<(), String> {
    match socket.get_mut() {
        MaybeTlsStream::Plain(stream) => stream.set_read_timeout(Some(duration)),
        MaybeTlsStream::Rustls(stream) => stream.sock.set_read_timeout(Some(duration)),
        _ => Ok(()),
    }
    .map_err(|error| error.to_string())
}

fn websocket_url(api_base_url: &str) -> String {
    let base = api_base_url.trim_end_matches('/');
    if let Some(rest) = base.strip_prefix("https://") {
        format!("wss://{rest}/api/events/ws")
    } else if let Some(rest) = base.strip_prefix("http://") {
        format!("ws://{rest}/api/events/ws")
    } else {
        format!("{base}/api/events/ws")
    }
}

fn sync_config_ready(config: &SyncConfig) -> bool {
    !config.api_base_url.trim().is_empty() && config.token.as_deref().unwrap_or("").trim() != ""
}

pub fn decode_tree_node_snapshot_payload(value: &Value) -> Result<BackendEventPayload, String> {
    let payload: TreeNodeSnapshotPayload =
        serde_json::from_value(value.clone()).map_err(|error| error.to_string())?;
    validate_payload_id(&payload.node.id)?;
    validate_non_empty(&payload.node.kind, "tree node kind is required")?;
    validate_non_empty(&payload.node.title, "tree node title is required")?;
    if let Some(artifact) = payload.artifact.as_ref() {
        validate_payload_id(&artifact.id)?;
        validate_non_empty(&artifact.kind, "artifact type is required")?;
        validate_non_empty(&artifact.title, "artifact title is required")?;
    }
    Ok(BackendEventPayload::TreeNodeSnapshot(payload))
}

pub fn decode_page_snapshot_payload(value: &Value) -> Result<BackendEventPayload, String> {
    let payload: PageSnapshotPayload =
        serde_json::from_value(value.clone()).map_err(|error| error.to_string())?;
    validate_payload_id(&payload.id)?;
    validate_non_empty(&payload.title, "page title is required")?;
    Ok(BackendEventPayload::PageSnapshot(payload))
}

pub fn decode_entity_deleted_payload(value: &Value) -> Result<BackendEventPayload, String> {
    let payload: EntityDeletedPayload =
        serde_json::from_value(value.clone()).map_err(|error| error.to_string())?;
    validate_payload_id(&payload.id)?;
    Ok(BackendEventPayload::EntityDeleted(payload))
}

/// Decodes a per-field change row carried live over the websocket (the server
/// echoes the same rows it appended to the feed). Applied directly via the
/// seq-guarded apply; the cursor is untouched (pull advances it on recovery).
pub fn decode_sync_change_payload(value: &Value) -> Result<BackendEventPayload, String> {
    let change: SyncChange =
        serde_json::from_value(value.clone()).map_err(|error| error.to_string())?;
    validate_payload_id(&change.entity_id)?;
    validate_non_empty(&change.entity_kind, "change entity kind is required")?;
    validate_non_empty(&change.op, "change op is required")?;
    Ok(BackendEventPayload::SyncChange(change))
}

fn validate_payload_id(id: &str) -> Result<(), String> {
    validate_non_empty(id, "payload id is required")
}

fn validate_non_empty(value: &str, message: &str) -> Result<(), String> {
    if value.trim().is_empty() {
        Err(message.to_string())
    } else {
        Ok(())
    }
}

fn subscriber_handles(
    subscriber: &Arc<dyn EventSubscriber>,
    envelope: &BackendEventEnvelope,
) -> bool {
    subscriber
        .subscriptions()
        .iter()
        .any(|subscription| subscription_matches(subscription, &envelope.subscription_key))
}

fn subscription_matches(
    subscription: &BackendEventSubscription,
    published: &BackendEventSubscription,
) -> bool {
    if subscription.stream != published.stream {
        return false;
    }
    if let Some(want) = subscription.event_kind.as_deref() {
        match published.event_kind.as_deref() {
            Some(have) if want == have => {}
            _ => return false,
        }
    }
    if subscription.resource_kind != "*" && subscription.resource_kind != published.resource_kind {
        return false;
    }
    if subscription.resource_id != "*" && subscription.resource_id != published.resource_id {
        return false;
    }
    normalize_qualifiers(subscription.qualifiers.as_ref())
        == normalize_qualifiers(published.qualifiers.as_ref())
}

fn normalize_qualifiers(value: Option<&HashMap<String, String>>) -> String {
    let Some(value) = value else {
        return String::new();
    };
    let mut entries: Vec<_> = value.iter().collect();
    entries.sort_by(|left, right| left.0.cmp(right.0));
    entries
        .into_iter()
        .map(|(key, value)| format!("{key}={value}"))
        .collect::<Vec<_>>()
        .join("&")
}

fn subscription_key_hash(subscription: &BackendEventSubscription) -> String {
    format!(
        "{}|{}|{}|{}|{}",
        subscription.event_kind.as_deref().unwrap_or(""),
        subscription.stream,
        subscription.resource_kind,
        subscription.resource_id,
        normalize_qualifiers(subscription.qualifiers.as_ref())
    )
}

fn insert_received_event(db: &Db, event: &BackendEventWire) -> DbResult<bool> {
    let subscription_key_json = serde_json::to_string(&event.subscription_key)?;
    let payload_json = event.payload.to_string();
    let now = current_time_ms();
    db.with_tx(|tx| {
        let changed = tx.execute(
            "
            INSERT OR IGNORE INTO backend_events (
                id,
                event_kind,
                subscription_key_json,
                payload_json,
                status,
                validation_error,
                occurred_at,
                received_at,
                updated_at
            ) VALUES (?1, ?2, ?3, ?4, ?5, NULL, ?6, ?7, ?7)
            ",
            params![
                event.event_id,
                event.kind,
                subscription_key_json,
                payload_json,
                BackendEventStatus::Received.as_str(),
                event.occurred_at,
                now
            ],
        )?;
        Ok(changed > 0)
    })
}

fn update_event_status(
    db: &Db,
    event_id: &str,
    status: BackendEventStatus,
    validation_error: Option<&str>,
) -> DbResult<()> {
    let now = current_time_ms();
    db.with_tx(|tx| {
        tx.execute(
            "
            UPDATE backend_events
            SET status = ?1, validation_error = ?2, updated_at = ?3
            WHERE id = ?4
            ",
            params![status.as_str(), validation_error, now, event_id],
        )?;
        Ok(())
    })
}

fn current_time_ms() -> i64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|duration| duration.as_millis() as i64)
        .unwrap_or(0)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::atomic::{AtomicUsize, Ordering};

    struct FakeSubscriber {
        handled: Arc<AtomicUsize>,
    }

    impl EventSubscriber for FakeSubscriber {
        fn name(&self) -> &'static str {
            "fake"
        }

        fn subscriptions(&self) -> Vec<BackendEventSubscription> {
            vec![BackendEventSubscription {
                event_kind: Some("artifact.updated".to_string()),
                stream: "workspace".to_string(),
                resource_kind: "*".to_string(),
                resource_id: "*".to_string(),
                qualifiers: None,
            }]
        }

        fn payload_registrations(&self) -> Vec<PayloadRegistration> {
            vec![PayloadRegistration {
                event_kind: "artifact.updated",
                decoder: decode_tree_node_snapshot_payload,
            }]
        }

        fn handle(
            &self,
            _db: &Db,
            _events: &DataEventHub,
            _config: &SyncConfig,
            event: &ValidatedBackendEvent,
        ) -> DbResult<()> {
            match &event.payload {
                BackendEventPayload::TreeNodeSnapshot(payload) => {
                    if payload.node.id == "artifact-1" && payload.node.title == "Updated memo" {
                        self.handled.fetch_add(1, Ordering::SeqCst);
                    }
                }
                BackendEventPayload::EntityDeleted(_) => {}
                _ => {}
            }
            Ok(())
        }
    }

    #[test]
    fn dispatches_valid_registered_event() {
        let db = test_db("valid_registered_event");
        let events = DataEventHub::default();
        let processor = BackendEventProcessor::default();
        let handled = Arc::new(AtomicUsize::new(0));
        processor.register_subscriber(Arc::new(FakeSubscriber {
            handled: handled.clone(),
        }));

        processor
            .process_event(
                &db,
                &events,
                &test_config(),
                test_event(
                    "evt-valid",
                    "artifact.updated",
                    serde_json::json!({
                        "node": {
                            "id": "artifact-1",
                            "parentId": "folder-1",
                            "kind": "artifact",
                            "title": "Updated memo",
                            "artifactId": "artifact-1"
                        },
                        "artifact": {
                            "id": "artifact-1",
                            "type": "page",
                            "folderId": "folder-1",
                            "title": "Updated memo",
                            "content": "Body",
                            "summary": "Short",
                            "sourceUrl": null,
                            "metadata": {},
                            "updatedAt": "2026-05-24T00:00:00Z"
                        }
                    }),
                ),
            )
            .expect("process event");

        assert_eq!(handled.load(Ordering::SeqCst), 1);
        assert_eq!(event_status(&db, "evt-valid"), "DISPATCHED");
    }

    #[test]
    fn records_invalid_payload_without_dispatching() {
        let db = test_db("invalid_payload");
        let events = DataEventHub::default();
        let processor = BackendEventProcessor::default();
        let handled = Arc::new(AtomicUsize::new(0));
        processor.register_subscriber(Arc::new(FakeSubscriber {
            handled: handled.clone(),
        }));

        processor
            .process_event(
                &db,
                &events,
                &test_config(),
                test_event("evt-invalid", "artifact.updated", serde_json::json!({})),
            )
            .expect("process event");

        assert_eq!(handled.load(Ordering::SeqCst), 0);
        assert_eq!(event_status(&db, "evt-invalid"), "INVALID");
    }

    #[test]
    fn records_unknown_event_as_ignored() {
        let db = test_db("unknown_event");
        let events = DataEventHub::default();
        let processor = BackendEventProcessor::default();

        processor
            .process_event(
                &db,
                &events,
                &test_config(),
                test_event(
                    "evt-unknown",
                    "source.updated",
                    serde_json::json!({"id":"source-1"}),
                ),
            )
            .expect("process event");

        assert_eq!(event_status(&db, "evt-unknown"), "IGNORED");
    }

    fn test_event(event_id: &str, kind: &str, payload: Value) -> BackendEventWire {
        BackendEventWire {
            event_id: event_id.to_string(),
            kind: kind.to_string(),
            subscription_key: BackendEventSubscription {
                event_kind: Some(kind.to_string()),
                stream: "workspace".to_string(),
                resource_kind: "artifact".to_string(),
                resource_id: "artifact-1".to_string(),
                qualifiers: None,
            },
            payload,
            occurred_at: "2026-05-24T00:00:00Z".to_string(),
        }
    }

    fn test_config() -> SyncConfig {
        SyncConfig {
            api_base_url: "http://localhost:8000".to_string(),
            token: Some("token".to_string()),
        }
    }

    fn test_db(name: &str) -> Db {
        let mut path = std::env::temp_dir();
        path.push(format!(
            "aladin_realtime_test_{name}_{}.sqlite",
            current_time_ms()
        ));
        Db::open(path).expect("open db")
    }

    fn event_status(db: &Db, event_id: &str) -> String {
        db.with_conn(|conn| {
            conn.query_row(
                "SELECT status FROM backend_events WHERE id = ?1",
                params![event_id],
                |row| row.get(0),
            )
            .map_err(crate::db::DbError::Sqlite)
        })
        .expect("event status")
    }
}

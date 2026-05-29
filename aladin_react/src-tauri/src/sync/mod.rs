use std::{
    collections::HashMap,
    sync::{
        atomic::{AtomicBool, AtomicU64, Ordering},
        Arc, Condvar, Mutex,
    },
    thread,
    time::Duration,
};

use crate::{
    db::{
        repo::outbox::{self, OutboxDao, SqliteOutboxDao, MAX_OUTBOX_ATTEMPTS},
        Db, DbResult,
    },
    events::DataEventHub,
    realtime::{self, BackendEventProcessor, EventSubscriber},
};

pub mod poke;
pub mod pull;
pub mod push;

/// How often the read/convergence loop pulls the change-feed delta. The
/// realtime websocket is the fast path (any event pokes an immediate pull — see
/// realtime::run_websocket_loop); this poll is the guaranteed fallback when a
/// poke is missed or the socket is down.
const PULL_INTERVAL_SECS: u64 = 2;

#[derive(Debug, Clone)]
pub struct SyncConfig {
    pub api_base_url: String,
    pub token: Option<String>,
}

pub trait OutboxProcessor: Send + Sync {
    fn entity_kind(&self) -> &'static str;
    fn process(
        &self,
        db: &Db,
        events: &DataEventHub,
        config: &SyncConfig,
        entry: &outbox::OutboxMutationRow,
    ) -> DbResult<()>;

    fn mark_failed(
        &self,
        tx: &rusqlite::Transaction<'_>,
        entry: &outbox::OutboxMutationRow,
    ) -> DbResult<()>;
}

#[derive(Clone)]
pub struct SyncHandle {
    inner: Arc<SyncRuntimeState>,
}

struct SyncRuntimeState {
    config: Mutex<Option<SyncConfig>>,
    processors: Mutex<HashMap<String, Arc<dyn OutboxProcessor>>>,
    event_processor: BackendEventProcessor,
    subscription_version: AtomicU64,
    outbox: Arc<dyn OutboxDao>,
    started: AtomicBool,
    realtime_started: AtomicBool,
    pull_started: AtomicBool,
    // Wakes the pull/push loop early when a local write enqueues an intent, so
    // changes flush immediately instead of waiting for the next poll tick. The
    // bool is a "work pending" flag so a nudge during a push isn't lost.
    wake: (Mutex<bool>, Condvar),
}

impl Default for SyncHandle {
    fn default() -> Self {
        Self {
            inner: Arc::new(SyncRuntimeState::default()),
        }
    }
}

impl Default for SyncRuntimeState {
    fn default() -> Self {
        Self {
            config: Mutex::new(None),
            processors: Mutex::new(HashMap::new()),
            event_processor: BackendEventProcessor::default(),
            subscription_version: AtomicU64::new(0),
            outbox: Arc::new(SqliteOutboxDao),
            started: AtomicBool::new(false),
            realtime_started: AtomicBool::new(false),
            pull_started: AtomicBool::new(false),
            wake: (Mutex::new(false), Condvar::new()),
        }
    }
}

impl SyncHandle {
    pub fn set(&self, config: Option<SyncConfig>) {
        if let Ok(mut guard) = self.inner.config.lock() {
            *guard = config;
        }
    }

    pub fn get(&self) -> Option<SyncConfig> {
        self.inner
            .config
            .lock()
            .ok()
            .and_then(|guard| guard.clone())
    }

    pub fn register_processor(&self, processor: Arc<dyn OutboxProcessor>) {
        if let Ok(mut processors) = self.inner.processors.lock() {
            processors.insert(processor.entity_kind().to_string(), processor);
        }
    }

    pub fn register_event_subscriber(&self, subscriber: Arc<dyn EventSubscriber>) {
        self.inner.event_processor.register_subscriber(subscriber);
        self.inner
            .subscription_version
            .fetch_add(1, Ordering::SeqCst);
    }

    pub fn event_processor(&self) -> BackendEventProcessor {
        self.inner.event_processor.clone()
    }

    pub fn subscription_version(&self) -> u64 {
        self.inner.subscription_version.load(Ordering::SeqCst)
    }

    pub fn start_polling(&self, db: Db, events: DataEventHub) {
        if self.inner.started.swap(true, Ordering::SeqCst) {
            return;
        }
        let sync = self.clone();
        thread::spawn(move || loop {
            if let Err(error) = sync.drain_once(&db, &events) {
                eprintln!("sync poll failed: {error}");
            }
            thread::sleep(Duration::from_secs(60));
        });
    }

    pub fn start_realtime(&self, db: Db, events: DataEventHub) {
        if self.inner.realtime_started.swap(true, Ordering::SeqCst) {
            return;
        }
        let sync = self.clone();
        thread::spawn(move || realtime::run_websocket_loop(sync, db, events));
    }

    /// Spawns the read/convergence loop: pulls the change-feed delta and applies
    /// it into `nodes` on boot and every PULL_INTERVAL_SECS thereafter. This is
    /// the mechanism that makes a client converge to server state (Phase A).
    pub fn start_pull_polling(&self, db: Db, events: DataEventHub) {
        if self.inner.pull_started.swap(true, Ordering::SeqCst) {
            return;
        }
        let sync = self.clone();
        thread::spawn(move || {
            let api = crate::api::sync::SyncApi;
            loop {
                if let Some(config) = sync.get() {
                    // Push local intents first (so the server has them), then pull
                    // the converged delta (including our own echoes, idempotent).
                    if let Err(error) = push::push_pending(&db, &events, &config, &api) {
                        eprintln!("sync push failed: {error}");
                    }
                    if let Err(error) = pull::pull_and_apply(&db, &events, &config, &api) {
                        eprintln!("sync pull failed: {error}");
                    }
                }
                // Wait until a local write nudges us, or the poll interval elapses
                // (the guaranteed fallback). A nudge that arrives mid-cycle sets
                // the flag, so the next wait returns immediately — no lost wakeup.
                sync.wait_for_nudge(PULL_INTERVAL_SECS);
            }
        });
    }

    /// Wakes the sync loop now (called right after a local write enqueues an
    /// intent) so it flushes without waiting for the poll tick.
    pub fn nudge(&self) {
        let (lock, cv) = &self.inner.wake;
        if let Ok(mut pending) = lock.lock() {
            *pending = true;
            cv.notify_one();
        }
    }

    fn wait_for_nudge(&self, secs: u64) {
        let (lock, cv) = &self.inner.wake;
        let mut pending = match lock.lock() {
            Ok(guard) => guard,
            Err(poisoned) => poisoned.into_inner(),
        };
        if !*pending {
            pending = match cv.wait_timeout(pending, Duration::from_secs(secs)) {
                Ok((guard, _)) => guard,
                Err(poisoned) => poisoned.into_inner().0,
            };
        }
        *pending = false;
    }

    /// Runs one sync tick on demand (push pending intents, then pull + apply the
    /// delta). Returns the number of changes applied; a no-op when there is no
    /// configured session.
    pub fn pull_now(&self, db: &Db, events: &DataEventHub) -> DbResult<usize> {
        let Some(config) = self.get() else {
            return Ok(0);
        };
        let api = crate::api::sync::SyncApi;
        if let Err(error) = push::push_pending(db, events, &config, &api) {
            eprintln!("sync push failed: {error}");
        }
        pull::pull_and_apply(db, events, &config, &api)
    }

    pub fn drain_once(&self, db: &Db, events: &DataEventHub) -> DbResult<usize> {
        let Some(config) = self.get() else {
            return Ok(0);
        };
        if config.api_base_url.trim().is_empty() || config.token.as_deref().unwrap_or("").is_empty()
        {
            return Ok(0);
        }

        let pending = db.with_conn(|c| self.inner.outbox.list_pending(c, 50))?;
        if pending.is_empty() {
            return Ok(0);
        }

        let processors = self
            .inner
            .processors
            .lock()
            .map_err(|_| crate::db::DbError::NotInitialized)?
            .clone();

        let mut processed = 0usize;
        for entry in pending {
            let Some(processor) = processors.get(&entry.entity_kind).cloned() else {
                continue;
            };
            match processor.process(db, events, &config, &entry) {
                Ok(()) => {
                    processed += 1;
                }
                Err(error) => {
                    let next_attempts = entry.attempts + 1;
                    let should_mark_failed = match &error {
                        crate::db::DbError::Api(api) => matches!(
                            api.kind(),
                            crate::api::ApiErrorKind::Validation
                                | crate::api::ApiErrorKind::Conflict
                                | crate::api::ApiErrorKind::NotFound
                        ),
                        _ => false,
                    } || next_attempts >= MAX_OUTBOX_ATTEMPTS;
                    let error_message = error.to_string();
                    let updated_at = current_time_ms();
                    db.with_tx(|tx| {
                        self.inner.outbox.record_retry(
                            tx,
                            &entry.id,
                            &error_message,
                            updated_at,
                        )?;
                        if should_mark_failed {
                            self.inner.outbox.mark_failed(
                                tx,
                                &entry.id,
                                &error_message,
                                updated_at,
                            )?;
                            processor.mark_failed(tx, &entry)?;
                        }
                        Ok(())
                    })?;
                }
            }
        }

        Ok(processed)
    }
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
    use crate::realtime::{
        decode_entity_deleted_payload, BackendEventSubscription, PayloadRegistration,
        ValidatedBackendEvent,
    };

    struct TestEventSubscriber {
        resource_kind: &'static str,
        resource_id: &'static str,
    }

    impl EventSubscriber for TestEventSubscriber {
        fn name(&self) -> &'static str {
            "test"
        }

        fn subscriptions(&self) -> Vec<BackendEventSubscription> {
            vec![BackendEventSubscription {
                event_kind: Some("artifact.deleted".to_string()),
                stream: "workspace".to_string(),
                resource_kind: self.resource_kind.to_string(),
                resource_id: self.resource_id.to_string(),
                qualifiers: None,
            }]
        }

        fn payload_registrations(&self) -> Vec<PayloadRegistration> {
            vec![PayloadRegistration {
                event_kind: "artifact.deleted",
                decoder: decode_entity_deleted_payload,
            }]
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

    #[test]
    fn register_event_subscriber_bumps_subscription_version_and_updates_subscriptions() {
        let sync = SyncHandle::default();
        assert_eq!(sync.subscription_version(), 0);
        assert!(sync.event_processor().subscriptions().is_empty());

        sync.register_event_subscriber(Arc::new(TestEventSubscriber {
            resource_kind: "artifact",
            resource_id: "artifact-1",
        }));

        assert_eq!(sync.subscription_version(), 1);
        let subscriptions = sync.event_processor().subscriptions();
        assert_eq!(subscriptions.len(), 1);
        assert_eq!(subscriptions[0].resource_kind, "artifact");
        assert_eq!(subscriptions[0].resource_id, "artifact-1");

        sync.register_event_subscriber(Arc::new(TestEventSubscriber {
            resource_kind: "folder",
            resource_id: "folder-1",
        }));

        assert_eq!(sync.subscription_version(), 2);
        let subscriptions = sync.event_processor().subscriptions();
        assert_eq!(subscriptions.len(), 2);
    }
}

use std::{
    sync::{
        atomic::{AtomicBool, AtomicU64, Ordering},
        Arc, Mutex,
    },
    thread,
    time::Duration,
};

use crate::{
    db::{Db, DbResult},
    events::DataEventHub,
    realtime::{self, BackendEventProcessor, EventSubscriber},
};

pub mod engine;
pub mod live;
pub mod pull;

/// Recovery-pull cadence (doc §6: periodic time-based trigger). Steady state is
/// the live websocket (committed frames applied directly); this heartbeat heals
/// what the live stream missed (a dropped frame while connected) and bounds
/// cursor staleness in wall-clock time. Reconnect catch-up is a separate pull on
/// ws connect. There is no local-write trigger — writes proxy to the server and
/// return as frames.
const PULL_INTERVAL_SECS: u64 = 20;

#[derive(Debug, Clone)]
pub struct SyncConfig {
    pub api_base_url: String,
    pub token: Option<String>,
}

#[derive(Clone)]
pub struct SyncHandle {
    inner: Arc<SyncRuntimeState>,
}

struct SyncRuntimeState {
    config: Mutex<Option<SyncConfig>>,
    event_processor: BackendEventProcessor,
    subscription_version: AtomicU64,
    realtime_started: AtomicBool,
    pull_started: AtomicBool,
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
            event_processor: BackendEventProcessor::default(),
            subscription_version: AtomicU64::new(0),
            realtime_started: AtomicBool::new(false),
            pull_started: AtomicBool::new(false),
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

    pub fn start_realtime(&self, db: Db, events: DataEventHub) {
        if self.inner.realtime_started.swap(true, Ordering::SeqCst) {
            return;
        }
        let sync = self.clone();
        thread::spawn(move || realtime::run_websocket_loop(sync, db, events));
    }

    /// Spawns the read/convergence loop: push pending intents, then pull + apply
    /// the change-feed delta. Runs on boot, on a nudge (local write), and every
    /// PULL_INTERVAL_SECS as a recovery heartbeat. Steady-state convergence is
    /// the live websocket; this loop is the send path + the recovery pull.
    pub fn start_pull_polling(&self, db: Db, events: DataEventHub) {
        if self.inner.pull_started.swap(true, Ordering::SeqCst) {
            return;
        }
        let sync = self.clone();
        thread::spawn(move || {
            let api = crate::api::sync::SyncApi;
            loop {
                if let Some(config) = sync.get() {
                    if let Err(error) = pull::pull_and_apply(&db, &events, &config, &api) {
                        eprintln!("sync pull failed: {error}");
                    }
                }
                // Periodic recovery heartbeat (doc §6). Live + reconnect-pull are
                // the fast paths; this bounds cursor staleness in wall-clock time.
                thread::sleep(Duration::from_secs(PULL_INTERVAL_SECS));
            }
        });
    }

    /// Runs one pull tick on demand (pull + apply frames since the cursor).
    /// Returns the number of entities applied; a no-op when there is no
    /// configured session. Used on ws (re)connect (recovery) + by callers.
    pub fn pull_now(&self, db: &Db, events: &DataEventHub) -> DbResult<usize> {
        let Some(config) = self.get() else {
            return Ok(0);
        };
        let api = crate::api::sync::SyncApi;
        pull::pull_and_apply(db, events, &config, &api)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::realtime::{
        decode_frame_payload, BackendEventSubscription, PayloadRegistration, ValidatedBackendEvent,
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
                decoder: decode_frame_payload,
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

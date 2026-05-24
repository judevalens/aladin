use std::{
    collections::HashMap,
    sync::{
        atomic::{AtomicBool, Ordering},
        Arc, Mutex,
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
};

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
pub struct SyncState {
    inner: Arc<SyncStateInner>,
}

struct SyncStateInner {
    config: Mutex<Option<SyncConfig>>,
    processors: Mutex<HashMap<String, Arc<dyn OutboxProcessor>>>,
    outbox: Arc<dyn OutboxDao>,
    started: AtomicBool,
}

impl Default for SyncState {
    fn default() -> Self {
        Self {
            inner: Arc::new(SyncStateInner::default()),
        }
    }
}

impl Default for SyncStateInner {
    fn default() -> Self {
        Self {
            config: Mutex::new(None),
            processors: Mutex::new(HashMap::new()),
            outbox: Arc::new(SqliteOutboxDao),
            started: AtomicBool::new(false),
        }
    }
}

impl SyncState {
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

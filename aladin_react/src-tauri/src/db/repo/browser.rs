use rusqlite::{params, Connection};
use serde::{Deserialize, Serialize};
use std::sync::Arc;

use crate::{
    api::browser::{BrowserApi, BrowserApiClient, BrowserTreeApiRecord},
    db::{
        repo::{
            artifacts::{ArtifactDao, ArtifactRow, SqliteArtifactDao},
            outbox::{self, OutboxDao, SqliteOutboxDao},
            MutationMode, SyncStatus,
        },
        Db, DbResult,
    },
    events::{DataEvent, DataEventHub, EntityDeletedEvent},
    realtime::{
        decode_entity_deleted_payload, decode_page_snapshot_payload,
        decode_tree_node_snapshot_payload, ArtifactSnapshotPayload, BackendEventPayload,
        BackendEventSubscription, EventSubscriber, PageSnapshotPayload, PayloadRegistration,
        TreeNodePayload, ValidatedBackendEvent,
    },
    sync::{OutboxProcessor, SyncConfig},
};

#[derive(Debug, Serialize, Deserialize, Clone)]
#[serde(rename_all = "camelCase")]
pub struct BrowserNodeRow {
    pub id: String,
    pub parent_id: Option<String>,
    pub title: String,
    pub kind: String,
    pub artifact_id: Option<String>,
    pub updated_at: i64,
    pub sync_status: SyncStatus,
    pub version: i64,
}

#[derive(Debug, Deserialize, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct BrowserMutationInput {
    pub id: String,
    pub parent_id: Option<String>,
    pub title: String,
    pub updated_at: i64,
    pub mutation_id: String,
}

#[derive(Debug, Deserialize, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct BrowserNodeCreateInput {
    pub id: String,
    pub parent_id: Option<String>,
    pub kind: String,
    pub title: String,
    pub artifact_type: Option<String>,
    pub content: Option<String>,
    pub summary: Option<String>,
    pub source_url: Option<String>,
    pub updated_at: i64,
    pub mutation_id: String,
}

#[derive(Debug, Deserialize, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct BrowserDeleteInput {
    pub id: String,
    pub updated_at: i64,
    pub mutation_id: String,
}

pub struct BrowserRepo {
    api: Arc<dyn BrowserApiClient>,
    dao: Arc<dyn BrowserDao>,
    artifact_dao: Arc<dyn ArtifactDao>,
    outbox: Arc<dyn OutboxDao>,
}

pub struct SqliteBrowserDao;

pub struct BrowserEventSubscriber {
    repo: BrowserRepo,
}

pub trait BrowserDao: Send + Sync {
    fn list(&self, conn: &Connection) -> DbResult<Vec<BrowserNodeRow>>;
    fn list_all(&self, conn: &Connection) -> DbResult<Vec<BrowserNodeRow>>;
    fn get(&self, conn: &Connection, id: &str) -> DbResult<Option<BrowserNodeRow>>;
    fn upsert(&self, conn: &Connection, row: &BrowserNodeRow) -> DbResult<()>;
    fn delete(&self, conn: &Connection, id: &str) -> DbResult<()>;
    fn delete_subtree(&self, conn: &Connection, id: &str) -> DbResult<Vec<BrowserNodeRow>>;
    fn tombstone_subtree(
        &self,
        conn: &Connection,
        id: &str,
        updated_at: i64,
    ) -> DbResult<Vec<BrowserNodeRow>>;
    fn update_sync_status(
        &self,
        conn: &Connection,
        id: &str,
        sync_status: SyncStatus,
    ) -> DbResult<()>;
    fn prune_not_pending_except(
        &self,
        conn: &Connection,
        keep_ids: &[String],
    ) -> DbResult<Vec<String>>;
}

impl BrowserRepo {
    pub fn new(
        api: Arc<dyn BrowserApiClient>,
        dao: Arc<dyn BrowserDao>,
        artifact_dao: Arc<dyn ArtifactDao>,
        outbox: Arc<dyn OutboxDao>,
    ) -> Self {
        Self {
            api,
            dao,
            artifact_dao,
            outbox,
        }
    }

    pub fn list_nodes(&self, conn: &Connection) -> DbResult<Vec<BrowserNodeRow>> {
        self.dao.list(conn)
    }

    pub fn get_node(&self, conn: &Connection, id: &str) -> DbResult<Option<BrowserNodeRow>> {
        self.dao.get(conn, id)
    }

    pub fn upsert_node(&self, conn: &Connection, row: &BrowserNodeRow) -> DbResult<()> {
        self.dao.upsert(conn, row)
    }

    pub fn refresh_workspace(
        &self,
        db: &Db,
        events: &DataEventHub,
        config: &SyncConfig,
    ) -> DbResult<Vec<BrowserNodeRow>> {
        let remote = self.api.get_tree(config).map_err(crate::db::DbError::Api)?;
        let refreshed = db.with_tx(|tx| {
            let current_browser = self.dao.list_all(tx)?;
            let current_artifacts = self.artifact_dao.list_all(tx)?;
            let mut flattened = RemoteWorkspaceRows::default();
            flatten_remote_tree(
                &remote,
                None,
                &current_browser,
                &current_artifacts,
                &mut flattened,
            );

            for row in &flattened.browser_nodes {
                self.dao.upsert(tx, row)?;
            }
            for row in &flattened.artifacts {
                self.artifact_dao.upsert(tx, row)?;
            }
            let pruned_nodes = self
                .dao
                .prune_not_pending_except(tx, &flattened.browser_ids)?;
            let pruned_artifacts = self
                .artifact_dao
                .prune_not_pending_except(tx, &flattened.artifact_ids)?;
            Ok((
                flattened.browser_nodes,
                flattened.artifacts,
                pruned_nodes,
                pruned_artifacts,
            ))
        })?;

        for row in &refreshed.0 {
            emit_browser_node_updated(events, row);
        }
        for row in &refreshed.1 {
            events.emit(DataEvent::ArtifactChanged(row.clone()));
        }
        for id in &refreshed.2 {
            events.emit(DataEvent::BrowserNodeDeleted(EntityDeletedEvent {
                id: id.clone(),
            }));
        }
        for id in &refreshed.3 {
            events.emit(DataEvent::ArtifactDeleted(EntityDeletedEvent {
                id: id.clone(),
            }));
        }

        Ok(refreshed.0)
    }

    pub fn apply_backend_event(
        &self,
        db: &Db,
        events: &DataEventHub,
        event: &ValidatedBackendEvent,
    ) -> DbResult<bool> {
        match &event.payload {
            BackendEventPayload::TreeNodeSnapshot(payload) => {
                enum TreeOutcome {
                    Applied(BrowserNodeRow, Option<ArtifactRow>),
                    Skipped,
                    NeedsRefresh,
                }
                let outcome = db.with_tx(|tx| {
                    if let Some(parent_id) = payload.node.parent_id.as_ref() {
                        if self.dao.get(tx, parent_id)?.is_none() {
                            return Ok(TreeOutcome::NeedsRefresh);
                        }
                    }
                    let current_node = self.dao.get(tx, &payload.node.id)?;
                    if current_node.as_ref().map(|row| row.sync_status) == Some(SyncStatus::Pending)
                    {
                        return Ok(TreeOutcome::Skipped);
                    }
                    let updated_at = event_time_ms(
                        payload
                            .artifact
                            .as_ref()
                            .and_then(|artifact| artifact.updated_at.as_deref()),
                        event,
                    );
                    let node = browser_node_from_tree_snapshot(
                        &payload.node,
                        updated_at,
                        current_node.as_ref(),
                    );
                    self.dao.upsert(tx, &node)?;
                    let artifact = if let Some(artifact) = payload.artifact.as_ref() {
                        let current_artifact = self.artifact_dao.get(tx, &artifact.id)?;
                        if current_artifact.as_ref().map(|row| row.sync_status)
                            == Some(SyncStatus::Pending)
                        {
                            None
                        } else {
                            let row = artifact_row_from_snapshot(
                                artifact,
                                updated_at,
                                current_artifact.as_ref(),
                            );
                            self.artifact_dao.upsert(tx, &row)?;
                            Some(row)
                        }
                    } else {
                        None
                    };
                    Ok(TreeOutcome::Applied(node, artifact))
                })?;
                match outcome {
                    TreeOutcome::Applied(node, artifact) => {
                        if let Some(artifact) = artifact {
                            events.emit(DataEvent::ArtifactChanged(artifact));
                        }
                        if event.envelope.kind.ends_with(".created") {
                            emit_browser_node_created(events, &node);
                        } else {
                            emit_browser_node_updated(events, &node);
                        }
                        Ok(true)
                    }
                    TreeOutcome::Skipped => Ok(true),
                    TreeOutcome::NeedsRefresh => Ok(false),
                }
            }
            BackendEventPayload::PageSnapshot(payload) => {
                enum PageOutcome {
                    Applied(ArtifactRow),
                    Skipped,
                    NeedsRefresh,
                }
                let outcome = db.with_tx(|tx| {
                    let Some(current) = self.artifact_dao.get(tx, &payload.id)? else {
                        return Ok(PageOutcome::NeedsRefresh);
                    };
                    if current.sync_status == SyncStatus::Pending {
                        return Ok(PageOutcome::Skipped);
                    }
                    let row = artifact_row_from_page_snapshot(
                        payload,
                        event_time_ms(payload.updated_at.as_deref(), event),
                        &current,
                    );
                    self.artifact_dao.upsert(tx, &row)?;
                    Ok(PageOutcome::Applied(row))
                })?;
                match outcome {
                    PageOutcome::Applied(artifact) => {
                        events.emit(DataEvent::ArtifactChanged(artifact));
                        Ok(true)
                    }
                    PageOutcome::Skipped => Ok(true),
                    PageOutcome::NeedsRefresh => Ok(false),
                }
            }
            BackendEventPayload::EntityDeleted(payload) => {
                let deleted = db.with_tx(|tx| {
                    self.dao
                        .tombstone_subtree(tx, &payload.id, event_time_ms(None, event))
                })?;
                if deleted.is_empty() {
                    return Ok(false);
                }
                emit_deleted_events(events, &deleted);
                Ok(true)
            }
            // Legacy subscriber (no longer registered) — the new live path
            // (WorkspaceLiveSubscriber) handles SyncChange. Deleted at cutover.
            BackendEventPayload::SyncChange(_) => Ok(true),
        }
    }

    pub fn get_artifact(&self, conn: &Connection, id: &str) -> DbResult<Option<ArtifactRow>> {
        self.artifact_dao.get(conn, id)
    }

    fn emit_optional_artifact_changed(
        &self,
        db: &Db,
        events: &DataEventHub,
        row: &BrowserNodeRow,
    ) -> DbResult<()> {
        let Some(artifact_id) = row.artifact_id.as_ref() else {
            return Ok(());
        };
        let artifact = db.with_conn(|conn| self.artifact_dao.get(conn, artifact_id))?;
        if let Some(artifact) = artifact {
            events.emit(DataEvent::ArtifactChanged(artifact));
        }
        Ok(())
    }

    pub fn create(
        &self,
        db: &Db,
        events: &DataEventHub,
        config: Option<&SyncConfig>,
        mode: MutationMode,
        input: BrowserNodeCreateInput,
    ) -> DbResult<BrowserNodeRow> {
        let row = if let Some(config) = config.filter(sync_config_ready) {
            match self.create_remote(db, events, config, &input) {
                Ok(row) => row,
                Err(crate::db::DbError::Api(error))
                    if mode == MutationMode::UserAction && error.is_transient() =>
                {
                    self.create_local(db, events, input)?
                }
                Err(error) => return Err(error),
            }
        } else if mode == MutationMode::UserAction {
            self.create_local(db, events, input)?
        } else {
            return Err(crate::db::DbError::MissingLocalState);
        };

        emit_browser_node_created(events, &row);
        self.emit_optional_artifact_changed(db, events, &row)?;
        Ok(row)
    }

    pub fn rename(
        &self,
        db: &Db,
        events: &DataEventHub,
        config: Option<&SyncConfig>,
        mode: MutationMode,
        input: BrowserMutationInput,
    ) -> DbResult<BrowserNodeRow> {
        if let Some(config) = config.filter(sync_config_ready) {
            match self.rename_remote(db, events, config, &input) {
                Ok(row) => {
                    emit_browser_node_updated(events, &row);
                    return Ok(row);
                }
                Err(crate::db::DbError::Api(error))
                    if mode == MutationMode::UserAction && error.is_transient() => {}
                Err(error) => return Err(error),
            }
        }

        if mode == MutationMode::OutboxReplay {
            return Err(crate::db::DbError::MissingLocalState);
        }

        let row = self.rename_local(db, events, input)?;
        emit_browser_node_updated(events, &row);
        Ok(row)
    }

    pub fn delete(
        &self,
        db: &Db,
        events: &DataEventHub,
        config: Option<&SyncConfig>,
        mode: MutationMode,
        input: BrowserDeleteInput,
    ) -> DbResult<()> {
        let deleted = if let Some(config) = config.filter(sync_config_ready) {
            match self.delete_remote(db, config, &input) {
                Ok(rows) => rows,
                Err(crate::db::DbError::Api(error))
                    if mode == MutationMode::UserAction && error.is_transient() =>
                {
                    self.delete_local(db, &input)?
                }
                Err(error) => return Err(error),
            }
        } else if mode == MutationMode::UserAction {
            self.delete_local(db, &input)?
        } else {
            return Err(crate::db::DbError::MissingLocalState);
        };

        emit_deleted_events(events, &deleted);
        Ok(())
    }

    pub fn create_local(
        &self,
        db: &Db,
        _events: &DataEventHub,
        input: BrowserNodeCreateInput,
    ) -> DbResult<BrowserNodeRow> {
        let row = db.with_tx(|tx| {
            let row = browser_node_from_create_input(&input, SyncStatus::Pending);
            self.dao.upsert(tx, &row)?;
            if row.kind == "artifact" {
                self.artifact_dao.upsert(
                    tx,
                    &artifact_row_from_create_input(&input, SyncStatus::Pending)?,
                )?;
            }
            self.outbox.insert(
                tx,
                &outbox::OutboxMutationRow {
                    id: input.mutation_id.clone(),
                    entity_kind: "browser".to_string(),
                    entity_id: input.id.clone(),
                    op_type: "create".to_string(),
                    payload_json: serde_json::to_string(&input)?,
                    mutation_id: input.mutation_id.clone(),
                    attempts: 0,
                    status: "PENDING".to_string(),
                    last_error: None,
                    created_at: input.updated_at,
                    updated_at: input.updated_at,
                },
            )?;
            Ok(row)
        })?;

        Ok(row)
    }

    fn create_remote(
        &self,
        db: &Db,
        _events: &DataEventHub,
        config: &SyncConfig,
        input: &BrowserNodeCreateInput,
    ) -> DbResult<BrowserNodeRow> {
        let created = self
            .api
            .create_node(config, &browser_create_api_input(input))
            .map_err(crate::db::DbError::Api)?;
        let row = db.with_tx(|tx| {
            let row = BrowserNodeRow {
                id: created.node.id.clone(),
                parent_id: created.node.parent_id.clone(),
                title: created.node.title.clone(),
                kind: created.node.kind.clone(),
                artifact_id: created.node.artifact_id.clone(),
                updated_at: input.updated_at,
                sync_status: SyncStatus::Synced,
                version: 1,
            };
            if created.node.id != input.id {
                self.dao.delete(tx, &input.id)?;
                if row.kind == "artifact" {
                    self.artifact_dao.delete(tx, &input.id)?;
                }
            }
            self.dao.upsert(tx, &row)?;
            if let Some(artifact) = created.artifact.as_ref() {
                self.artifact_dao.upsert(
                    tx,
                    &ArtifactRow {
                        id: artifact.id.clone(),
                        folder_id: artifact.folder_id.clone(),
                        title: artifact.title.clone(),
                        kind: artifact.kind.clone(),
                        content: Some(artifact.content.clone()),
                        source_url: artifact.source_url.clone(),
                        resource_url: None,
                        metadata_json: artifact
                            .summary
                            .clone()
                            .map(|summary| serde_json::json!({ "summary": summary }).to_string()),
                        updated_at: input.updated_at,
                        sync_status: SyncStatus::Synced,
                        version: 1,
                    },
                )?;
            }
            Ok(row)
        })?;
        Ok(row)
    }

    pub fn rename_local(
        &self,
        db: &Db,
        _events: &DataEventHub,
        input: BrowserMutationInput,
    ) -> DbResult<BrowserNodeRow> {
        let row = db.with_tx(|tx| {
            let current = self.dao.get(tx, &input.id)?;
            let row = BrowserNodeRow {
                id: input.id.clone(),
                parent_id: input
                    .parent_id
                    .clone()
                    .or_else(|| current.as_ref().and_then(|node| node.parent_id.clone())),
                title: input.title.clone(),
                kind: current
                    .as_ref()
                    .map(|node| node.kind.clone())
                    .unwrap_or_else(|| "folder".to_string()),
                artifact_id: current.as_ref().and_then(|node| node.artifact_id.clone()),
                updated_at: input.updated_at,
                sync_status: SyncStatus::Pending,
                version: current.as_ref().map_or(1, |node| node.version + 1),
            };
            self.dao.upsert(tx, &row)?;
            self.outbox.insert(
                tx,
                &outbox::OutboxMutationRow {
                    id: input.mutation_id.clone(),
                    entity_kind: "browser".to_string(),
                    entity_id: input.id.clone(),
                    op_type: "update".to_string(),
                    payload_json: serde_json::to_string(&input)?,
                    mutation_id: input.mutation_id.clone(),
                    attempts: 0,
                    status: "PENDING".to_string(),
                    last_error: None,
                    created_at: input.updated_at,
                    updated_at: input.updated_at,
                },
            )?;
            Ok(row)
        })?;

        Ok(row)
    }

    fn rename_remote(
        &self,
        db: &Db,
        _events: &DataEventHub,
        config: &SyncConfig,
        input: &BrowserMutationInput,
    ) -> DbResult<BrowserNodeRow> {
        let node = self
            .api
            .rename_folder(config, &input.id, &input.title)
            .map_err(crate::db::DbError::Api)?;
        let row = db.with_tx(|tx| {
            let current = self.dao.get(tx, &input.id)?;
            let row = BrowserNodeRow {
                id: node.id.clone(),
                parent_id: node.parent_id.clone(),
                title: node.title.clone(),
                kind: current
                    .as_ref()
                    .map(|existing| existing.kind.clone())
                    .unwrap_or_else(|| "folder".to_string()),
                artifact_id: current
                    .as_ref()
                    .and_then(|existing| existing.artifact_id.clone()),
                updated_at: input.updated_at,
                sync_status: SyncStatus::Synced,
                version: current.as_ref().map_or(1, |existing| existing.version),
            };
            self.dao.upsert(tx, &row)?;
            Ok(row)
        })?;
        Ok(row)
    }

    fn delete_local(&self, db: &Db, input: &BrowserDeleteInput) -> DbResult<Vec<BrowserNodeRow>> {
        db.with_tx(|tx| {
            let deleted = self
                .dao
                .tombstone_subtree(tx, &input.id, input.updated_at)?;
            for row in &deleted {
                if let Some(artifact_id) = row.artifact_id.as_ref() {
                    self.artifact_dao
                        .tombstone(tx, artifact_id, input.updated_at)?;
                }
            }
            self.outbox.insert(
                tx,
                &outbox::OutboxMutationRow {
                    id: input.mutation_id.clone(),
                    entity_kind: "browser".to_string(),
                    entity_id: input.id.clone(),
                    op_type: "delete".to_string(),
                    payload_json: serde_json::to_string(input)?,
                    mutation_id: input.mutation_id.clone(),
                    attempts: 0,
                    status: "PENDING".to_string(),
                    last_error: None,
                    created_at: input.updated_at,
                    updated_at: input.updated_at,
                },
            )?;
            Ok(deleted)
        })
    }

    fn delete_remote(
        &self,
        db: &Db,
        config: &SyncConfig,
        input: &BrowserDeleteInput,
    ) -> DbResult<Vec<BrowserNodeRow>> {
        self.api
            .delete_node(config, &input.id)
            .map_err(crate::db::DbError::Api)?;
        db.with_tx(|tx| {
            let deleted = self.dao.delete_subtree(tx, &input.id)?;
            for row in &deleted {
                if let Some(artifact_id) = row.artifact_id.as_ref() {
                    self.artifact_dao.delete(tx, artifact_id)?;
                }
            }
            Ok(deleted)
        })
    }

    fn process_outbox_entry(
        &self,
        db: &Db,
        events: &DataEventHub,
        config: &SyncConfig,
        entry: &outbox::OutboxMutationRow,
    ) -> DbResult<()> {
        match entry.op_type.as_str() {
            "create" => {
                let payload: BrowserNodeCreateInput = serde_json::from_str(&entry.payload_json)?;
                self.create(
                    db,
                    events,
                    Some(config),
                    MutationMode::OutboxReplay,
                    payload,
                )?;
            }
            "update" => {
                let payload: BrowserMutationInput = serde_json::from_str(&entry.payload_json)?;
                self.rename(
                    db,
                    events,
                    Some(config),
                    MutationMode::OutboxReplay,
                    payload,
                )?;
            }
            "delete" => {
                let payload: BrowserDeleteInput = serde_json::from_str(&entry.payload_json)?;
                self.delete(
                    db,
                    events,
                    Some(config),
                    MutationMode::OutboxReplay,
                    payload,
                )?;
            }
            _ => {}
        }

        db.with_tx(|tx| self.outbox.delete(tx, &entry.id))?;
        Ok(())
    }

    fn mark_failed_entry(
        &self,
        tx: &rusqlite::Transaction<'_>,
        entry: &outbox::OutboxMutationRow,
    ) -> DbResult<()> {
        self.dao
            .update_sync_status(tx, &entry.entity_id, SyncStatus::Failed)?;
        self.artifact_dao
            .update_sync_status(tx, &entry.entity_id, SyncStatus::Failed)
    }
}

impl BrowserEventSubscriber {
    pub fn new(repo: BrowserRepo) -> Self {
        Self { repo }
    }
}

impl Default for BrowserEventSubscriber {
    fn default() -> Self {
        Self::new(BrowserRepo::default())
    }
}

impl EventSubscriber for BrowserEventSubscriber {
    fn name(&self) -> &'static str {
        "browser"
    }

    fn subscriptions(&self) -> Vec<BackendEventSubscription> {
        self.payload_registrations()
            .into_iter()
            .map(|registration| BackendEventSubscription {
                event_kind: Some(registration.event_kind.to_string()),
                stream: "workspace".to_string(),
                resource_kind: "*".to_string(),
                resource_id: "*".to_string(),
                qualifiers: None,
            })
            .collect()
    }

    fn payload_registrations(&self) -> Vec<PayloadRegistration> {
        vec![
            PayloadRegistration {
                event_kind: "artifact.created",
                decoder: decode_tree_node_snapshot_payload,
            },
            PayloadRegistration {
                event_kind: "artifact.updated",
                decoder: decode_tree_node_snapshot_payload,
            },
            PayloadRegistration {
                event_kind: "artifact.deleted",
                decoder: decode_entity_deleted_payload,
            },
            PayloadRegistration {
                event_kind: "folder.created",
                decoder: decode_tree_node_snapshot_payload,
            },
            PayloadRegistration {
                event_kind: "folder.updated",
                decoder: decode_tree_node_snapshot_payload,
            },
            PayloadRegistration {
                event_kind: "folder.deleted",
                decoder: decode_entity_deleted_payload,
            },
            PayloadRegistration {
                event_kind: "page.updated",
                decoder: decode_page_snapshot_payload,
            },
        ]
    }

    fn handle(
        &self,
        db: &Db,
        events: &DataEventHub,
        config: &SyncConfig,
        event: &ValidatedBackendEvent,
    ) -> DbResult<()> {
        if self.repo.apply_backend_event(db, events, event)? {
            return Ok(());
        }
        self.repo.refresh_workspace(db, events, config).map(|_| ())
    }
}

impl Default for BrowserRepo {
    fn default() -> Self {
        Self::new(
            Arc::new(BrowserApi),
            Arc::new(SqliteBrowserDao),
            Arc::new(SqliteArtifactDao),
            Arc::new(SqliteOutboxDao),
        )
    }
}

fn emit_browser_node_created(events: &DataEventHub, row: &BrowserNodeRow) {
    events.emit(DataEvent::BrowserNodeCreated(row.clone()));
}

fn emit_browser_node_updated(events: &DataEventHub, row: &BrowserNodeRow) {
    events.emit(DataEvent::BrowserNodeUpdated(row.clone()));
}

fn sync_config_ready(config: &&SyncConfig) -> bool {
    !config.api_base_url.trim().is_empty() && config.token.as_deref().unwrap_or("").trim() != ""
}

fn browser_node_from_create_input(
    input: &BrowserNodeCreateInput,
    sync_status: SyncStatus,
) -> BrowserNodeRow {
    let is_artifact = input.kind.trim().eq_ignore_ascii_case("artifact");
    BrowserNodeRow {
        id: input.id.clone(),
        parent_id: input.parent_id.clone(),
        title: input.title.clone(),
        kind: if is_artifact {
            "artifact".to_string()
        } else {
            "folder".to_string()
        },
        artifact_id: is_artifact.then(|| input.id.clone()),
        updated_at: input.updated_at,
        sync_status,
        version: 1,
    }
}

fn artifact_row_from_create_input(
    input: &BrowserNodeCreateInput,
    sync_status: SyncStatus,
) -> DbResult<ArtifactRow> {
    let kind = input
        .artifact_type
        .clone()
        .filter(|kind| !kind.trim().is_empty())
        .ok_or_else(|| crate::db::DbError::InvalidInput("artifact type is required".to_string()))?;
    Ok(ArtifactRow {
        id: input.id.clone(),
        folder_id: input.parent_id.clone(),
        title: input.title.clone(),
        kind,
        content: input.content.clone(),
        source_url: input.source_url.clone(),
        resource_url: None,
        metadata_json: input
            .summary
            .clone()
            .map(|summary| serde_json::json!({ "summary": summary }).to_string()),
        updated_at: input.updated_at,
        sync_status,
        version: 1,
    })
}

fn browser_create_api_input(
    input: &BrowserNodeCreateInput,
) -> crate::api::browser::BrowserNodeCreateApiInput {
    crate::api::browser::BrowserNodeCreateApiInput {
        id: input.id.clone(),
        kind: input.kind.clone(),
        parent_id: input.parent_id.clone(),
        title: input.title.clone(),
        artifact: input.artifact_type.as_ref().map(|artifact_type| {
            crate::api::browser::BrowserArtifactCreateApiPayload {
                id: Some(input.id.clone()),
                artifact_type: artifact_type.clone(),
                content: input.content.clone(),
                summary: input.summary.clone(),
                source_url: input.source_url.clone(),
            }
        }),
    }
}

fn artifact_row_from_snapshot(
    payload: &ArtifactSnapshotPayload,
    updated_at: i64,
    current: Option<&ArtifactRow>,
) -> ArtifactRow {
    ArtifactRow {
        id: payload.id.clone(),
        folder_id: payload.folder_id.clone(),
        title: payload.title.clone(),
        kind: payload.kind.clone(),
        content: payload
            .content
            .clone()
            .or_else(|| current.and_then(|row| row.content.clone())),
        source_url: payload.source_url.clone(),
        resource_url: payload
            .resource_url
            .clone()
            .or_else(|| current.and_then(|row| row.resource_url.clone())),
        metadata_json: snapshot_metadata_json(payload, current),
        updated_at,
        sync_status: SyncStatus::Synced,
        version: current.map_or(0, |row| row.version),
    }
}

fn artifact_row_from_page_snapshot(
    payload: &PageSnapshotPayload,
    updated_at: i64,
    current: &ArtifactRow,
) -> ArtifactRow {
    // Post-M5: page content lives in page_content, not artifacts.content.
    // The page.updated event still carries the title, so we refresh that
    // (and updated_at) on the artifact row but leave content alone. The
    // blocks payload is handled by the page_content event subscriber.
    ArtifactRow {
        id: current.id.clone(),
        folder_id: current.folder_id.clone(),
        title: payload.title.clone(),
        kind: current.kind.clone(),
        content: current.content.clone(),
        source_url: current.source_url.clone(),
        resource_url: current.resource_url.clone(),
        metadata_json: current.metadata_json.clone(),
        updated_at,
        sync_status: SyncStatus::Synced,
        version: current.version,
    }
}

fn browser_node_from_tree_snapshot(
    payload: &TreeNodePayload,
    updated_at: i64,
    current: Option<&BrowserNodeRow>,
) -> BrowserNodeRow {
    BrowserNodeRow {
        id: payload.id.clone(),
        parent_id: payload.parent_id.clone(),
        title: payload.title.clone(),
        kind: payload.kind.clone(),
        artifact_id: payload.artifact_id.clone(),
        updated_at,
        sync_status: SyncStatus::Synced,
        version: current.map_or(0, |row| row.version),
    }
}

fn snapshot_metadata_json(
    payload: &ArtifactSnapshotPayload,
    current: Option<&ArtifactRow>,
) -> Option<String> {
    if let Some(summary) = payload.summary.as_ref() {
        return Some(serde_json::json!({ "summary": summary }).to_string());
    }
    if let Some(metadata) = payload.metadata.as_ref().filter(|value| !value.is_null()) {
        return Some(metadata.to_string());
    }
    current.and_then(|row| row.metadata_json.clone())
}

fn event_time_ms(payload_updated_at: Option<&str>, event: &ValidatedBackendEvent) -> i64 {
    updated_at_ms(payload_updated_at.or(Some(event.envelope.occurred_at.as_str())))
}

#[derive(Default)]
struct RemoteWorkspaceRows {
    browser_nodes: Vec<BrowserNodeRow>,
    artifacts: Vec<ArtifactRow>,
    browser_ids: Vec<String>,
    artifact_ids: Vec<String>,
}

fn flatten_remote_tree(
    records: &[BrowserTreeApiRecord],
    parent_id: Option<String>,
    current_browser: &[BrowserNodeRow],
    current_artifacts: &[ArtifactRow],
    out: &mut RemoteWorkspaceRows,
) {
    for node in records {
        let node_parent_id = parent_id.clone();
        let current_node = current_browser.iter().find(|row| row.id == node.id);
        out.browser_ids.push(node.id.clone());
        if current_node.map(|row| row.sync_status) == Some(SyncStatus::Pending) {
            if let Some(row) = current_node {
                out.browser_nodes.push(row.clone());
            }
        } else {
            out.browser_nodes.push(BrowserNodeRow {
                id: node.id.clone(),
                parent_id: node_parent_id.clone(),
                title: node.title.clone(),
                kind: if node.kind.eq_ignore_ascii_case("artifact") {
                    "artifact".to_string()
                } else {
                    "folder".to_string()
                },
                artifact_id: node.artifact_id.clone(),
                updated_at: updated_at_ms(node.updated_at.as_deref()),
                sync_status: SyncStatus::Synced,
                version: current_node.map_or(0, |row| row.version),
            });
        }

        if node.kind.eq_ignore_ascii_case("artifact") {
            if let Some(artifact_id) = node.artifact_id.as_ref() {
                out.artifact_ids.push(artifact_id.clone());
                let current_artifact = current_artifacts.iter().find(|row| row.id == *artifact_id);
                if current_artifact.map(|row| row.sync_status) == Some(SyncStatus::Pending) {
                    if let Some(row) = current_artifact {
                        out.artifacts.push(row.clone());
                    }
                } else {
                    out.artifacts.push(ArtifactRow {
                        id: artifact_id.clone(),
                        folder_id: node_parent_id.clone(),
                        title: node.title.clone(),
                        kind: node
                            .artifact_type
                            .clone()
                            .or_else(|| current_artifact.map(|row| row.kind.clone()))
                            .unwrap_or_else(|| "page".to_string()),
                        content: current_artifact.and_then(|row| row.content.clone()),
                        source_url: current_artifact.and_then(|row| row.source_url.clone()),
                        resource_url: current_artifact.and_then(|row| row.resource_url.clone()),
                        metadata_json: current_artifact.and_then(|row| row.metadata_json.clone()),
                        updated_at: updated_at_ms(node.updated_at.as_deref()),
                        sync_status: SyncStatus::Synced,
                        version: current_artifact.map_or(0, |row| row.version),
                    });
                }
            }
        }

        let child_parent_id = if node.kind.eq_ignore_ascii_case("folder") {
            Some(node.id.clone())
        } else {
            node_parent_id
        };
        flatten_remote_tree(
            &node.children,
            child_parent_id,
            current_browser,
            current_artifacts,
            out,
        );
    }
}

fn updated_at_ms(value: Option<&str>) -> i64 {
    value
        .and_then(chrono_like_parse_ms)
        .unwrap_or_else(current_time_ms)
}

fn chrono_like_parse_ms(value: &str) -> Option<i64> {
    // Avoid adding a date-time dependency in the Tauri shell; RFC3339 from the API sorts as UTC.
    value.parse::<i64>().ok()
}

fn current_time_ms() -> i64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|duration| duration.as_millis() as i64)
        .unwrap_or(0)
}

fn emit_deleted_events(events: &DataEventHub, rows: &[BrowserNodeRow]) {
    for row in rows {
        events.emit(DataEvent::BrowserNodeDeleted(EntityDeletedEvent {
            id: row.id.clone(),
        }));
        if let Some(artifact_id) = row.artifact_id.as_ref() {
            events.emit(DataEvent::ArtifactDeleted(EntityDeletedEvent {
                id: artifact_id.clone(),
            }));
        }
    }
}

impl BrowserDao for SqliteBrowserDao {
    fn list(&self, conn: &Connection) -> DbResult<Vec<BrowserNodeRow>> {
        let mut stmt = conn.prepare(
            "SELECT id, parent_id, title, kind, artifact_id, updated_at, sync_status, version FROM folders WHERE is_deleted = 0 ORDER BY rowid ASC",
        )?;
        let rows = stmt.query_map([], map_row)?;
        rows.collect::<rusqlite::Result<Vec<_>>>()
            .map_err(Into::into)
    }

    fn list_all(&self, conn: &Connection) -> DbResult<Vec<BrowserNodeRow>> {
        let mut stmt = conn.prepare(
            "SELECT id, parent_id, title, kind, artifact_id, updated_at, sync_status, version FROM folders ORDER BY rowid ASC",
        )?;
        let rows = stmt.query_map([], map_row)?;
        rows.collect::<rusqlite::Result<Vec<_>>>()
            .map_err(Into::into)
    }

    fn get(&self, conn: &Connection, id: &str) -> DbResult<Option<BrowserNodeRow>> {
        let mut stmt = conn.prepare(
            "SELECT id, parent_id, title, kind, artifact_id, updated_at, sync_status, version FROM folders WHERE id = ?1",
        )?;
        let mut rows = stmt.query_map(params![id], map_row)?;
        Ok(rows.next().transpose()?)
    }

    fn upsert(&self, conn: &Connection, row: &BrowserNodeRow) -> DbResult<()> {
        conn.execute(
            "INSERT INTO folders (id, parent_id, title, kind, artifact_id, updated_at, sync_status, version, is_deleted) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, 0)
             ON CONFLICT(id) DO UPDATE SET
                parent_id=excluded.parent_id,
                title=excluded.title,
                kind=excluded.kind,
                artifact_id=excluded.artifact_id,
                updated_at=excluded.updated_at,
                sync_status=excluded.sync_status,
                version=excluded.version,
                is_deleted=0",
            params![
                row.id,
                row.parent_id,
                row.title,
                row.kind,
                row.artifact_id,
                row.updated_at,
                row.sync_status.as_str(),
                row.version
            ],
        )?;
        Ok(())
    }

    fn delete(&self, conn: &Connection, id: &str) -> DbResult<()> {
        conn.execute("DELETE FROM folders WHERE id = ?1", params![id])?;
        Ok(())
    }

    fn delete_subtree(&self, conn: &Connection, id: &str) -> DbResult<Vec<BrowserNodeRow>> {
        let rows = select_subtree(conn, id)?;
        conn.execute(
            "WITH RECURSIVE subtree(id) AS (
                SELECT id FROM folders WHERE id = ?1
                UNION ALL
                SELECT f.id FROM folders f JOIN subtree s ON f.parent_id = s.id
             )
             DELETE FROM folders WHERE id IN (SELECT id FROM subtree)",
            params![id],
        )?;
        Ok(rows)
    }

    fn tombstone_subtree(
        &self,
        conn: &Connection,
        id: &str,
        updated_at: i64,
    ) -> DbResult<Vec<BrowserNodeRow>> {
        let rows = select_subtree(conn, id)?;
        conn.execute(
            "WITH RECURSIVE subtree(id) AS (
                SELECT id FROM folders WHERE id = ?1
                UNION ALL
                SELECT f.id FROM folders f JOIN subtree s ON f.parent_id = s.id
             )
             UPDATE folders
                SET is_deleted = 1,
                    sync_status = 'PENDING',
                    updated_at = ?2,
                    version = version + 1
              WHERE id IN (SELECT id FROM subtree)",
            params![id, updated_at],
        )?;
        Ok(rows)
    }

    fn update_sync_status(
        &self,
        conn: &Connection,
        id: &str,
        sync_status: SyncStatus,
    ) -> DbResult<()> {
        conn.execute(
            "UPDATE folders SET sync_status = ?2, is_deleted = CASE WHEN ?2 = 'FAILED' THEN 0 ELSE is_deleted END WHERE id = ?1",
            params![id, sync_status.as_str()],
        )?;
        Ok(())
    }

    fn prune_not_pending_except(
        &self,
        conn: &Connection,
        keep_ids: &[String],
    ) -> DbResult<Vec<String>> {
        let ids = select_prunable_ids(conn, "folders", keep_ids)?;
        for id in &ids {
            conn.execute("DELETE FROM folders WHERE id = ?1", params![id])?;
        }
        Ok(ids)
    }
}

fn select_subtree(conn: &Connection, id: &str) -> DbResult<Vec<BrowserNodeRow>> {
    let mut stmt = conn.prepare(
        "WITH RECURSIVE subtree(id) AS (
            SELECT id FROM folders WHERE id = ?1
            UNION ALL
            SELECT f.id FROM folders f JOIN subtree s ON f.parent_id = s.id
         )
         SELECT id, parent_id, title, kind, artifact_id, updated_at, sync_status, version
           FROM folders
          WHERE id IN (SELECT id FROM subtree)",
    )?;
    let rows = stmt.query_map(params![id], map_row)?;
    rows.collect::<rusqlite::Result<Vec<_>>>()
        .map_err(Into::into)
}

fn select_prunable_ids(
    conn: &Connection,
    table: &str,
    keep_ids: &[String],
) -> DbResult<Vec<String>> {
    let mut stmt = conn.prepare(&format!(
        "SELECT id FROM {table} WHERE sync_status != 'PENDING'"
    ))?;
    let rows = stmt.query_map([], |row| row.get::<_, String>(0))?;
    let ids = rows
        .collect::<rusqlite::Result<Vec<_>>>()?
        .into_iter()
        .filter(|id| !keep_ids.iter().any(|keep_id| keep_id == id))
        .collect();
    Ok(ids)
}

fn map_row(row: &rusqlite::Row<'_>) -> rusqlite::Result<BrowserNodeRow> {
    Ok(BrowserNodeRow {
        id: row.get(0)?,
        parent_id: row.get(1)?,
        title: row.get(2)?,
        kind: row.get(3)?,
        artifact_id: row.get(4)?,
        updated_at: row.get(5)?,
        sync_status: SyncStatus::from_db(&row.get::<_, String>(6)?, 6)?,
        version: row.get(7)?,
    })
}

impl OutboxProcessor for BrowserRepo {
    fn entity_kind(&self) -> &'static str {
        "browser"
    }

    fn process(
        &self,
        db: &Db,
        events: &DataEventHub,
        config: &SyncConfig,
        entry: &outbox::OutboxMutationRow,
    ) -> DbResult<()> {
        self.process_outbox_entry(db, events, config, entry)
    }

    fn mark_failed(
        &self,
        tx: &rusqlite::Transaction<'_>,
        entry: &outbox::OutboxMutationRow,
    ) -> DbResult<()> {
        self.mark_failed_entry(tx, entry)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::api::{ApiError, ApiErrorKind};
    use crate::db::repo::outbox::{OutboxDao, SqliteOutboxDao};
    use crate::db::Db;
    use crate::events::DataEventHub;
    use std::collections::HashMap;
    use std::path::PathBuf;
    use std::sync::{Arc, Mutex};
    use std::time::{SystemTime, UNIX_EPOCH};

    #[derive(Clone)]
    enum BrowserApiBehavior {
        Ok(crate::api::browser::BrowserNodeCreateApiRecord),
        Err(ApiErrorKind),
    }

    struct MockBrowserApi {
        create: BrowserApiBehavior,
        rename: BrowserApiBehavior,
    }

    impl BrowserApiClient for MockBrowserApi {
        fn get_tree(
            &self,
            _config: &SyncConfig,
        ) -> crate::api::ApiResult<Vec<crate::api::browser::BrowserTreeApiRecord>> {
            Ok(vec![])
        }

        fn create_node(
            &self,
            _config: &SyncConfig,
            _input: &crate::api::browser::BrowserNodeCreateApiInput,
        ) -> crate::api::ApiResult<crate::api::browser::BrowserNodeCreateApiRecord> {
            match &self.create {
                BrowserApiBehavior::Ok(record) => Ok(record.clone()),
                BrowserApiBehavior::Err(kind) => Err(ApiError::for_test(*kind)),
            }
        }

        fn rename_folder(
            &self,
            _config: &SyncConfig,
            _id: &str,
            _title: &str,
        ) -> crate::api::ApiResult<crate::api::browser::BrowserNodeApiRecord> {
            match &self.rename {
                BrowserApiBehavior::Ok(record) => Ok(record.node.clone()),
                BrowserApiBehavior::Err(kind) => Err(ApiError::for_test(*kind)),
            }
        }

        fn delete_node(&self, _config: &SyncConfig, _id: &str) -> crate::api::ApiResult<()> {
            Ok(())
        }
    }

    #[derive(Default)]
    struct MockBrowserDao {
        rows: Mutex<HashMap<String, BrowserNodeRow>>,
    }

    impl MockBrowserDao {
        fn insert(&self, row: BrowserNodeRow) {
            self.rows.lock().unwrap().insert(row.id.clone(), row);
        }

        fn get_row(&self, id: &str) -> Option<BrowserNodeRow> {
            self.rows.lock().unwrap().get(id).cloned()
        }
    }

    impl BrowserDao for MockBrowserDao {
        fn list(&self, _conn: &Connection) -> DbResult<Vec<BrowserNodeRow>> {
            Ok(self.rows.lock().unwrap().values().cloned().collect())
        }

        fn list_all(&self, _conn: &Connection) -> DbResult<Vec<BrowserNodeRow>> {
            Ok(self.rows.lock().unwrap().values().cloned().collect())
        }

        fn get(&self, _conn: &Connection, id: &str) -> DbResult<Option<BrowserNodeRow>> {
            Ok(self.get_row(id))
        }

        fn upsert(&self, _conn: &Connection, row: &BrowserNodeRow) -> DbResult<()> {
            self.insert(row.clone());
            Ok(())
        }

        fn delete(&self, _conn: &Connection, id: &str) -> DbResult<()> {
            self.rows.lock().unwrap().remove(id);
            Ok(())
        }

        fn delete_subtree(&self, _conn: &Connection, id: &str) -> DbResult<Vec<BrowserNodeRow>> {
            let mut rows = self.rows.lock().unwrap();
            Ok(rows.remove(id).into_iter().collect())
        }

        fn tombstone_subtree(
            &self,
            _conn: &Connection,
            id: &str,
            _updated_at: i64,
        ) -> DbResult<Vec<BrowserNodeRow>> {
            let mut rows = self.rows.lock().unwrap();
            if let Some(row) = rows.get_mut(id) {
                row.sync_status = SyncStatus::Pending;
                return Ok(vec![row.clone()]);
            }
            Ok(vec![])
        }

        fn update_sync_status(
            &self,
            _conn: &Connection,
            id: &str,
            sync_status: SyncStatus,
        ) -> DbResult<()> {
            if let Some(row) = self.rows.lock().unwrap().get_mut(id) {
                row.sync_status = sync_status;
            }
            Ok(())
        }

        fn prune_not_pending_except(
            &self,
            _conn: &Connection,
            keep_ids: &[String],
        ) -> DbResult<Vec<String>> {
            let mut rows = self.rows.lock().unwrap();
            let ids = rows
                .keys()
                .filter(|id| !keep_ids.iter().any(|keep_id| keep_id == *id))
                .cloned()
                .collect::<Vec<_>>();
            for id in &ids {
                rows.remove(id);
            }
            Ok(ids)
        }
    }

    fn test_db(name: &str) -> Db {
        let nanos = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_nanos();
        let path = std::env::temp_dir().join(format!(
            "aladin_repo_test_{name}_{}_{}.sqlite",
            std::process::id(),
            nanos
        ));
        Db::open(PathBuf::from(path)).unwrap()
    }

    fn sync_config() -> SyncConfig {
        SyncConfig {
            api_base_url: "http://localhost:3000".to_string(),
            token: Some("token".to_string()),
        }
    }

    fn outbox_count(db: &Db) -> usize {
        let outbox = SqliteOutboxDao;
        db.with_conn(|conn| outbox.list_pending(conn, 100))
            .unwrap()
            .len()
    }

    #[test]
    fn create_remote_success_skips_outbox_and_persists_synced_row() {
        let db = test_db("browser_create_remote_success");
        let dao = Arc::new(MockBrowserDao::default());
        let repo = BrowserRepo::new(
            Arc::new(MockBrowserApi {
                create: BrowserApiBehavior::Ok(crate::api::browser::BrowserNodeCreateApiRecord {
                    node: crate::api::browser::BrowserNodeApiRecord {
                        id: "folder-1".to_string(),
                        parent_id: Some("parent-1".to_string()),
                        kind: "folder".to_string(),
                        title: "Remote Folder".to_string(),
                        artifact_id: None,
                        position: 1,
                    },
                    artifact: None,
                }),
                rename: BrowserApiBehavior::Err(ApiErrorKind::Unknown),
            }),
            dao.clone(),
            Arc::new(SqliteArtifactDao),
            Arc::new(SqliteOutboxDao),
        );

        let row = repo
            .create(
                &db,
                &DataEventHub::default(),
                Some(&sync_config()),
                MutationMode::UserAction,
                BrowserNodeCreateInput {
                    id: "temp-1".to_string(),
                    parent_id: Some("parent-1".to_string()),
                    kind: "folder".to_string(),
                    title: "Remote Folder".to_string(),
                    artifact_type: None,
                    content: None,
                    summary: None,
                    source_url: None,
                    updated_at: 10,
                    mutation_id: "mut-1".to_string(),
                },
            )
            .unwrap();

        assert_eq!(row.id, "folder-1");
        assert_eq!(row.sync_status, SyncStatus::Synced);
        assert_eq!(outbox_count(&db), 0);
        assert_eq!(dao.get_row("folder-1").unwrap().title, "Remote Folder");
    }

    #[test]
    fn create_unauthorized_does_not_fallback_local() {
        let db = test_db("browser_create_unauthorized");
        let dao = Arc::new(MockBrowserDao::default());
        let repo = BrowserRepo::new(
            Arc::new(MockBrowserApi {
                create: BrowserApiBehavior::Err(ApiErrorKind::Unauthorized),
                rename: BrowserApiBehavior::Err(ApiErrorKind::Unknown),
            }),
            dao.clone(),
            Arc::new(SqliteArtifactDao),
            Arc::new(SqliteOutboxDao),
        );

        let result = repo.create(
            &db,
            &DataEventHub::default(),
            Some(&sync_config()),
            MutationMode::UserAction,
            BrowserNodeCreateInput {
                id: "temp-1".to_string(),
                parent_id: None,
                kind: "folder".to_string(),
                title: "Blocked".to_string(),
                artifact_type: None,
                content: None,
                summary: None,
                source_url: None,
                updated_at: 10,
                mutation_id: "mut-1".to_string(),
            },
        );

        assert!(matches!(result, Err(crate::db::DbError::Api(_))));
        assert_eq!(outbox_count(&db), 0);
        assert!(dao.get_row("temp-1").is_none());
    }

    #[test]
    fn create_transient_falls_back_local_and_enqueues_outbox() {
        let db = test_db("browser_create_transient");
        let dao = Arc::new(MockBrowserDao::default());
        let repo = BrowserRepo::new(
            Arc::new(MockBrowserApi {
                create: BrowserApiBehavior::Err(ApiErrorKind::Transient),
                rename: BrowserApiBehavior::Err(ApiErrorKind::Unknown),
            }),
            dao.clone(),
            Arc::new(SqliteArtifactDao),
            Arc::new(SqliteOutboxDao),
        );

        let row = repo
            .create(
                &db,
                &DataEventHub::default(),
                Some(&sync_config()),
                MutationMode::UserAction,
                BrowserNodeCreateInput {
                    id: "temp-1".to_string(),
                    parent_id: None,
                    kind: "folder".to_string(),
                    title: "Pending".to_string(),
                    artifact_type: None,
                    content: None,
                    summary: None,
                    source_url: None,
                    updated_at: 10,
                    mutation_id: "mut-1".to_string(),
                },
            )
            .unwrap();

        assert_eq!(row.id, "temp-1");
        assert_eq!(row.sync_status, SyncStatus::Pending);
        assert_eq!(outbox_count(&db), 1);
        assert_eq!(dao.get_row("temp-1").unwrap().title, "Pending");
    }

    #[test]
    fn rename_remote_success_uses_remote_and_skips_outbox() {
        let db = test_db("browser_rename_remote_success");
        let dao = Arc::new(MockBrowserDao::default());
        dao.insert(BrowserNodeRow {
            id: "folder-1".to_string(),
            parent_id: None,
            title: "Old".to_string(),
            kind: "folder".to_string(),
            artifact_id: None,
            updated_at: 1,
            sync_status: SyncStatus::Synced,
            version: 4,
        });
        let repo = BrowserRepo::new(
            Arc::new(MockBrowserApi {
                create: BrowserApiBehavior::Err(ApiErrorKind::Unknown),
                rename: BrowserApiBehavior::Ok(crate::api::browser::BrowserNodeCreateApiRecord {
                    node: crate::api::browser::BrowserNodeApiRecord {
                        id: "folder-1".to_string(),
                        parent_id: None,
                        kind: "folder".to_string(),
                        title: "New".to_string(),
                        artifact_id: None,
                        position: 1,
                    },
                    artifact: None,
                }),
            }),
            dao.clone(),
            Arc::new(SqliteArtifactDao),
            Arc::new(SqliteOutboxDao),
        );

        let row = repo
            .rename(
                &db,
                &DataEventHub::default(),
                Some(&sync_config()),
                MutationMode::UserAction,
                BrowserMutationInput {
                    id: "folder-1".to_string(),
                    parent_id: None,
                    title: "New".to_string(),
                    updated_at: 10,
                    mutation_id: "mut-2".to_string(),
                },
            )
            .unwrap();

        assert_eq!(row.title, "New");
        assert_eq!(row.sync_status, SyncStatus::Synced);
        assert_eq!(row.version, 4);
        assert_eq!(outbox_count(&db), 0);
    }
}

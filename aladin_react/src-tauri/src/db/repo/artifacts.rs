use rusqlite::{params, Connection};
use serde::{Deserialize, Serialize};
use std::sync::Arc;

use crate::{
    api::artifacts::{
        ArtifactApi, ArtifactApiClient, ArtifactCreateApiInput, BrowserNodeApiRecord,
    },
    db::{
        repo::{
            browser::{BrowserDao, BrowserNodeRow, SqliteBrowserDao},
            outbox::{self, OutboxDao, SqliteOutboxDao},
            MutationMode, SyncStatus,
        },
        Db, DbResult,
    },
    events::{DataEvent, DataEventHub, EntityDeletedEvent},
    sync::{OutboxProcessor, SyncConfig},
};

#[derive(Debug, Serialize, Deserialize, Clone)]
#[serde(rename_all = "camelCase")]
pub struct ArtifactRow {
    pub id: String,
    pub folder_id: Option<String>,
    pub title: String,
    pub kind: String,
    pub content: Option<String>,
    pub source_url: Option<String>,
    pub resource_url: Option<String>,
    pub metadata_json: Option<String>,
    pub updated_at: i64,
    pub sync_status: SyncStatus,
    pub version: i64,
}

#[derive(Debug, Deserialize, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ArtifactLocalMutationInput {
    pub id: String,
    pub folder_id: Option<String>,
    pub r#type: Option<String>,
    pub title: String,
    pub content: Option<String>,
    pub summary: Option<String>,
    pub source_url: Option<String>,
    pub updated_at: i64,
    pub mutation_id: String,
}

#[derive(Debug, Deserialize, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ArtifactDeleteInput {
    pub id: String,
    pub updated_at: i64,
    pub mutation_id: String,
}

pub struct ArtifactRepo {
    api: Arc<dyn ArtifactApiClient>,
    dao: Arc<dyn ArtifactDao>,
    browser_dao: Arc<dyn BrowserDao>,
    outbox: Arc<dyn OutboxDao>,
}

pub struct SqliteArtifactDao;

pub trait ArtifactDao: Send + Sync {
    fn list(&self, conn: &Connection) -> DbResult<Vec<ArtifactRow>>;
    fn list_all(&self, conn: &Connection) -> DbResult<Vec<ArtifactRow>>;
    fn get(&self, conn: &Connection, id: &str) -> DbResult<Option<ArtifactRow>>;
    fn get_many(&self, conn: &Connection, ids: &[String]) -> DbResult<Vec<ArtifactRow>>;
    fn upsert(&self, conn: &Connection, row: &ArtifactRow) -> DbResult<()>;
    fn delete(&self, conn: &Connection, id: &str) -> DbResult<()>;
    fn tombstone(
        &self,
        conn: &Connection,
        id: &str,
        updated_at: i64,
    ) -> DbResult<Option<ArtifactRow>>;
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

impl ArtifactRepo {
    pub fn new(
        api: Arc<dyn ArtifactApiClient>,
        dao: Arc<dyn ArtifactDao>,
        browser_dao: Arc<dyn BrowserDao>,
        outbox: Arc<dyn OutboxDao>,
    ) -> Self {
        Self {
            api,
            dao,
            browser_dao,
            outbox,
        }
    }

    pub fn list_artifacts(&self, conn: &Connection) -> DbResult<Vec<ArtifactRow>> {
        self.dao.list(conn)
    }

    pub fn get_artifact(&self, conn: &Connection, id: &str) -> DbResult<Option<ArtifactRow>> {
        self.dao.get(conn, id)
    }

    pub fn get_artifacts(&self, conn: &Connection, ids: &[String]) -> DbResult<Vec<ArtifactRow>> {
        self.dao.get_many(conn, ids)
    }

    pub fn upsert_artifact(&self, conn: &Connection, row: &ArtifactRow) -> DbResult<()> {
        self.dao.upsert(conn, row)
    }

    pub fn create(
        &self,
        db: &Db,
        events: &DataEventHub,
        config: Option<&SyncConfig>,
        mode: MutationMode,
        input: ArtifactLocalMutationInput,
    ) -> DbResult<ArtifactRow> {
        validate_artifact_type(input.r#type.as_deref())?;
        if let Some(config) = config.filter(sync_config_ready) {
            match self.create_remote(db, events, config, &input) {
                Ok(row) => {
                    emit_artifact_events(events, event_source(row.sync_status), "created", &row);
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
        let row = self.create_local(db, events, input)?;
        emit_artifact_events(events, event_source(row.sync_status), "created", &row);
        Ok(row)
    }

    pub fn rename(
        &self,
        db: &Db,
        events: &DataEventHub,
        config: Option<&SyncConfig>,
        mode: MutationMode,
        input: ArtifactLocalMutationInput,
    ) -> DbResult<ArtifactRow> {
        if let Some(config) = config.filter(sync_config_ready) {
            match self.rename_remote(db, events, config, &input) {
                Ok(row) => {
                    emit_artifact_events(events, event_source(row.sync_status), "updated", &row);
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
        emit_artifact_events(events, event_source(row.sync_status), "updated", &row);
        Ok(row)
    }

    pub fn delete(
        &self,
        db: &Db,
        events: &DataEventHub,
        config: Option<&SyncConfig>,
        mode: MutationMode,
        input: ArtifactDeleteInput,
    ) -> DbResult<()> {
        if let Some(config) = config.filter(sync_config_ready) {
            match self.delete_remote(db, config, &input) {
                Ok(()) => {
                    emit_artifact_deleted(events, &input.id);
                    return Ok(());
                }
                Err(crate::db::DbError::Api(error))
                    if mode == MutationMode::UserAction && error.is_transient() => {}
                Err(error) => return Err(error),
            }
        }

        if mode == MutationMode::OutboxReplay {
            return Err(crate::db::DbError::MissingLocalState);
        }

        self.delete_local(db, &input)?;
        emit_artifact_deleted(events, &input.id);
        Ok(())
    }

    pub fn create_local(
        &self,
        db: &Db,
        _events: &DataEventHub,
        input: ArtifactLocalMutationInput,
    ) -> DbResult<ArtifactRow> {
        let row = db.with_tx(|tx| {
            let row = ArtifactRow {
                id: input.id.clone(),
                folder_id: input.folder_id.clone(),
                title: input.title.clone(),
                kind: input.r#type.clone().expect("validated artifact type"),
                content: input.content.clone(),
                source_url: input.source_url.clone(),
                resource_url: None,
                metadata_json: input
                    .summary
                    .clone()
                    .map(|summary| serde_json::json!({ "summary": summary }).to_string()),
                updated_at: input.updated_at,
                sync_status: SyncStatus::Pending,
                version: 1,
            };
            self.dao.upsert(tx, &row)?;
            self.browser_dao
                .upsert(tx, &browser_node_from_artifact(&row))?;
            self.outbox.insert(
                tx,
                &outbox::OutboxMutationRow {
                    id: input.mutation_id.clone(),
                    entity_kind: "artifact".to_string(),
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
        input: &ArtifactLocalMutationInput,
    ) -> DbResult<ArtifactRow> {
        let created = self
            .api
            .create_artifact(
                config,
                &ArtifactCreateApiInput {
                    id: &input.id,
                    artifact_type: input.r#type.as_deref().expect("validated artifact type"),
                    folder_id: input.folder_id.as_deref(),
                    title: &input.title,
                    content: input.content.as_deref(),
                    summary: input.summary.as_deref(),
                    source_url: input.source_url.as_deref(),
                },
            )
            .map_err(crate::db::DbError::Api)?;
        let row = db.with_tx(|tx| {
            let row = ArtifactRow {
                id: created.artifact.id.clone(),
                folder_id: created.artifact.folder_id.clone(),
                title: created.artifact.title.clone(),
                kind: created.artifact.kind.clone(),
                content: Some(created.artifact.content.clone()),
                source_url: created.artifact.source_url.clone(),
                resource_url: None,
                metadata_json: created
                    .artifact
                    .summary
                    .clone()
                    .map(|summary| serde_json::json!({ "summary": summary }).to_string()),
                updated_at: input.updated_at,
                sync_status: SyncStatus::Synced,
                version: 1,
            };
            if created.artifact.id != input.id {
                self.dao.delete(tx, &input.id)?;
                self.browser_dao.delete(tx, &input.id)?;
            }
            self.dao.upsert(tx, &row)?;
            self.browser_dao
                .upsert(tx, &browser_node_from_api(&created.node, &row))?;
            Ok(row)
        })?;
        Ok(row)
    }

    pub fn rename_local(
        &self,
        db: &Db,
        _events: &DataEventHub,
        input: ArtifactLocalMutationInput,
    ) -> DbResult<ArtifactRow> {
        let row = db.with_tx(|tx| {
            let current = self
                .dao
                .get(tx, &input.id)?
                .ok_or(crate::db::DbError::MissingLocalState)?;
            let row = ArtifactRow {
                id: current.id,
                folder_id: current.folder_id,
                title: input.title.clone(),
                kind: current.kind,
                content: current.content,
                source_url: current.source_url,
                resource_url: current.resource_url,
                metadata_json: current.metadata_json,
                updated_at: input.updated_at,
                sync_status: SyncStatus::Pending,
                version: current.version + 1,
            };
            self.dao.upsert(tx, &row)?;
            self.browser_dao
                .upsert(tx, &browser_node_from_artifact(&row))?;
            self.outbox.insert(
                tx,
                &outbox::OutboxMutationRow {
                    id: input.mutation_id.clone(),
                    entity_kind: "artifact".to_string(),
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
        input: &ArtifactLocalMutationInput,
    ) -> DbResult<ArtifactRow> {
        let artifact = self
            .api
            .rename_artifact(config, &input.id, &input.title)
            .map_err(crate::db::DbError::Api)?;
        let row = db.with_tx(|tx| {
            let current = self
                .dao
                .get(tx, &input.id)?
                .ok_or(crate::db::DbError::MissingLocalState)?;
            let row = ArtifactRow {
                id: artifact.id.clone(),
                folder_id: artifact.folder_id.clone(),
                title: artifact.title.clone(),
                kind: artifact.kind.clone(),
                content: Some(artifact.content.clone()),
                source_url: artifact.source_url.clone(),
                resource_url: current.resource_url,
                metadata_json: current.metadata_json,
                updated_at: input.updated_at,
                sync_status: SyncStatus::Synced,
                version: current.version,
            };
            self.dao.upsert(tx, &row)?;
            self.browser_dao
                .upsert(tx, &browser_node_from_artifact(&row))?;
            Ok(row)
        })?;
        Ok(row)
    }

    fn delete_local(&self, db: &Db, input: &ArtifactDeleteInput) -> DbResult<()> {
        db.with_tx(|tx| {
            self.dao.tombstone(tx, &input.id, input.updated_at)?;
            self.browser_dao
                .tombstone_subtree(tx, &input.id, input.updated_at)?;
            self.outbox.insert(
                tx,
                &outbox::OutboxMutationRow {
                    id: input.mutation_id.clone(),
                    entity_kind: "artifact".to_string(),
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
            Ok(())
        })
    }

    fn delete_remote(
        &self,
        db: &Db,
        config: &SyncConfig,
        input: &ArtifactDeleteInput,
    ) -> DbResult<()> {
        self.api
            .delete_artifact(config, &input.id)
            .map_err(crate::db::DbError::Api)?;
        db.with_tx(|tx| {
            self.dao.delete(tx, &input.id)?;
            self.browser_dao.delete(tx, &input.id)?;
            Ok(())
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
                let payload: ArtifactLocalMutationInput =
                    serde_json::from_str(&entry.payload_json)?;
                self.create(
                    db,
                    events,
                    Some(config),
                    MutationMode::OutboxReplay,
                    payload,
                )?;
            }
            "update" => {
                let payload: ArtifactLocalMutationInput =
                    serde_json::from_str(&entry.payload_json)?;
                self.rename(
                    db,
                    events,
                    Some(config),
                    MutationMode::OutboxReplay,
                    payload,
                )?;
            }
            "delete" => {
                let payload: ArtifactDeleteInput = serde_json::from_str(&entry.payload_json)?;
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
        self.browser_dao
            .update_sync_status(tx, &entry.entity_id, SyncStatus::Failed)
    }
}

impl Default for ArtifactRepo {
    fn default() -> Self {
        Self::new(
            Arc::new(ArtifactApi),
            Arc::new(SqliteArtifactDao),
            Arc::new(SqliteBrowserDao),
            Arc::new(SqliteOutboxDao),
        )
    }
}

fn browser_node_from_artifact(row: &ArtifactRow) -> BrowserNodeRow {
    BrowserNodeRow {
        id: row.id.clone(),
        parent_id: row.folder_id.clone(),
        title: row.title.clone(),
        kind: "artifact".to_string(),
        artifact_id: Some(row.id.clone()),
        updated_at: row.updated_at,
        sync_status: row.sync_status,
        version: row.version,
    }
}

fn browser_node_from_api(node: &BrowserNodeApiRecord, row: &ArtifactRow) -> BrowserNodeRow {
    BrowserNodeRow {
        id: node.id.clone(),
        parent_id: node.parent_id.clone(),
        title: node.title.clone(),
        kind: node.kind.clone(),
        artifact_id: node.artifact_id.clone(),
        updated_at: row.updated_at,
        sync_status: row.sync_status,
        version: row.version,
    }
}

fn emit_artifact_events(events: &DataEventHub, _source: &str, change: &str, row: &ArtifactRow) {
    events.emit(DataEvent::ArtifactChanged(row.clone()));
    if change == "created" {
        events.emit(DataEvent::BrowserNodeCreated(browser_node_from_artifact(
            row,
        )));
    } else {
        events.emit(DataEvent::BrowserNodeUpdated(browser_node_from_artifact(
            row,
        )));
    }
}

fn emit_artifact_deleted(events: &DataEventHub, id: &str) {
    events.emit(DataEvent::ArtifactDeleted(EntityDeletedEvent {
        id: id.to_string(),
    }));
    events.emit(DataEvent::BrowserNodeDeleted(EntityDeletedEvent {
        id: id.to_string(),
    }));
}

fn sync_config_ready(config: &&SyncConfig) -> bool {
    !config.api_base_url.trim().is_empty() && config.token.as_deref().unwrap_or("").trim() != ""
}

fn event_source(sync_status: SyncStatus) -> &'static str {
    match sync_status {
        SyncStatus::Pending => "local",
        SyncStatus::Synced | SyncStatus::Failed => "remote",
    }
}

fn validate_artifact_type(artifact_type: Option<&str>) -> DbResult<()> {
    if artifact_type
        .map(|artifact_type| artifact_type.trim().is_empty())
        .unwrap_or(true)
    {
        return Err(crate::db::DbError::InvalidInput(
            "artifact type is required".to_string(),
        ));
    }
    Ok(())
}

impl ArtifactDao for SqliteArtifactDao {
    fn list(&self, conn: &Connection) -> DbResult<Vec<ArtifactRow>> {
        let mut stmt = conn.prepare(
            "SELECT id, folder_id, title, kind, content, source_url, resource_url, metadata_json, updated_at, sync_status, version FROM artifacts WHERE is_deleted = 0 ORDER BY rowid ASC",
        )?;
        let rows = stmt.query_map([], map_row)?;
        rows.collect::<rusqlite::Result<Vec<_>>>()
            .map_err(Into::into)
    }

    fn list_all(&self, conn: &Connection) -> DbResult<Vec<ArtifactRow>> {
        let mut stmt = conn.prepare(
            "SELECT id, folder_id, title, kind, content, source_url, resource_url, metadata_json, updated_at, sync_status, version FROM artifacts ORDER BY rowid ASC",
        )?;
        let rows = stmt.query_map([], map_row)?;
        rows.collect::<rusqlite::Result<Vec<_>>>()
            .map_err(Into::into)
    }

    fn get(&self, conn: &Connection, id: &str) -> DbResult<Option<ArtifactRow>> {
        let mut stmt = conn.prepare(SELECT_BASE_WHERE_ID)?;
        let mut rows = stmt.query_map(params![id], map_row)?;
        Ok(rows.next().transpose()?)
    }

    fn get_many(&self, conn: &Connection, ids: &[String]) -> DbResult<Vec<ArtifactRow>> {
        if ids.is_empty() {
            return Ok(vec![]);
        }
        let placeholders = std::iter::repeat("?")
            .take(ids.len())
            .collect::<Vec<_>>()
            .join(",");
        let sql = format!("{SELECT_BASE} WHERE is_deleted = 0 AND id IN ({placeholders})");
        let mut stmt = conn.prepare(&sql)?;
        let params_iter: Vec<&dyn rusqlite::ToSql> =
            ids.iter().map(|s| s as &dyn rusqlite::ToSql).collect();
        let rows = stmt.query_map(params_iter.as_slice(), map_row)?;
        rows.collect::<rusqlite::Result<Vec<_>>>()
            .map_err(Into::into)
    }

    fn upsert(&self, conn: &Connection, row: &ArtifactRow) -> DbResult<()> {
        conn.execute(
            "INSERT INTO artifacts
                (id, folder_id, title, kind, content, source_url, resource_url, metadata_json, updated_at, sync_status, version, is_deleted)
             VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, 0)
             ON CONFLICT(id) DO UPDATE SET
                folder_id=excluded.folder_id,
                title=excluded.title,
                kind=excluded.kind,
                content=excluded.content,
                source_url=excluded.source_url,
                resource_url=excluded.resource_url,
                metadata_json=excluded.metadata_json,
                updated_at=excluded.updated_at,
                sync_status=excluded.sync_status,
                version=excluded.version,
                is_deleted=0",
            params![
                row.id,
                row.folder_id,
                row.title,
                row.kind,
                row.content,
                row.source_url,
                row.resource_url,
                row.metadata_json,
                row.updated_at,
                row.sync_status.as_str(),
                row.version,
            ],
        )?;
        Ok(())
    }

    fn delete(&self, conn: &Connection, id: &str) -> DbResult<()> {
        conn.execute("DELETE FROM artifacts WHERE id = ?1", params![id])?;
        Ok(())
    }

    fn tombstone(
        &self,
        conn: &Connection,
        id: &str,
        updated_at: i64,
    ) -> DbResult<Option<ArtifactRow>> {
        let current = self.get(conn, id)?;
        conn.execute(
            "UPDATE artifacts
                SET is_deleted = 1,
                    sync_status = 'PENDING',
                    updated_at = ?2,
                    version = version + 1
              WHERE id = ?1",
            params![id, updated_at],
        )?;
        Ok(current)
    }

    fn update_sync_status(
        &self,
        conn: &Connection,
        id: &str,
        sync_status: SyncStatus,
    ) -> DbResult<()> {
        conn.execute(
            "UPDATE artifacts SET sync_status = ?2, is_deleted = CASE WHEN ?2 = 'FAILED' THEN 0 ELSE is_deleted END WHERE id = ?1",
            params![id, sync_status.as_str()],
        )?;
        Ok(())
    }

    fn prune_not_pending_except(
        &self,
        conn: &Connection,
        keep_ids: &[String],
    ) -> DbResult<Vec<String>> {
        let mut stmt = conn.prepare("SELECT id FROM artifacts WHERE sync_status != 'PENDING'")?;
        let rows = stmt.query_map([], |row| row.get::<_, String>(0))?;
        let ids = rows
            .collect::<rusqlite::Result<Vec<_>>>()?
            .into_iter()
            .filter(|id| !keep_ids.iter().any(|keep_id| keep_id == id))
            .collect::<Vec<_>>();
        for id in &ids {
            conn.execute("DELETE FROM artifacts WHERE id = ?1", params![id])?;
        }
        Ok(ids)
    }
}

const SELECT_BASE: &str =
    "SELECT id, folder_id, title, kind, content, source_url, resource_url, metadata_json, updated_at, sync_status, version FROM artifacts";
const SELECT_BASE_WHERE_ID: &str = "SELECT id, folder_id, title, kind, content, source_url, resource_url, metadata_json, updated_at, sync_status, version FROM artifacts WHERE id = ?1 AND is_deleted = 0";

fn map_row(row: &rusqlite::Row<'_>) -> rusqlite::Result<ArtifactRow> {
    Ok(ArtifactRow {
        id: row.get(0)?,
        folder_id: row.get(1)?,
        title: row.get(2)?,
        kind: row.get(3)?,
        content: row.get(4)?,
        source_url: row.get(5)?,
        resource_url: row.get(6)?,
        metadata_json: row.get(7)?,
        updated_at: row.get(8)?,
        sync_status: SyncStatus::from_db(&row.get::<_, String>(9)?, 9)?,
        version: row.get(10)?,
    })
}

impl OutboxProcessor for ArtifactRepo {
    fn entity_kind(&self) -> &'static str {
        "artifact"
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
    enum ArtifactApiBehavior {
        Ok(crate::api::artifacts::ArtifactCreateApiRecord),
        Err(ApiErrorKind),
    }

    struct MockArtifactApi {
        create: ArtifactApiBehavior,
        rename: ArtifactApiBehavior,
    }

    impl ArtifactApiClient for MockArtifactApi {
        fn create_artifact(
            &self,
            _config: &SyncConfig,
            _input: &ArtifactCreateApiInput<'_>,
        ) -> crate::api::ApiResult<crate::api::artifacts::ArtifactCreateApiRecord> {
            match &self.create {
                ArtifactApiBehavior::Ok(record) => Ok(record.clone()),
                ArtifactApiBehavior::Err(kind) => Err(ApiError::for_test(*kind)),
            }
        }

        fn rename_artifact(
            &self,
            _config: &SyncConfig,
            _id: &str,
            _title: &str,
        ) -> crate::api::ApiResult<crate::api::artifacts::ArtifactApiRecord> {
            match &self.rename {
                ArtifactApiBehavior::Ok(record) => Ok(record.artifact.clone()),
                ArtifactApiBehavior::Err(kind) => Err(ApiError::for_test(*kind)),
            }
        }

        fn delete_artifact(&self, _config: &SyncConfig, _id: &str) -> crate::api::ApiResult<()> {
            Ok(())
        }
    }

    #[derive(Default)]
    struct MockArtifactDao {
        rows: Mutex<HashMap<String, ArtifactRow>>,
    }

    impl MockArtifactDao {
        fn insert(&self, row: ArtifactRow) {
            self.rows.lock().unwrap().insert(row.id.clone(), row);
        }

        fn get_row(&self, id: &str) -> Option<ArtifactRow> {
            self.rows.lock().unwrap().get(id).cloned()
        }
    }

    impl ArtifactDao for MockArtifactDao {
        fn list(&self, _conn: &Connection) -> DbResult<Vec<ArtifactRow>> {
            Ok(self.rows.lock().unwrap().values().cloned().collect())
        }

        fn list_all(&self, _conn: &Connection) -> DbResult<Vec<ArtifactRow>> {
            Ok(self.rows.lock().unwrap().values().cloned().collect())
        }

        fn get(&self, _conn: &Connection, id: &str) -> DbResult<Option<ArtifactRow>> {
            Ok(self.get_row(id))
        }

        fn get_many(&self, _conn: &Connection, ids: &[String]) -> DbResult<Vec<ArtifactRow>> {
            let rows = self.rows.lock().unwrap();
            Ok(ids.iter().filter_map(|id| rows.get(id).cloned()).collect())
        }

        fn upsert(&self, _conn: &Connection, row: &ArtifactRow) -> DbResult<()> {
            self.insert(row.clone());
            Ok(())
        }

        fn delete(&self, _conn: &Connection, id: &str) -> DbResult<()> {
            self.rows.lock().unwrap().remove(id);
            Ok(())
        }

        fn tombstone(
            &self,
            _conn: &Connection,
            id: &str,
            _updated_at: i64,
        ) -> DbResult<Option<ArtifactRow>> {
            let mut rows = self.rows.lock().unwrap();
            if let Some(row) = rows.get_mut(id) {
                row.sync_status = SyncStatus::Pending;
                return Ok(Some(row.clone()));
            }
            Ok(None)
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
        let db = test_db("artifact_create_remote_success");
        let dao = Arc::new(MockArtifactDao::default());
        let repo = ArtifactRepo::new(
            Arc::new(MockArtifactApi {
                create: ArtifactApiBehavior::Ok(crate::api::artifacts::ArtifactCreateApiRecord {
                    artifact: crate::api::artifacts::ArtifactApiRecord {
                        id: "artifact-1".to_string(),
                        kind: "page".to_string(),
                        folder_id: Some("folder-1".to_string()),
                        title: "Remote Artifact".to_string(),
                        content: "body".to_string(),
                        summary: Some("sum".to_string()),
                        source_url: None,
                    },
                    node: crate::api::artifacts::BrowserNodeApiRecord {
                        id: "artifact-1".to_string(),
                        parent_id: Some("folder-1".to_string()),
                        kind: "artifact".to_string(),
                        title: "Remote Artifact".to_string(),
                        artifact_id: Some("artifact-1".to_string()),
                        position: 1,
                    },
                }),
                rename: ArtifactApiBehavior::Err(ApiErrorKind::Unknown),
            }),
            dao.clone(),
            Arc::new(SqliteBrowserDao),
            Arc::new(SqliteOutboxDao),
        );

        let row = repo
            .create(
                &db,
                &DataEventHub::default(),
                Some(&sync_config()),
                MutationMode::UserAction,
                ArtifactLocalMutationInput {
                    id: "temp-1".to_string(),
                    folder_id: Some("folder-1".to_string()),
                    r#type: Some("page".to_string()),
                    title: "Remote Artifact".to_string(),
                    content: Some("body".to_string()),
                    summary: Some("sum".to_string()),
                    source_url: None,
                    updated_at: 10,
                    mutation_id: "mut-1".to_string(),
                },
            )
            .unwrap();

        assert_eq!(row.id, "artifact-1");
        assert_eq!(row.sync_status, SyncStatus::Synced);
        assert_eq!(outbox_count(&db), 0);
        assert_eq!(dao.get_row("artifact-1").unwrap().title, "Remote Artifact");
    }

    #[test]
    fn create_not_found_does_not_fallback_local() {
        let db = test_db("artifact_create_not_found");
        let dao = Arc::new(MockArtifactDao::default());
        let repo = ArtifactRepo::new(
            Arc::new(MockArtifactApi {
                create: ArtifactApiBehavior::Err(ApiErrorKind::NotFound),
                rename: ArtifactApiBehavior::Err(ApiErrorKind::Unknown),
            }),
            dao.clone(),
            Arc::new(SqliteBrowserDao),
            Arc::new(SqliteOutboxDao),
        );

        let result = repo.create(
            &db,
            &DataEventHub::default(),
            Some(&sync_config()),
            MutationMode::UserAction,
            ArtifactLocalMutationInput {
                id: "temp-1".to_string(),
                folder_id: None,
                r#type: Some("page".to_string()),
                title: "Missing".to_string(),
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
    fn create_missing_type_fails_without_outbox() {
        let db = test_db("artifact_create_missing_type");
        let dao = Arc::new(MockArtifactDao::default());
        let repo = ArtifactRepo::new(
            Arc::new(MockArtifactApi {
                create: ArtifactApiBehavior::Err(ApiErrorKind::Unknown),
                rename: ArtifactApiBehavior::Err(ApiErrorKind::Unknown),
            }),
            dao.clone(),
            Arc::new(SqliteBrowserDao),
            Arc::new(SqliteOutboxDao),
        );

        let result = repo.create(
            &db,
            &DataEventHub::default(),
            Some(&sync_config()),
            MutationMode::UserAction,
            ArtifactLocalMutationInput {
                id: "temp-1".to_string(),
                folder_id: None,
                r#type: Some("".to_string()),
                title: "Missing".to_string(),
                content: None,
                summary: None,
                source_url: None,
                updated_at: 10,
                mutation_id: "mut-1".to_string(),
            },
        );

        assert!(matches!(result, Err(crate::db::DbError::InvalidInput(_))));
        assert_eq!(outbox_count(&db), 0);
        assert!(dao.get_row("temp-1").is_none());
    }

    #[test]
    fn create_transient_falls_back_local_and_enqueues_outbox() {
        let db = test_db("artifact_create_transient");
        let dao = Arc::new(MockArtifactDao::default());
        let repo = ArtifactRepo::new(
            Arc::new(MockArtifactApi {
                create: ArtifactApiBehavior::Err(ApiErrorKind::Server),
                rename: ArtifactApiBehavior::Err(ApiErrorKind::Unknown),
            }),
            dao.clone(),
            Arc::new(SqliteBrowserDao),
            Arc::new(SqliteOutboxDao),
        );

        let row = repo
            .create(
                &db,
                &DataEventHub::default(),
                Some(&sync_config()),
                MutationMode::UserAction,
                ArtifactLocalMutationInput {
                    id: "temp-1".to_string(),
                    folder_id: None,
                    r#type: Some("page".to_string()),
                    title: "Pending".to_string(),
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
    fn rename_remote_success_skips_outbox_and_keeps_version() {
        let db = test_db("artifact_rename_remote_success");
        let dao = Arc::new(MockArtifactDao::default());
        dao.insert(ArtifactRow {
            id: "artifact-1".to_string(),
            folder_id: None,
            title: "Old".to_string(),
            kind: "page".to_string(),
            content: Some("body".to_string()),
            source_url: None,
            resource_url: None,
            metadata_json: None,
            updated_at: 1,
            sync_status: SyncStatus::Synced,
            version: 3,
        });
        let repo = ArtifactRepo::new(
            Arc::new(MockArtifactApi {
                create: ArtifactApiBehavior::Err(ApiErrorKind::Unknown),
                rename: ArtifactApiBehavior::Ok(crate::api::artifacts::ArtifactCreateApiRecord {
                    artifact: crate::api::artifacts::ArtifactApiRecord {
                        id: "artifact-1".to_string(),
                        kind: "page".to_string(),
                        folder_id: None,
                        title: "New".to_string(),
                        content: "body".to_string(),
                        summary: None,
                        source_url: None,
                    },
                    node: crate::api::artifacts::BrowserNodeApiRecord {
                        id: "artifact-1".to_string(),
                        parent_id: None,
                        kind: "artifact".to_string(),
                        title: "New".to_string(),
                        artifact_id: Some("artifact-1".to_string()),
                        position: 1,
                    },
                }),
            }),
            dao.clone(),
            Arc::new(SqliteBrowserDao),
            Arc::new(SqliteOutboxDao),
        );

        let row = repo
            .rename(
                &db,
                &DataEventHub::default(),
                Some(&sync_config()),
                MutationMode::UserAction,
                ArtifactLocalMutationInput {
                    id: "artifact-1".to_string(),
                    folder_id: None,
                    r#type: None,
                    title: "New".to_string(),
                    content: None,
                    summary: None,
                    source_url: None,
                    updated_at: 10,
                    mutation_id: "mut-2".to_string(),
                },
            )
            .unwrap();

        assert_eq!(row.title, "New");
        assert_eq!(row.sync_status, SyncStatus::Synced);
        assert_eq!(row.version, 3);
        assert_eq!(outbox_count(&db), 0);
    }
}

use serde::Deserialize;
use tauri::State;

use crate::commands::nodes::{mirror_delete, mirror_upsert};
use crate::db::repo::artifacts::{
    self, ArtifactDeleteInput, ArtifactLocalMutationInput, ArtifactRepo, ArtifactRow,
};
use crate::db::repo::nodes::NodeRow;
use crate::db::repo::MutationMode;
use crate::db::{Db, DbResult};
use crate::events::DataEventHub;
use crate::sync::SyncHandle;

#[derive(Debug, Deserialize, Clone, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub struct LocalArtifactMutationCommand {
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

#[derive(Debug, Deserialize, Clone, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub struct LocalArtifactDeleteCommand {
    pub id: String,
    pub updated_at: i64,
    pub mutation_id: String,
}

impl From<LocalArtifactDeleteCommand> for ArtifactDeleteInput {
    fn from(value: LocalArtifactDeleteCommand) -> Self {
        Self {
            id: value.id,
            updated_at: value.updated_at,
            mutation_id: value.mutation_id,
        }
    }
}

impl From<LocalArtifactMutationCommand> for ArtifactLocalMutationInput {
    fn from(value: LocalArtifactMutationCommand) -> Self {
        Self {
            id: value.id,
            folder_id: value.folder_id,
            r#type: value.r#type,
            title: value.title,
            content: value.content,
            summary: value.summary,
            source_url: value.source_url,
            updated_at: value.updated_at,
            mutation_id: value.mutation_id,
        }
    }
}

#[tauri::command]
pub fn db_list_artifacts(db: State<'_, Db>) -> DbResult<Vec<artifacts::ArtifactRow>> {
    let repo = ArtifactRepo::default();
    db.with_conn(|conn| repo.list_artifacts(conn))
}

#[tauri::command]
pub fn db_get_artifact(db: State<'_, Db>, id: String) -> DbResult<Option<artifacts::ArtifactRow>> {
    let repo = ArtifactRepo::default();
    db.with_conn(|conn| repo.get_artifact(conn, &id))
}

#[tauri::command]
pub fn db_get_artifacts(
    db: State<'_, Db>,
    ids: Vec<String>,
) -> DbResult<Vec<artifacts::ArtifactRow>> {
    let repo = ArtifactRepo::default();
    db.with_conn(|conn| repo.get_artifacts(conn, &ids))
}

#[tauri::command]
pub fn db_upsert_artifact(db: State<'_, Db>, row: artifacts::ArtifactRow) -> DbResult<()> {
    let repo = ArtifactRepo::default();
    db.with_conn(|conn| repo.upsert_artifact(conn, &row))
}

#[tauri::command]
pub fn db_create_artifact(
    db: State<'_, Db>,
    events: State<'_, DataEventHub>,
    sync: State<'_, SyncHandle>,
    input: LocalArtifactMutationCommand,
) -> DbResult<artifacts::ArtifactRow> {
    let row = ArtifactRepo::default().create(
        &db,
        &events,
        sync.get().as_ref(),
        MutationMode::UserAction,
        input.into(),
    )?;
    mirror_upsert(&db, &events, node_row_from_artifact(&row))?;
    Ok(row)
}

#[tauri::command]
pub fn db_rename_artifact(
    db: State<'_, Db>,
    events: State<'_, DataEventHub>,
    sync: State<'_, SyncHandle>,
    input: LocalArtifactMutationCommand,
) -> DbResult<artifacts::ArtifactRow> {
    let row = ArtifactRepo::default().rename(
        &db,
        &events,
        sync.get().as_ref(),
        MutationMode::UserAction,
        input.into(),
    )?;
    mirror_upsert(&db, &events, node_row_from_artifact(&row))?;
    Ok(row)
}

#[tauri::command]
pub fn db_delete_artifact(
    db: State<'_, Db>,
    events: State<'_, DataEventHub>,
    sync: State<'_, SyncHandle>,
    input: LocalArtifactDeleteCommand,
) -> DbResult<()> {
    let id = input.id.clone();
    ArtifactRepo::default().delete(
        &db,
        &events,
        sync.get().as_ref(),
        MutationMode::UserAction,
        input.into(),
    )?;
    mirror_delete(&db, &events, &id)?;
    Ok(())
}

/// Mirrors an artifact mutation result into a unified `nodes` row. The node id
/// IS the artifact id in the new model. `summary` is left to the pull engine
/// (the server emits it as its own field); the optimistic local row carries the
/// other fields directly from the artifact write.
fn node_row_from_artifact(row: &ArtifactRow) -> NodeRow {
    NodeRow {
        id: row.id.clone(),
        kind: "artifact".to_string(),
        parent_id: row.folder_id.clone(),
        position: 0,
        title: Some(row.title.clone()),
        artifact_type: Some(row.kind.clone()),
        content: row.content.clone(),
        source_url: row.source_url.clone(),
        summary: None,
        metadata_json: row.metadata_json.clone(),
        updated_at: row.updated_at,
    }
}

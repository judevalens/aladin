use serde::Deserialize;
use tauri::State;

use crate::commands::nodes::{mirror_delete, mirror_rename, mirror_upsert};
use crate::db::repo::artifacts::ArtifactRow;
use crate::db::repo::browser::{
    BrowserDeleteInput, BrowserMutationInput, BrowserNodeCreateInput, BrowserNodeRow, BrowserRepo,
};
use crate::db::repo::nodes::NodeRow;
use crate::db::repo::MutationMode;
use crate::db::{Db, DbResult};
use crate::events::DataEventHub;
use crate::sync::SyncHandle;

#[derive(Debug, Deserialize, Clone, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub struct LocalBrowserMutationCommand {
    pub id: String,
    pub parent_id: Option<String>,
    pub title: String,
    pub updated_at: i64,
    pub mutation_id: String,
}

#[derive(Debug, Deserialize, Clone, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub struct LocalBrowserDeleteCommand {
    pub id: String,
    pub updated_at: i64,
    pub mutation_id: String,
}

impl From<LocalBrowserDeleteCommand> for BrowserDeleteInput {
    fn from(value: LocalBrowserDeleteCommand) -> Self {
        Self {
            id: value.id,
            updated_at: value.updated_at,
            mutation_id: value.mutation_id,
        }
    }
}

impl From<LocalBrowserMutationCommand> for BrowserMutationInput {
    fn from(value: LocalBrowserMutationCommand) -> Self {
        Self {
            id: value.id,
            parent_id: value.parent_id,
            title: value.title,
            updated_at: value.updated_at,
            mutation_id: value.mutation_id,
        }
    }
}

#[derive(Debug, Deserialize, Clone, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub struct LocalBrowserNodeCreateCommand {
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

impl From<LocalBrowserNodeCreateCommand> for BrowserNodeCreateInput {
    fn from(value: LocalBrowserNodeCreateCommand) -> Self {
        Self {
            id: value.id,
            parent_id: value.parent_id,
            kind: value.kind,
            title: value.title,
            artifact_type: value.artifact_type,
            content: value.content,
            summary: value.summary,
            source_url: value.source_url,
            updated_at: value.updated_at,
            mutation_id: value.mutation_id,
        }
    }
}

#[derive(Debug, Clone, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub struct BrowserCreateResult {
    pub node: BrowserNodeRow,
    pub artifact: Option<ArtifactRow>,
}

#[tauri::command]
pub fn db_list_browser_nodes(db: State<'_, Db>) -> DbResult<Vec<BrowserNodeRow>> {
    let repo = BrowserRepo::default();
    db.with_conn(|conn| repo.list_nodes(conn))
}

#[tauri::command]
pub fn db_get_browser_node(db: State<'_, Db>, id: String) -> DbResult<Option<BrowserNodeRow>> {
    let repo = BrowserRepo::default();
    db.with_conn(|conn| repo.get_node(conn, &id))
}

#[tauri::command]
pub fn db_upsert_browser_node(db: State<'_, Db>, row: BrowserNodeRow) -> DbResult<()> {
    let repo = BrowserRepo::default();
    db.with_conn(|conn| repo.upsert_node(conn, &row))
}

#[tauri::command]
pub fn db_create_browser_node(
    db: State<'_, Db>,
    events: State<'_, DataEventHub>,
    sync: State<'_, SyncHandle>,
    input: LocalBrowserNodeCreateCommand,
) -> DbResult<BrowserCreateResult> {
    let repo = BrowserRepo::default();
    // Capture the artifact fields before `input` is consumed, to mirror them
    // into the unified nodes model.
    let artifact_type = input.artifact_type.clone();
    let content = input.content.clone();
    let summary = input.summary.clone();
    let source_url = input.source_url.clone();
    let node = repo.create(
        &db,
        &events,
        sync.get().as_ref(),
        MutationMode::UserAction,
        input.into(),
    )?;
    let artifact = node
        .artifact_id
        .as_ref()
        .map(|artifact_id| db.with_conn(|conn| repo.get_artifact(conn, artifact_id)))
        .transpose()?
        .flatten();

    let is_artifact = node.kind.eq_ignore_ascii_case("artifact");
    mirror_upsert(
        &db,
        &events,
        NodeRow {
            id: node.id.clone(),
            kind: if is_artifact { "artifact" } else { "folder" }.to_string(),
            parent_id: node.parent_id.clone(),
            position: 0,
            title: Some(node.title.clone()),
            artifact_type: if is_artifact { artifact_type } else { None },
            content: if is_artifact { content } else { None },
            source_url: if is_artifact { source_url } else { None },
            summary: if is_artifact { summary } else { None },
            metadata_json: None,
            updated_at: node.updated_at,
        },
    )?;

    Ok(BrowserCreateResult { node, artifact })
}

#[tauri::command]
pub fn db_rename_browser_node(
    db: State<'_, Db>,
    events: State<'_, DataEventHub>,
    sync: State<'_, SyncHandle>,
    input: LocalBrowserMutationCommand,
) -> DbResult<BrowserNodeRow> {
    let row = BrowserRepo::default().rename(
        &db,
        &events,
        sync.get().as_ref(),
        MutationMode::UserAction,
        input.into(),
    )?;
    mirror_rename(
        &db,
        &events,
        &row.id,
        &row.kind,
        row.title.clone(),
        row.parent_id.clone(),
        row.updated_at,
    )?;
    Ok(row)
}

#[tauri::command]
pub fn db_delete_browser_node(
    db: State<'_, Db>,
    events: State<'_, DataEventHub>,
    sync: State<'_, SyncHandle>,
    input: LocalBrowserDeleteCommand,
) -> DbResult<()> {
    let id = input.id.clone();
    BrowserRepo::default().delete(
        &db,
        &events,
        sync.get().as_ref(),
        MutationMode::UserAction,
        input.into(),
    )?;
    mirror_delete(&db, &events, &id)?;
    Ok(())
}

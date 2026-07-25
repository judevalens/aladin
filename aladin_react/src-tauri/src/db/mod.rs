use std::path::PathBuf;
use std::sync::{Arc, Mutex};

use rusqlite::{Connection, Transaction};
use thiserror::Error;

pub mod repo;
mod schema;

#[derive(Debug, Error)]
pub enum DbError {
    #[error(transparent)]
    Sqlite(#[from] rusqlite::Error),
    #[error(transparent)]
    Serde(#[from] serde_json::Error),
    #[error(transparent)]
    Api(#[from] crate::api::ApiError),
    #[error("db not initialized")]
    NotInitialized,
}

impl serde::Serialize for DbError {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        serializer.serialize_str(&self.to_string())
    }
}

pub type DbResult<T> = Result<T, DbError>;

#[derive(Clone)]
pub struct Db {
    conn: Arc<Mutex<Connection>>,
}

impl Db {
    pub fn open(path: PathBuf) -> DbResult<Self> {
        if let Some(parent) = path.parent() {
            std::fs::create_dir_all(parent).ok();
        }
        let mut conn = Connection::open(path)?;
        conn.execute_batch("PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;")?;
        schema::migrate(&mut conn)?;
        Ok(Self {
            conn: Arc::new(Mutex::new(conn)),
        })
    }

    pub fn with_conn<R>(&self, f: impl FnOnce(&Connection) -> DbResult<R>) -> DbResult<R> {
        let guard = self.conn.lock().map_err(|_| DbError::NotInitialized)?;
        f(&guard)
    }

    pub fn with_tx<R>(&self, f: impl FnOnce(&Transaction<'_>) -> DbResult<R>) -> DbResult<R> {
        let mut guard = self.conn.lock().map_err(|_| DbError::NotInitialized)?;
        let tx = guard.transaction()?;
        let result = f(&tx)?;
        tx.commit()?;
        Ok(result)
    }
}

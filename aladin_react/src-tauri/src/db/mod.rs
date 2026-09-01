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

    /// Opens the cache, recovering from an unusable file. The local SQLite is a rebuildable,
    /// server-authoritative read replica — it re-syncs from the server on the next pull — so if
    /// open/migrate fails (corruption from a power loss mid-write, a disk fault, or a bad
    /// half-applied schema), deleting and recreating it is always safe and beats bricking the app
    /// at launch (the old behavior: `.expect()` → panic → the app won't start until a terminal
    /// `make nuke-local-db`). Recovers ONCE; a second failure (e.g. no disk) is a real error.
    pub fn open_or_recover(path: PathBuf) -> DbResult<Self> {
        match Self::open(path.clone()) {
            Ok(db) => Ok(db),
            Err(err) => {
                eprintln!(
                    "[db] local cache unusable ({err}); recreating it (will re-sync from server)"
                );
                // Remove the db and its WAL/SHM sidecars so a truly corrupt file can't survive.
                let _ = std::fs::remove_file(&path);
                let _ = std::fs::remove_file(path.with_extension("sqlite-wal"));
                let _ = std::fs::remove_file(path.with_extension("sqlite-shm"));
                Self::open(path)
            }
        }
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

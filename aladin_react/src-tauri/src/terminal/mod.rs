use std::collections::HashMap;
use std::io::{Read, Write};
use std::sync::{Arc, Mutex};

use portable_pty::{native_pty_system, Child, CommandBuilder, MasterPty, PtySize};
use serde::Serialize;
use tauri::ipc::{Channel, Response};
use thiserror::Error;

/// Embedded terminal — a PTY-backed shell surface. Each session owns a native pty;
/// a dedicated reader thread streams stdout to the frontend over the per-session
/// `output` channel as RAW BYTES (Tauri sends these as an ArrayBuffer over the
/// binary IPC path — no base64/JSON on the hot path). A small `control` channel
/// carries the rare exit event as JSON. This mirrors the blocking `std::thread`
/// model used elsewhere in the shell — no async runtime. Sessions are ephemeral.
#[derive(Debug, Error)]
pub enum TerminalError {
    #[error("{0}")]
    Pty(String),
    #[error("terminal session not found")]
    NotFound,
    #[error("terminal manager unavailable")]
    Poisoned,
}

impl serde::Serialize for TerminalError {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        serializer.serialize_str(&self.to_string())
    }
}

pub type TerminalResult<T> = Result<T, TerminalError>;

/// Streamed down a session's `control` channel (low frequency, so JSON is fine).
#[derive(Debug, Clone, Serialize)]
#[serde(tag = "type", content = "payload", rename_all = "camelCase")]
pub enum TerminalControlEvent {
    /// The shell exited; the reader thread has torn the session down.
    Exit { code: Option<i32> },
}

/// Parameters for a new session (defaults resolved by the command layer).
pub struct OpenOptions {
    pub shell: String,
    pub cwd: String,
    pub cols: u16,
    pub rows: u16,
}

struct Session {
    master: Box<dyn MasterPty + Send>,
    writer: Box<dyn Write + Send>,
    child: Box<dyn Child + Send + Sync>,
}

#[derive(Default, Clone)]
pub struct TerminalManager {
    sessions: Arc<Mutex<HashMap<String, Session>>>,
}

impl TerminalManager {
    /// Spawn a shell on a fresh pty and stream its output down `output` (raw bytes).
    pub fn open(
        &self,
        id: String,
        output: Channel<Response>,
        control: Channel<TerminalControlEvent>,
        opts: OpenOptions,
    ) -> TerminalResult<()> {
        let pty_system = native_pty_system();
        let size = PtySize {
            rows: opts.rows,
            cols: opts.cols,
            pixel_width: 0,
            pixel_height: 0,
        };
        let pair = pty_system
            .openpty(size)
            .map_err(|e| TerminalError::Pty(e.to_string()))?;

        let mut cmd = CommandBuilder::new(&opts.shell);
        cmd.cwd(&opts.cwd);
        cmd.env("TERM", "xterm-256color");
        let child = pair
            .slave
            .spawn_command(cmd)
            .map_err(|e| TerminalError::Pty(e.to_string()))?;

        let mut reader = pair
            .master
            .try_clone_reader()
            .map_err(|e| TerminalError::Pty(e.to_string()))?;
        let writer = pair
            .master
            .take_writer()
            .map_err(|e| TerminalError::Pty(e.to_string()))?;
        // Drop the slave handle so the reader sees EOF once the child exits.
        drop(pair.slave);

        let sessions = self.sessions.clone();
        let reader_id = id.clone();
        std::thread::spawn(move || {
            let mut buf = [0u8; 8192];
            loop {
                match reader.read(&mut buf) {
                    Ok(0) => break,
                    Ok(n) => {
                        if output.send(Response::new(buf[..n].to_vec())).is_err() {
                            break;
                        }
                    }
                    Err(_) => break,
                }
            }
            let _ = control.send(TerminalControlEvent::Exit { code: None });
            if let Ok(mut map) = sessions.lock() {
                map.remove(&reader_id);
            }
        });

        let session = Session {
            master: pair.master,
            writer,
            child,
        };
        self.sessions
            .lock()
            .map_err(|_| TerminalError::Poisoned)?
            .insert(id, session);
        Ok(())
    }

    pub fn write(&self, id: &str, data: &[u8]) -> TerminalResult<()> {
        let mut map = self.sessions.lock().map_err(|_| TerminalError::Poisoned)?;
        let session = map.get_mut(id).ok_or(TerminalError::NotFound)?;
        session
            .writer
            .write_all(data)
            .map_err(|e| TerminalError::Pty(e.to_string()))?;
        session
            .writer
            .flush()
            .map_err(|e| TerminalError::Pty(e.to_string()))?;
        Ok(())
    }

    pub fn resize(&self, id: &str, cols: u16, rows: u16) -> TerminalResult<()> {
        let map = self.sessions.lock().map_err(|_| TerminalError::Poisoned)?;
        let session = map.get(id).ok_or(TerminalError::NotFound)?;
        session
            .master
            .resize(PtySize {
                rows,
                cols,
                pixel_width: 0,
                pixel_height: 0,
            })
            .map_err(|e| TerminalError::Pty(e.to_string()))?;
        Ok(())
    }

    pub fn close(&self, id: &str) -> TerminalResult<()> {
        let mut map = self.sessions.lock().map_err(|_| TerminalError::Poisoned)?;
        if let Some(mut session) = map.remove(id) {
            let _ = session.child.kill();
        }
        Ok(())
    }
}

use serde::Deserialize;
use tauri::{
    ipc::{Channel, Response},
    State,
};

use crate::terminal::{OpenOptions, TerminalControlEvent, TerminalManager, TerminalResult};

// Embedded terminal commands (thin adapters over TerminalManager). The frontend
// mints the session id (crypto.randomUUID) and opens a Channel per session; Rust
// spawns the pty and streams output back down that channel.

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TerminalOpenInput {
    pub cols: u16,
    pub rows: u16,
    pub cwd: Option<String>,
    pub shell: Option<String>,
}

#[tauri::command]
pub fn terminal_open(
    manager: State<'_, TerminalManager>,
    id: String,
    output: Channel<Response>,
    control: Channel<TerminalControlEvent>,
    input: TerminalOpenInput,
) -> TerminalResult<()> {
    let shell = input
        .shell
        .filter(|s| !s.is_empty())
        .or_else(|| std::env::var("SHELL").ok())
        .filter(|s| !s.is_empty())
        .unwrap_or_else(|| "/bin/zsh".to_string());
    let cwd = input
        .cwd
        .filter(|s| !s.is_empty())
        .or_else(|| std::env::var("HOME").ok())
        .filter(|s| !s.is_empty())
        .unwrap_or_else(|| "/".to_string());
    manager.open(
        id,
        output,
        control,
        OpenOptions {
            shell,
            cwd,
            cols: input.cols,
            rows: input.rows,
        },
    )
}

#[tauri::command]
pub fn terminal_write(
    manager: State<'_, TerminalManager>,
    id: String,
    data: String,
) -> TerminalResult<()> {
    manager.write(&id, data.as_bytes())
}

#[tauri::command]
pub fn terminal_resize(
    manager: State<'_, TerminalManager>,
    id: String,
    cols: u16,
    rows: u16,
) -> TerminalResult<()> {
    manager.resize(&id, cols, rows)
}

#[tauri::command]
pub fn terminal_close(manager: State<'_, TerminalManager>, id: String) -> TerminalResult<()> {
    manager.close(&id)
}

//! Local cache for Doc Surface vendored dependencies.
//!
//! Serves `vendor://deps/<sha>` from a content-addressed local cache. On a miss
//! it fetches the dep ONCE from the backend's PUBLIC `/vendor/<sha>` route, writes
//! it to disk, and serves it. Because deps are content-addressed (sha256), a
//! cached file is immutable — safe to keep forever — and a changed dep is simply a
//! different sha (a different URL → an automatic miss → refetch).
//!
//! Net effect: a heavy dep (e.g. mermaid) is downloaded once ever, then loads from
//! local disk with zero network on every doc + app restart, even offline.

use std::fs;

use tauri::{AppHandle, Manager, Runtime, UriSchemeContext, UriSchemeResponder};

use crate::sync::SyncHandle;

/// Async `vendor://` scheme handler. The fetch-on-miss is blocking, so it runs on
/// a spawned thread and responds when done (never blocks the webview thread).
pub fn handle<R: Runtime>(
    ctx: UriSchemeContext<'_, R>,
    request: tauri::http::Request<Vec<u8>>,
    responder: UriSchemeResponder,
) {
    let app = ctx.app_handle().clone();
    let uri = request.uri().to_string();
    std::thread::spawn(move || {
        responder.respond(resolve(&app, &uri));
    });
}

fn resolve<R: Runtime>(app: &AppHandle<R>, uri: &str) -> tauri::http::Response<Vec<u8>> {
    let sha = match parse_sha(uri) {
        Some(s) => s,
        None => return reply(400, b"bad vendor uri".to_vec()),
    };
    let cache_dir = match app.path().app_cache_dir() {
        Ok(d) => d.join("vendor"),
        Err(_) => return reply(500, b"no cache dir".to_vec()),
    };
    let path = cache_dir.join(&sha);

    // 1. Cache hit — serve from local disk, zero network.
    if let Ok(bytes) = fs::read(&path) {
        return js(bytes);
    }

    // 2. Miss — fetch the public /vendor/<sha> from the backend once, then cache.
    let url = format!("{}/vendor/{}", backend_base(app).trim_end_matches('/'), sha);
    match reqwest::blocking::Client::new().get(&url).send() {
        Ok(resp) if resp.status().is_success() => match resp.bytes() {
            Ok(body) => {
                let _ = fs::create_dir_all(&cache_dir);
                let _ = fs::write(&path, &body); // best-effort; serve even if the write fails
                js(body.to_vec())
            }
            Err(_) => reply(502, b"vendor read failed".to_vec()),
        },
        Ok(resp) => reply(resp.status().as_u16(), b"vendor upstream error".to_vec()),
        Err(_) => reply(502, b"vendor fetch failed".to_vec()),
    }
}

/// Extract the hex sha from `vendor://deps/<sha>` (traversal-proof: hex only).
fn parse_sha(uri: &str) -> Option<String> {
    let tail = uri.rsplit('/').next().unwrap_or("");
    let sha = tail.split('?').next().unwrap_or(tail).split('#').next().unwrap_or(tail);
    if !sha.is_empty() && sha.chars().all(|c| c.is_ascii_hexdigit()) {
        Some(sha.to_string())
    } else {
        None
    }
}

/// The backend base URL the frontend set via `sync_set_session` (dev fallback).
fn backend_base<R: Runtime>(app: &AppHandle<R>) -> String {
    app.state::<SyncHandle>()
        .get()
        .map(|c| c.api_base_url)
        .filter(|u| !u.trim().is_empty())
        .unwrap_or_else(|| "http://localhost:8000".to_string())
}

fn js(bytes: Vec<u8>) -> tauri::http::Response<Vec<u8>> {
    tauri::http::Response::builder()
        .status(200)
        .header("Content-Type", "text/javascript")
        .header("Access-Control-Allow-Origin", "*")
        .header("Cache-Control", "public, max-age=31536000, immutable")
        .body(bytes)
        .expect("vendor response")
}

fn reply(status: u16, body: Vec<u8>) -> tauri::http::Response<Vec<u8>> {
    tauri::http::Response::builder()
        .status(status)
        .header("Access-Control-Allow-Origin", "*")
        .body(body)
        .expect("vendor error response")
}

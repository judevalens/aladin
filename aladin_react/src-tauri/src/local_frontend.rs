use std::{
    collections::BTreeMap,
    io,
    net::{Ipv4Addr, SocketAddr},
    sync::{
        atomic::{AtomicBool, Ordering},
        Arc, Mutex,
    },
    thread::{self, JoinHandle},
};

use tauri::{ipc::CapabilityBuilder, Manager, WebviewUrl, WebviewWindowBuilder};
use tiny_http::{Header, Method, Request, Response, Server, StatusCode};

const BOOTSTRAP_LABEL: &str = "local-frontend-bootstrap";
const STORAGE_SCRIPT: &str = include_str!("local_frontend_storage.js");

pub struct LocalFrontend {
    server: LoopbackServer,
    bootstrapped: Mutex<bool>,
}

// Stable per app identity: changing ports would change localStorage/IndexedDB origins.
fn frontend_port(identifier: &str) -> u16 {
    match identifier {
        "com.aladin.app" => 4174,
        "com.aladin.react" => 4175,
        _ => {
            let hash = identifier.bytes().fold(2166136261u32, |hash, byte| {
                (hash ^ u32::from(byte)).wrapping_mul(16777619)
            });
            45000 + (hash % 10000) as u16
        }
    }
}

pub fn open(app: &mut tauri::App) -> Result<(), Box<dyn std::error::Error>> {
    let config = app
        .config()
        .app
        .windows
        .iter()
        .find(|w| w.label == "main")
        .ok_or("main window configuration missing")?
        .clone();
    if cfg!(dev) || cfg!(mobile) {
        WebviewWindowBuilder::from_config(app, &config)?.build()?;
        return Ok(());
    }

    let resolver = app.asset_resolver();
    let server = LoopbackServer::start(frontend_port(&app.config().identifier), move |path| {
        resolver
            .get(path.to_owned())
            .map(|asset| (asset.bytes, asset.mime_type))
    })?;
    let origin = server.origin();
    app.add_capability(
        CapabilityBuilder::new("local-frontend")
            .local(false)
            .window("main")
            .remote(format!("{origin}/*"))
            .permission("core:default")
            .permission("desktop-commands"),
    )?;
    app.add_capability(
        CapabilityBuilder::new("legacy-storage-bootstrap")
            .window(BOOTSTRAP_LABEL)
            .permission("bootstrap-local-frontend"),
    )?;
    app.manage(LocalFrontend {
        server,
        bootstrapped: Mutex::new(false),
    });

    // Read storage inside its original webview origin, never from WebKit's files.
    let mut bootstrap = config;
    bootstrap.label = BOOTSTRAP_LABEL.into();
    bootstrap.url = WebviewUrl::App("desktop-bootstrap.html".into());
    WebviewWindowBuilder::from_config(app, &bootstrap)?
        .initialization_script(format!(r#"(() => {{
            if (window.top !== window) return;
            {STORAGE_SCRIPT}
            window.addEventListener('DOMContentLoaded', async () => {{
                try {{
                    const entries = storageMigration.collect(window.localStorage);
                    await window.__TAURI_INTERNALS__.invoke('finish_local_frontend_bootstrap', {{ entries }});
                }} catch {{
                    document.getElementById('status').textContent = 'Aladin could not start. Quit and reopen the app to retry.';
                }}
            }}, {{ once: true }});
        }})();"#))
        .build()?;
    Ok(())
}

#[tauri::command]
pub async fn finish_local_frontend_bootstrap(
    app: tauri::AppHandle,
    window: tauri::WebviewWindow,
    state: tauri::State<'_, LocalFrontend>,
    entries: BTreeMap<String, String>,
) -> Result<(), String> {
    if window.label() != BOOTSTRAP_LABEL {
        return Err("not the startup window".into());
    }
    validate_entries(&entries)?;
    let mut done = state
        .bootstrapped
        .lock()
        .map_err(|_| "startup lock failed")?;
    if *done {
        return Err("startup already completed".into());
    }
    let origin = state.server.origin();
    let mut config = app
        .config()
        .app
        .windows
        .iter()
        .find(|w| w.label == "main")
        .ok_or("main window configuration missing")?
        .clone();
    config.url = WebviewUrl::External(origin.parse().map_err(|_| "invalid frontend URL")?);
    let entries = serde_json::to_string(&entries).map_err(|_| "invalid preferences")?;
    WebviewWindowBuilder::from_config(&app, &config)
        .map_err(|e| e.to_string())?
        .initialization_script(format!(
            r#"(() => {{
            if (window.top !== window || window.location.origin !== {origin:?}) return;
            {STORAGE_SCRIPT}
            storageMigration.restore(window.localStorage, {entries});
        }})();"#
        ))
        .build()
        .map_err(|e| e.to_string())?;
    *done = true;
    window.close().map_err(|e| e.to_string())
}

fn validate_entries(entries: &BTreeMap<String, String>) -> Result<(), String> {
    if entries.len() > 128
        || entries.iter().any(|(key, _)| !key.starts_with("aladin."))
        || entries
            .iter()
            .map(|(key, value)| key.len() + value.len())
            .sum::<usize>()
            > 2 * 1024 * 1024
    {
        return Err("invalid startup preferences".into());
    }
    Ok(())
}

struct LoopbackServer {
    address: SocketAddr,
    server: Arc<Server>,
    stopped: Arc<AtomicBool>,
    thread: Option<JoinHandle<()>>,
}

impl LoopbackServer {
    fn start<F>(port: u16, resolve: F) -> io::Result<Self>
    where
        F: Fn(&str) -> Option<(Vec<u8>, String)> + Send + 'static,
    {
        // Bind before creating a webview. Never load whatever already owns the port.
        let server = Arc::new(Server::http((Ipv4Addr::LOCALHOST, port)).map_err(|error| {
            io::Error::new(io::ErrorKind::AddrInUse,
                format!("Cannot start Aladin's local frontend on 127.0.0.1:{port}: {error}. Close the other app using this port and retry."))
        })?);
        let address = server
            .server_addr()
            .to_ip()
            .ok_or_else(|| io::Error::other("missing frontend address"))?;
        let stopped = Arc::new(AtomicBool::new(false));
        let running = stopped.clone();
        let requests = server.clone();
        let thread = thread::Builder::new()
            .name("local-frontend".into())
            .spawn(move || {
                for request in requests.incoming_requests() {
                    if running.load(Ordering::Acquire) {
                        break;
                    }
                    let response = serve(&request, address, &resolve);
                    let _ = request.respond(response);
                }
            })?;
        Ok(Self {
            address,
            server,
            stopped,
            thread: Some(thread),
        })
    }

    fn origin(&self) -> String {
        format!("http://{}", self.address)
    }
}

impl Drop for LoopbackServer {
    fn drop(&mut self) {
        self.stopped.store(true, Ordering::Release);
        self.server.unblock();
        if let Some(thread) = self.thread.take() {
            let _ = thread.join();
        }
    }
}

fn serve<F>(request: &Request, address: SocketAddr, resolve: &F) -> Response<io::Cursor<Vec<u8>>>
where
    F: Fn(&str) -> Option<(Vec<u8>, String)>,
{
    let header = |name: &'static str| {
        request
            .headers()
            .iter()
            .find(|header| header.field.equiv(name))
            .map(|header| header.value.as_str())
    };
    let host = address.to_string();
    let origin = format!("http://{host}");
    let rejected = header("Host") != Some(host.as_str())
        || header("Origin").is_some_and(|value| value != origin)
        || header("Sec-Fetch-Site") == Some("cross-site");
    let (status, bytes, mime) = if rejected {
        (403, b"Forbidden".to_vec(), "text/plain".to_owned())
    } else if !matches!(request.method(), Method::Get | Method::Head) {
        (405, b"Method not allowed".to_vec(), "text/plain".to_owned())
    } else {
        let path = request.url().split('?').next().unwrap_or("/");
        match resolve(path) {
            Some((bytes, mime)) => (200, bytes, mime),
            None => (404, b"Not found".to_vec(), "text/plain".to_owned()),
        }
    };
    let mut response = Response::from_data(bytes).with_status_code(StatusCode(status));
    for (key, value) in [
        ("Content-Type", mime.as_str()),
        ("Cache-Control", "no-store"),
        ("X-Content-Type-Options", "nosniff"),
        ("X-Frame-Options", "DENY"),
        ("Content-Security-Policy", "frame-ancestors 'none'"),
        ("Referrer-Policy", "no-referrer"),
    ] {
        response.add_header(Header::from_bytes(key, value).expect("valid static header"));
    }
    response
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn origins_are_stable_and_separate_by_app_identity() {
        assert_eq!(frontend_port("com.aladin.app"), 4174);
        assert_eq!(frontend_port("com.aladin.react"), 4175);
        assert_eq!(
            frontend_port("com.aladin.react.b"),
            frontend_port("com.aladin.react.b")
        );
        assert_ne!(frontend_port("com.aladin.react.b"), 4174);
    }

    #[test]
    fn migration_only_accepts_bounded_app_storage() {
        assert!(
            validate_entries(&BTreeMap::from([("aladin.theme".into(), "dark".into())])).is_ok()
        );
        assert!(validate_entries(&BTreeMap::from([("foreign".into(), "value".into())])).is_err());
        assert!(validate_entries(&BTreeMap::from([(
            "aladin.large".into(),
            "x".repeat(2 * 1024 * 1024)
        )]))
        .is_err());
    }

    #[test]
    fn loopback_serves_assets_and_rejects_untrusted_requests() {
        let server = LoopbackServer::start(0, |path| match path {
            "/" => Some((b"<html>Aladin</html>".to_vec(), "text/html".into())),
            "/app.js" => Some((b"export {};".to_vec(), "text/javascript".into())),
            _ => None,
        })
        .unwrap();
        assert!(server.address.ip().is_loopback());
        let client = reqwest::blocking::Client::new();
        let url = server.origin();
        let response = client.get(&url).send().unwrap();
        assert_eq!(response.status(), 200);
        assert_eq!(response.headers()["x-frame-options"], "DENY");
        assert!(response
            .headers()
            .get("access-control-allow-origin")
            .is_none());
        assert_eq!(response.text().unwrap(), "<html>Aladin</html>");
        let script = client.get(format!("{url}/app.js?v=1")).send().unwrap();
        assert_eq!(script.headers()["content-type"], "text/javascript");
        assert_eq!(client.head(&url).send().unwrap().text().unwrap(), "");
        assert_eq!(
            client
                .get(format!("{url}/missing"))
                .send()
                .unwrap()
                .status(),
            404
        );
        assert_eq!(client.post(&url).send().unwrap().status(), 405);
        assert_eq!(
            client
                .get(&url)
                .header("Host", "attacker.example")
                .send()
                .unwrap()
                .status(),
            403
        );
        assert_eq!(
            client
                .get(&url)
                .header("Origin", "null")
                .send()
                .unwrap()
                .status(),
            403
        );
        assert_eq!(
            client
                .get(&url)
                .header("Origin", "https://attacker.example")
                .send()
                .unwrap()
                .status(),
            403
        );
        assert_eq!(
            client
                .get(&url)
                .header("Sec-Fetch-Site", "cross-site")
                .send()
                .unwrap()
                .status(),
            403
        );
        assert_eq!(
            client
                .get(&url)
                .header("Origin", &url)
                .send()
                .unwrap()
                .status(),
            200
        );
        assert!(LoopbackServer::start(server.address.port(), |_| None).is_err());
    }
}

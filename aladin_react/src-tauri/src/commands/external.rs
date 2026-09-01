fn validated_web_url(raw: &str) -> Result<String, String> {
    if raw.chars().any(char::is_control) {
        return Err("invalid source URL".into());
    }
    let url = reqwest::Url::parse(raw).map_err(|_| "invalid source URL")?;
    if !matches!(url.scheme(), "https" | "http")
        || url.host_str().is_none()
        || !url.username().is_empty()
        || url.password().is_some()
    {
        return Err("only http(s) source URLs are allowed".into());
    }
    Ok(url.into())
}

// Pass the validated URL as one argument to the OS opener, never through a shell.
#[cfg(any(target_os = "macos", target_os = "windows", target_os = "linux"))]
fn launch_url(url: &str) -> Result<(), String> {
    #[cfg(target_os = "macos")]
    let mut command = std::process::Command::new("/usr/bin/open");
    #[cfg(target_os = "linux")]
    let mut command = std::process::Command::new("xdg-open");
    #[cfg(target_os = "windows")]
    let mut command = {
        let mut command = std::process::Command::new("rundll32.exe");
        command.arg("url.dll,FileProtocolHandler");
        command
    };
    let status = command
        .arg(url)
        .status()
        .map_err(|_| "could not launch browser")?;
    if status.success() {
        Ok(())
    } else {
        Err("browser refused the source URL".into())
    }
}

#[cfg(not(any(target_os = "macos", target_os = "windows", target_os = "linux")))]
fn launch_url(_url: &str) -> Result<(), String> {
    Err("external browsing is not supported by this host".into())
}

#[tauri::command]
pub async fn open_external_url(window: tauri::WebviewWindow, url: String) -> Result<(), String> {
    if window.label() != "main" {
        return Err("not the main window".into());
    }
    let url = validated_web_url(&url)?;
    tauri::async_runtime::spawn_blocking(move || launch_url(&url))
        .await
        .map_err(|_| "could not open source".to_string())?
}

#[cfg(test)]
mod tests {
    use super::validated_web_url;

    #[test]
    fn source_urls_preserve_watch_positions_and_queries() {
        let url = "https://youtu.be/jNQXAC9IVRw?t=2&feature=share#source";
        assert_eq!(validated_web_url(url).unwrap(), url);
    }

    #[test]
    fn source_urls_reject_non_web_schemes_and_credentials() {
        for url in [
            "file:///etc/passwd",
            "javascript:alert(1)",
            "data:text/html,bad",
            "mailto:a@example.com",
            "https://user:pass@example.com",
            "--help",
            "https://example.com\n--help",
        ] {
            assert!(validated_web_url(url).is_err(), "accepted {url}");
        }
    }
}

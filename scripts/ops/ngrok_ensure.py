#!/usr/bin/env python3
"""Ensure a local ngrok HTTP tunnel exists for Aladin webhooks."""

from __future__ import annotations

import argparse
import json
import os
import shutil
import subprocess
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path

from read_env_keys import parse_env


ROOT = Path(__file__).resolve().parents[2]
LOG_DIR = ROOT / "logs"
NGROK_API = "http://127.0.0.1:4040/api/tunnels"


def env_flag(values: dict[str, str], key: str, default: bool) -> bool:
    raw = values.get(key, os.environ.get(key, "")).strip().lower()
    if raw in {"1", "true", "yes", "on"}:
        return True
    if raw in {"0", "false", "no", "off"}:
        return False
    return default


def fetch_tunnels() -> list[dict[str, object]]:
    try:
        with urllib.request.urlopen(NGROK_API, timeout=1.5) as response:
            payload = json.loads(response.read().decode("utf-8"))
    except (urllib.error.URLError, TimeoutError, json.JSONDecodeError):
        return []
    tunnels = payload.get("tunnels", [])
    if isinstance(tunnels, list):
        return [t for t in tunnels if isinstance(t, dict)]
    return []


def tunnel_for_port(tunnels: list[dict[str, object]], port: int) -> str | None:
    target = f"localhost:{port}"
    for tunnel in tunnels:
        public_url = str(tunnel.get("public_url", ""))
        config = tunnel.get("config", {})
        addr = str(config.get("addr", "")) if isinstance(config, dict) else ""
        if public_url.startswith("https://") and (addr.endswith(target) or addr.endswith(f"127.0.0.1:{port}")):
            return public_url
    for tunnel in tunnels:
        public_url = str(tunnel.get("public_url", ""))
        if public_url.startswith("https://"):
            return public_url
    return None


def start_ngrok(port: int) -> None:
    LOG_DIR.mkdir(parents=True, exist_ok=True)
    log_file = open(LOG_DIR / "ngrok.log", "ab")
    subprocess.Popen(
        ["ngrok", "http", str(port), "--log", "stdout"],
        stdout=log_file,
        stderr=subprocess.STDOUT,
        cwd=ROOT,
        start_new_session=True,
    )


def print_webhook_url(public_url: str, signing_key_present: bool) -> None:
    webhook_url = f"{public_url.rstrip('/')}/api/provider-connections/nango/webhook"
    print(f"ngrok: webhook URL -> {webhook_url}")
    if not signing_key_present:
        print("ngrok: NANGO_WEBHOOK_SIGNING_KEY is missing in backend_v2/.env")
    print("ngrok: configure this URL in Nango Environment Settings > Webhooks")


def main() -> int:
    parser = argparse.ArgumentParser(description="Ensure an ngrok tunnel for Aladin local webhooks.")
    parser.add_argument("--env", default="backend_v2/.env", help="Path to .env file.")
    parser.add_argument("--port", type=int, default=8000, help="Local backend port to expose.")
    args = parser.parse_args()

    env_path = Path(args.env)
    values = parse_env(env_path) if env_path.exists() else {}
    if not env_flag(values, "NGROK_AUTOSTART", True):
        print("ngrok: autostart disabled by NGROK_AUTOSTART=0")
        return 0

    if shutil.which("ngrok") is None:
        print("ngrok: command not found; skipping tunnel startup")
        return 0

    tunnels = fetch_tunnels()
    public_url = tunnel_for_port(tunnels, args.port)
    if public_url:
        print_webhook_url(public_url, "NANGO_WEBHOOK_SIGNING_KEY" in values)
        return 0

    print(f"ngrok: starting HTTP tunnel to localhost:{args.port}")
    start_ngrok(args.port)
    for _ in range(20):
        time.sleep(0.25)
        public_url = tunnel_for_port(fetch_tunnels(), args.port)
        if public_url:
            print_webhook_url(public_url, "NANGO_WEBHOOK_SIGNING_KEY" in values)
            return 0

    print("ngrok: started, but tunnel URL is not available yet; check logs/ngrok.log")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

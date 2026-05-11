#!/usr/bin/env python3
"""Print selected .env keys as shell-safe exports."""

from __future__ import annotations

import argparse
import shlex
from pathlib import Path


DEFAULT_KEYS = [
    "NANGO_ENCRYPTION_KEY",
    "NANGO_SECRET_KEY",
    "NANGO_BASE_URL",
    "NANGO_CONNECT_BASE_URL",
    "NANGO_GOOGLE_PROVIDER_CONFIG_KEY",
    "NANGO_WEBHOOK_SIGNING_KEY",
    "NGROK_AUTOSTART",
]


def parse_env(path: Path) -> dict[str, str]:
    values: dict[str, str] = {}
    for raw_line in path.read_text().splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue
        if line.startswith("export "):
            line = line[len("export ") :].strip()
        if "=" not in line:
            continue
        key, value = line.split("=", 1)
        key = key.strip()
        value = value.strip()
        if not key:
            continue
        if len(value) >= 2 and value[0] == value[-1] and value[0] in {"'", '"'}:
            value = value[1:-1]
        values[key] = value
    return values


def main() -> int:
    parser = argparse.ArgumentParser(description="Read selected keys from a .env file.")
    parser.add_argument("--env", default="backend_v2/.env", help="Path to .env file.")
    parser.add_argument(
        "--key",
        action="append",
        dest="keys",
        help="Key to print. Can be provided multiple times. Defaults to Nango keys.",
    )
    parser.add_argument(
        "--missing",
        choices=["skip", "empty"],
        default="skip",
        help="How to handle missing keys.",
    )
    args = parser.parse_args()

    env_path = Path(args.env)
    if not env_path.exists():
        raise SystemExit(f"env file not found: {env_path}")

    values = parse_env(env_path)
    keys = args.keys or DEFAULT_KEYS
    for key in keys:
        if key in values:
            print(f"export {key}={shlex.quote(values[key])}")
        elif args.missing == "empty":
            print(f"export {key}=''")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
"""Local Aladin ops harness.

This is intentionally a local-dev tool. It reads local environment config,
queries local services, and provides safe recovery commands for stream syncing.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parents[2]
BACKEND_DIR = ROOT / "backend_v2"
ENV_PATH = BACKEND_DIR / ".env"
DEFAULT_API_URL = "http://localhost:8000"
DEFAULT_LOKI_URL = "http://localhost:3100"


@dataclass
class CommandResult:
    returncode: int
    stdout: str
    stderr: str


def main() -> int:
    parser = argparse.ArgumentParser(prog="aladin-ops", description="Local Aladin ops harness")
    sub = parser.add_subparsers(dest="command", required=True)

    sub.add_parser("status", help="Show a full local ops dashboard")

    errors = sub.add_parser("errors", help="Show recent Loki errors and warnings")
    errors.add_argument("--window", default="15m")
    errors.add_argument("--limit", type=int, default=5)

    sub.add_parser("streams", help="Show provider stream status")
    sub.add_parser("queues", help="Show best-effort Asynq/Redis queue state")

    force = sub.add_parser("force-stream", help="Mark one provider stream due")
    force.add_argument("--provider", required=True)
    force.add_argument("--stream-key", required=True)

    reset = sub.add_parser("reset-stuck-cycles", help="Close stale active/running sync cycles")
    reset.add_argument("--age", default="30m")

    args = parser.parse_args()
    env = load_env()
    ctx = {
        "env": env,
        "database_url": env.get("DATABASE_URL", ""),
        "redis_url": env.get("REDIS_URL", ""),
        "api_url": env.get("ALADIN_API_URL", DEFAULT_API_URL).rstrip("/"),
        "loki_url": env.get("LOKI_URL", DEFAULT_LOKI_URL).rstrip("/"),
    }

    if args.command == "status":
        return status(ctx)
    if args.command == "errors":
        return errors_report(ctx, args.window, args.limit)
    if args.command == "streams":
        return streams_report(ctx)
    if args.command == "queues":
        return queues_report(ctx)
    if args.command == "force-stream":
        return force_stream(ctx, args.provider, args.stream_key)
    if args.command == "reset-stuck-cycles":
        return reset_stuck_cycles(ctx, args.age)
    return 1


def load_env() -> dict[str, str]:
    env = dict(os.environ)
    if not ENV_PATH.exists():
        return env
    for raw_line in ENV_PATH.read_text().splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        key = key.strip()
        value = value.strip().strip('"').strip("'")
        env.setdefault(key, value)
    return env


def status(ctx: dict[str, Any]) -> int:
    print_title("Aladin Local Ops")
    infra_report()
    api_report(ctx)
    process_report()
    db_report(ctx)
    streams_report(ctx, heading="Provider Streams")
    queues_report(ctx, heading="Queues")
    errors_report(ctx, "15m", 5, heading="Recent Logs")
    return 0


def infra_report() -> None:
    print_section("Infra")
    if not require_cmd("docker"):
        print_warn("docker not found; skipping compose status")
        return
    result = run(["docker", "compose", "ps", "--format", "json"], cwd=ROOT)
    if result.returncode != 0:
        print_warn("docker compose ps unavailable")
        if result.stderr:
            print_dim(result.stderr.strip())
        return

    rows = []
    for line in result.stdout.splitlines():
        if not line.strip():
            continue
        try:
            item = json.loads(line)
        except json.JSONDecodeError:
            continue
        rows.append([
            item.get("Name", ""),
            item.get("Service", ""),
            item.get("State", ""),
            item.get("Publishers", "") or item.get("Ports", ""),
        ])
    if not rows:
        print(result.stdout.strip() or "No compose services reported.")
        return
    print_table(["name", "service", "state", "ports"], rows)


def api_report(ctx: dict[str, Any]) -> None:
    print_section("API")
    api_url = ctx["api_url"]
    ready = http_json(f"{api_url}/readyz")
    if ready is None:
        print_warn(f"API unavailable at {api_url}")
        return
    print_kv("readyz", compact_json(ready))
    status_payload = http_json(f"{api_url}/api/worker/status")
    if status_payload is None:
        print_warn("/api/worker/status unavailable")
    else:
        print_kv("worker status", compact_json(status_payload))


def process_report() -> None:
    print_section("Processes")
    if not require_cmd("pgrep"):
        print_warn("pgrep not found; skipping process checks")
        return
    checks = [
        ("api", "go run ./cmd/api|cmd/api"),
        ("worker", "go run ./cmd/worker|cmd/worker"),
    ]
    rows = []
    for name, pattern in checks:
        result = run(["pgrep", "-fl", pattern])
        if result.returncode == 0 and result.stdout.strip():
            rows.append([name, "running", one_line(result.stdout)])
        else:
            rows.append([name, "not found", ""])
    print_table(["process", "state", "match"], rows)


def db_report(ctx: dict[str, Any]) -> None:
    print_section("Database")
    if not ctx["database_url"]:
        print_warn("DATABASE_URL missing; skipping DB checks")
        return
    count_sql = """
        SELECT 'provider_streams', COUNT(*) FROM provider_streams
        UNION ALL SELECT 'records', COUNT(*) FROM records
        UNION ALL SELECT 'records_enriched', COUNT(*) FROM records WHERE enrichment IS NOT NULL AND enrichment <> '{}'::jsonb
        UNION ALL SELECT 'records_with_key_claims', COUNT(*) FROM records WHERE enrichment ? 'key_claims'
        UNION ALL SELECT 'records_with_search_context', COUNT(*) FROM records WHERE enrichment ? 'search_context'
        UNION ALL SELECT 'source_subscriptions', COUNT(*) FROM source_subscriptions
        UNION ALL SELECT 'tenant_item_matches', COUNT(*) FROM tenant_item_matches
        UNION ALL SELECT 'sync_cycles_live', COUNT(*) FROM sync_cycles WHERE status IN ('active', 'running')
        ORDER BY 1;
    """
    rows = psql_rows(ctx, count_sql)
    if rows is None:
        return
    print_table(["table", "count"], rows)


def streams_report(ctx: dict[str, Any], heading: str = "Provider Streams") -> int:
    print_section(heading)
    if not ctx["database_url"]:
        print_warn("DATABASE_URL missing; skipping stream checks")
        return 0
    sql = """
        SELECT
            ps.provider,
            ps.stream_kind,
            ps.stream_key,
            ps.sync_status,
            COALESCE(to_char(ps.last_refresh_at, 'YYYY-MM-DD HH24:MI:SS'), 'never') AS last_refresh,
            COALESCE(ps.config->>'sync_interval_seconds', '10') AS interval_seconds,
            COUNT(sc.*) FILTER (WHERE sc.status IN ('active', 'running'))::text AS live_cycles
        FROM provider_streams ps
        LEFT JOIN sync_cycles sc ON sc.provider_stream_id = ps.id
        GROUP BY ps.id
        ORDER BY ps.provider, ps.stream_kind, ps.stream_key;
    """
    rows = psql_rows(ctx, sql)
    if rows is None:
        return 1
    if not rows:
        print("No provider streams.")
        return 0
    print_table(["provider", "kind", "stream_key", "status", "last_refresh", "interval_s", "live_cycles"], rows)
    return 0


def queues_report(ctx: dict[str, Any], heading: str = "Queues") -> int:
    print_section(heading)
    redis_url = ctx["redis_url"]
    if not redis_url:
        print_warn("REDIS_URL missing; skipping queue checks")
        return 0
    if not require_cmd("redis-cli"):
        print_warn("redis-cli not found; skipping queue checks")
        return 0

    keys = redis_scan(redis_url, "asynq:*")
    if keys is None:
        return 1
    rows = []
    for key in keys:
        if ":t:" in key:
            continue
        count = redis_key_count(redis_url, key)
        if count and count > 0:
            rows.append([key, str(count)])
    if not rows:
        print("No non-empty Asynq keys found.")
        return 0
    rows.sort(key=lambda row: row[0])
    print_table(["redis_key", "count"], rows[:80])
    if len(rows) > 80:
        print_dim(f"... {len(rows) - 80} more non-empty keys omitted")
    return 0


def errors_report(ctx: dict[str, Any], window: str, limit: int, heading: str = "Errors") -> int:
    print_section(f"{heading} ({window})")
    if not valid_window(window):
        print_error(f"Invalid window: {window}")
        return 1
    loki_url = ctx["loki_url"]
    error_query = 'sum by (service) (count_over_time({job="aladin"} |= "\\"level\\":\\"ERROR\\"" [' + window + "]))"
    warn_query = 'sum by (service) (count_over_time({job="aladin"} |= "\\"level\\":\\"WARN\\"" [' + window + "]))"
    error_counts = loki_instant(loki_url, error_query)
    warn_counts = loki_instant(loki_url, warn_query)
    if error_counts is None or warn_counts is None:
        print_warn(f"Loki unavailable at {loki_url}")
        return 0
    print_counts("errors", error_counts)
    print_counts("warnings", warn_counts)

    latest = loki_range(
        loki_url,
        '{job="aladin"} |= "\\"level\\":\\"ERROR\\""',
        window=window,
        limit=limit,
        direction="backward",
    )
    if not latest:
        print("No recent error lines.")
        return 0
    print()
    print("Latest errors:")
    for entry in latest:
        print(f"- {summarize_log_line(entry)}")
    return 0


def force_stream(ctx: dict[str, Any], provider: str, stream_key: str) -> int:
    print_section("Force Stream Due")
    if not ctx["database_url"]:
        print_error("DATABASE_URL missing")
        return 1
    sql = """
        UPDATE provider_streams
        SET last_refresh_at = NULL,
            sync_status = 'idle',
            last_picked_at = NULL
        WHERE provider = $OPS$%s$OPS$
          AND stream_key = $OPS$%s$OPS$;
    """ % (escape_dollar(provider), escape_dollar(stream_key))
    result = psql_exec(ctx, sql)
    if result is None:
        return 1
    print(result.strip() or "No output.")
    return 0


def reset_stuck_cycles(ctx: dict[str, Any], age: str) -> int:
    print_section("Reset Stuck Cycles")
    if not ctx["database_url"]:
        print_error("DATABASE_URL missing")
        return 1
    if not valid_window(age):
        print_error(f"Invalid age: {age}")
        return 1
    sql = """
        UPDATE sync_cycles
        SET status = 'closed',
            completed_at = now()
        WHERE status IN ('active', 'running')
          AND COALESCE(last_picked_at, created_at) < now() - interval '%s';
    """ % age
    result = psql_exec(ctx, sql)
    if result is None:
        return 1
    print(result.strip() or "No output.")
    return 0


def psql_rows(ctx: dict[str, Any], sql: str) -> list[list[str]] | None:
    if not require_cmd("psql"):
        print_warn("psql not found; skipping DB checks")
        return None
    result = run([
        "psql",
        ctx["database_url"],
        "-v",
        "ON_ERROR_STOP=1",
        "-At",
        "-F",
        "\t",
        "-c",
        sql,
    ])
    if result.returncode != 0:
        print_warn("psql query failed")
        print_dim(result.stderr.strip())
        return None
    rows = []
    for line in result.stdout.splitlines():
        if line.strip():
            rows.append(line.split("\t"))
    return rows


def psql_exec(ctx: dict[str, Any], sql: str) -> str | None:
    if not require_cmd("psql"):
        print_error("psql not found")
        return None
    result = run([
        "psql",
        ctx["database_url"],
        "-v",
        "ON_ERROR_STOP=1",
        "-c",
        sql,
    ])
    if result.returncode != 0:
        print_error("psql command failed")
        print_dim(result.stderr.strip())
        return None
    return result.stdout


def redis_scan(redis_url: str, pattern: str) -> list[str] | None:
    result = run(["redis-cli", "-u", redis_url, "--scan", "--pattern", pattern])
    if result.returncode != 0:
        print_warn("redis scan failed")
        print_dim(result.stderr.strip())
        return None
    return [line.strip() for line in result.stdout.splitlines() if line.strip()]


def redis_key_count(redis_url: str, key: str) -> int | None:
    type_result = run(["redis-cli", "-u", redis_url, "TYPE", key])
    if type_result.returncode != 0:
        return None
    key_type = type_result.stdout.strip()
    if key_type == "list":
        cmd = "LLEN"
    elif key_type == "zset":
        cmd = "ZCARD"
    elif key_type == "set":
        cmd = "SCARD"
    elif key_type == "stream":
        cmd = "XLEN"
    elif key_type == "hash":
        cmd = "HLEN"
    else:
        return None
    result = run(["redis-cli", "-u", redis_url, cmd, key])
    if result.returncode != 0:
        return None
    try:
        return int(result.stdout.strip())
    except ValueError:
        return None


def http_json(url: str) -> Any | None:
    try:
        with urllib.request.urlopen(url, timeout=2) as response:
            return json.loads(response.read().decode("utf-8"))
    except (urllib.error.URLError, TimeoutError, json.JSONDecodeError):
        return None


def loki_instant(loki_url: str, query: str) -> list[tuple[dict[str, str], float]] | None:
    params = urllib.parse.urlencode({"query": query})
    payload = http_json(f"{loki_url}/loki/api/v1/query?{params}")
    if not payload or payload.get("status") != "success":
        return None
    out = []
    for item in payload.get("data", {}).get("result", []):
        value = item.get("value", [None, "0"])[1]
        try:
            count = float(value)
        except (TypeError, ValueError):
            count = 0
        out.append((item.get("metric", {}), count))
    return out


def loki_range(loki_url: str, query: str, window: str, limit: int, direction: str) -> list[str] | None:
    params = urllib.parse.urlencode({
        "query": query,
        "limit": str(limit),
        "direction": direction,
        "start": str(time.time_ns() - (window_seconds(window) * 1_000_000_000)),
    })
    payload = http_json(f"{loki_url}/loki/api/v1/query_range?{params}")
    if not payload or payload.get("status") != "success":
        return None
    lines = []
    for stream in payload.get("data", {}).get("result", []):
        for _, line in stream.get("values", []):
            lines.append(line)
    return lines[:limit]


def run(cmd: list[str], cwd: Path | None = None, timeout: int = 10) -> CommandResult:
    try:
        proc = subprocess.run(
            cmd,
            cwd=str(cwd) if cwd else None,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            timeout=timeout,
            check=False,
        )
        return CommandResult(proc.returncode, proc.stdout, proc.stderr)
    except (OSError, subprocess.TimeoutExpired) as err:
        return CommandResult(1, "", str(err))


def require_cmd(name: str) -> bool:
    return shutil.which(name) is not None


def valid_window(value: str) -> bool:
    return re.fullmatch(r"[1-9][0-9]*[smhd]", value) is not None


def window_seconds(value: str) -> int:
    amount = int(value[:-1])
    unit = value[-1]
    if unit == "s":
        return amount
    if unit == "m":
        return amount * 60
    if unit == "h":
        return amount * 60 * 60
    if unit == "d":
        return amount * 24 * 60 * 60
    raise ValueError(f"invalid window: {value}")


def escape_dollar(value: str) -> str:
    return value.replace("$OPS$", "")


def print_title(title: str) -> None:
    print(f"\n{title}")
    print("=" * len(title))


def print_section(title: str) -> None:
    print(f"\n{title}")
    print("-" * len(title))


def print_warn(message: str) -> None:
    print(f"WARN: {message}")


def print_error(message: str) -> None:
    print(f"ERROR: {message}")


def print_dim(message: str) -> None:
    if message:
        print(f"  {message}")


def print_kv(key: str, value: str) -> None:
    print(f"{key}: {value}")


def print_counts(label: str, counts: list[tuple[dict[str, str], float]]) -> None:
    if not counts:
        print(f"{label}: 0")
        return
    total = int(sum(count for _, count in counts))
    print(f"{label}: {total}")
    for metric, count in counts:
        name = metric.get("filename") or metric.get("component") or "unknown"
        print(f"  {name}: {int(count)}")


def print_table(headers: list[str], rows: list[list[str]]) -> None:
    if not rows:
        print("No rows.")
        return
    widths = [len(h) for h in headers]
    normalized = []
    for row in rows:
        cells = [str(cell) for cell in row]
        cells += [""] * (len(headers) - len(cells))
        cells = cells[: len(headers)]
        normalized.append(cells)
        for idx, cell in enumerate(cells):
            widths[idx] = max(widths[idx], min(len(cell), 80))
    print("  ".join(h.ljust(widths[idx]) for idx, h in enumerate(headers)))
    print("  ".join("-" * widths[idx] for idx in range(len(headers))))
    for row in normalized:
        print("  ".join(truncate(cell, 80).ljust(widths[idx]) for idx, cell in enumerate(row)))


def truncate(value: str, limit: int) -> str:
    if len(value) <= limit:
        return value
    return value[: limit - 3] + "..."


def one_line(value: str) -> str:
    return "; ".join(line.strip() for line in value.splitlines() if line.strip())


def compact_json(value: Any) -> str:
    return json.dumps(value, sort_keys=True, separators=(",", ":"))


def summarize_log_line(line: str) -> str:
    try:
        item = json.loads(line)
    except json.JSONDecodeError:
        return truncate(line, 220)
    parts = [
        item.get("time", ""),
        item.get("component", ""),
        item.get("task_type", ""),
        item.get("msg", ""),
        item.get("err", ""),
    ]
    return truncate(" | ".join(str(part) for part in parts if part), 260)


if __name__ == "__main__":
    raise SystemExit(main())

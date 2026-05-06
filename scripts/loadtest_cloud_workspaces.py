#!/usr/bin/env python3
"""AFS Cloud workspace lifecycle load test."""

from __future__ import annotations

import argparse
import base64
import concurrent.futures
import http.client
import json
import os
import random
import re
import socket
import ssl
import sys
import threading
import time
import urllib.parse
from collections import Counter, defaultdict
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any


DEFAULT_BASE_URL = "https://afs.cloud"
LOCAL_HOSTS = {"localhost", "127.0.0.1", "::1"}
NAME_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]*$")


class LoadTestError(RuntimeError):
    pass


class HTTPStatusError(LoadTestError):
    def __init__(self, method: str, path: str, status: int, body: bytes):
        snippet = body.decode("utf-8", errors="replace").strip()
        if len(snippet) > 500:
            snippet = snippet[:500] + "..."
        super().__init__(f"{method} {path} returned HTTP {status}: {snippet}")
        self.status = status


@dataclass
class Metrics:
    lock: threading.Lock = field(default_factory=threading.Lock)
    durations_ms: dict[str, list[float]] = field(default_factory=lambda: defaultdict(list))
    status_counts: Counter[str] = field(default_factory=Counter)
    op_errors: Counter[str] = field(default_factory=Counter)
    scenario_errors: list[str] = field(default_factory=list)
    attempted: int = 0
    succeeded: int = 0
    failed: int = 0
    created: int = 0
    deleted: int = 0
    cleanup_deleted: int = 0

    def record_operation(self, op: str, duration_ms: float, status: int | None, error: BaseException | None = None) -> None:
        with self.lock:
            self.durations_ms[op].append(duration_ms)
            if status is not None:
                self.status_counts[f"{op}:{status}"] += 1
            if error is not None:
                self.op_errors[op] += 1

    def record_scenario(self, success: bool, error: BaseException | None = None) -> None:
        with self.lock:
            self.attempted += 1
            if success:
                self.succeeded += 1
            else:
                self.failed += 1
                if error is not None and len(self.scenario_errors) < 20:
                    self.scenario_errors.append(str(error))

    def bump(self, name: str) -> None:
        with self.lock:
            setattr(self, name, getattr(self, name) + 1)

    def snapshot(self) -> dict[str, int]:
        with self.lock:
            return {
                "attempted": self.attempted,
                "succeeded": self.succeeded,
                "failed": self.failed,
                "created": self.created,
                "deleted": self.deleted,
                "cleanup_deleted": self.cleanup_deleted,
            }


class AFSClient:
    def __init__(self, base_url: str, api_key: str, timeout: float, retries: int, metrics: Metrics):
        parsed = urllib.parse.urlparse(base_url)
        if parsed.scheme not in {"http", "https"}:
            raise LoadTestError(f"unsupported base URL scheme: {parsed.scheme!r}")
        if not parsed.hostname:
            raise LoadTestError(f"base URL must include a host: {base_url!r}")
        self.scheme = parsed.scheme
        self.host = parsed.hostname
        self.port = parsed.port
        self.netloc = parsed.netloc
        self.base_path = parsed.path.rstrip("/")
        self.api_key = api_key.strip()
        self.timeout = timeout
        self.retries = retries
        self.metrics = metrics
        self._conn: http.client.HTTPConnection | http.client.HTTPSConnection | None = None

    def close(self) -> None:
        if self._conn is not None:
            self._conn.close()
            self._conn = None

    def _connection(self) -> http.client.HTTPConnection | http.client.HTTPSConnection:
        if self._conn is not None:
            return self._conn
        if self.scheme == "https":
            context = ssl.create_default_context()
            self._conn = http.client.HTTPSConnection(self.host, port=self.port, timeout=self.timeout, context=context)
        else:
            self._conn = http.client.HTTPConnection(self.host, port=self.port, timeout=self.timeout)
        return self._conn

    def request_json(
        self,
        op: str,
        method: str,
        path: str,
        *,
        body: Any = None,
        expected: tuple[int, ...] = (200,),
    ) -> Any:
        full_path = self.base_path + path if self.base_path else path
        payload = None if body is None else json.dumps(body, separators=(",", ":")).encode("utf-8")
        headers = {"Accept": "application/json"}
        if payload is not None:
            headers["Content-Type"] = "application/json"
        if self.api_key:
            headers["Authorization"] = "Bearer " + self.api_key

        last_error: BaseException | None = None
        last_status: int | None = None
        started = time.perf_counter()
        try:
            for attempt in range(self.retries + 1):
                try:
                    conn = self._connection()
                    conn.request(method, full_path, body=payload, headers=headers)
                    response = conn.getresponse()
                    raw = response.read()
                    last_status = response.status
                    if response.status in expected:
                        if not raw:
                            self.metrics.record_operation(op, (time.perf_counter() - started) * 1000, response.status)
                            return None
                        decoded = json.loads(raw.decode("utf-8"))
                        self.metrics.record_operation(op, (time.perf_counter() - started) * 1000, response.status)
                        return decoded
                    if response.status == 429 or response.status >= 500:
                        last_error = HTTPStatusError(method, full_path, response.status, raw)
                        self.close()
                        if attempt < self.retries:
                            self._sleep_before_retry(attempt)
                            continue
                    raise HTTPStatusError(method, full_path, response.status, raw)
                except (http.client.HTTPException, OSError, TimeoutError, json.JSONDecodeError) as exc:
                    last_error = exc
                    self.close()
                    if attempt < self.retries:
                        self._sleep_before_retry(attempt)
                        continue
                    raise
            if last_error is not None:
                raise last_error
            raise LoadTestError(f"{method} {full_path} failed without an HTTP response")
        except BaseException as exc:
            self.metrics.record_operation(op, (time.perf_counter() - started) * 1000, last_status, exc)
            raise

    def _sleep_before_retry(self, attempt: int) -> None:
        delay = min(2.0, 0.15 * (2**attempt)) + random.uniform(0, 0.1)
        time.sleep(delay)


@dataclass(frozen=True)
class RunConfig:
    base_url: str
    api_key: str
    database_id: str
    prefix: str
    workspaces: int
    concurrency: int
    duration_seconds: float
    files_per_workspace: int
    checkpoints_per_workspace: int
    list_every: int
    timeout: float
    retries: int
    cleanup: bool
    cleanup_only: bool
    plan_only: bool
    progress_interval: float


class LoadRunner:
    def __init__(self, config: RunConfig):
        self.config = config
        self.metrics = Metrics()
        self.created: dict[str, str] = {}
        self.created_lock = threading.Lock()
        self.next_index = 0
        self.next_lock = threading.Lock()
        self.stop_event = threading.Event()
        self.deadline = time.monotonic() + config.duration_seconds if config.duration_seconds > 0 else 0.0

    def run(self) -> int:
        started = time.perf_counter()
        if self.config.cleanup_only:
            deleted = self.cleanup_by_prefix()
            elapsed = time.perf_counter() - started
            self.print_summary(elapsed)
            return 0 if deleted >= 0 else 1

        if self.config.progress_interval > 0:
            progress = threading.Thread(target=self._progress_loop, daemon=True)
            progress.start()

        with concurrent.futures.ThreadPoolExecutor(max_workers=self.config.concurrency) as pool:
            futures = [pool.submit(self.worker, worker_id) for worker_id in range(1, self.config.concurrency + 1)]
            for future in concurrent.futures.as_completed(futures):
                try:
                    future.result()
                except KeyboardInterrupt:
                    self.stop_event.set()
                    raise
                except BaseException as exc:
                    self.metrics.record_scenario(False, exc)
                    self.stop_event.set()

        if self.config.cleanup:
            self.cleanup_created()

        elapsed = time.perf_counter() - started
        self.print_summary(elapsed)
        clean_enough = not self.config.cleanup or not self.created
        return 0 if self.metrics.failed == 0 and clean_enough else 1

    def worker(self, worker_id: int) -> None:
        client = AFSClient(
            self.config.base_url,
            self.config.api_key,
            self.config.timeout,
            self.config.retries,
            self.metrics,
        )
        try:
            while not self.stop_event.is_set():
                index = self.next_work_index()
                if index is None:
                    return
                try:
                    self.run_workspace_scenario(client, worker_id, index)
                    self.metrics.record_scenario(True)
                except BaseException as exc:
                    self.metrics.record_scenario(False, exc)
        finally:
            client.close()

    def next_work_index(self) -> int | None:
        with self.next_lock:
            if self.config.duration_seconds > 0:
                if time.monotonic() >= self.deadline:
                    return None
            elif self.next_index >= self.config.workspaces:
                return None
            self.next_index += 1
            return self.next_index

    def run_workspace_scenario(self, client: AFSClient, worker_id: int, index: int) -> None:
        workspace_name = f"{self.config.prefix}-{index:06d}"
        import_response = client.request_json(
            "import_workspace",
            "POST",
            self.workspace_collection_path(":import"),
            body=self.import_payload(workspace_name, worker_id, index),
            expected=(201,),
        )
        workspace_id = (
            str(import_response.get("workspace_id") or "")
            or str(import_response.get("workspace", {}).get("id") or "")
            or workspace_name
        )
        self.track_created(workspace_name, workspace_id)
        self.metrics.bump("created")

        detail = client.request_json("get_workspace", "GET", self.workspace_path(workspace_id), expected=(200,))
        head_checkpoint = str(detail.get("head_checkpoint_id") or "initial")

        if self.config.list_every > 0 and index % self.config.list_every == 0:
            client.request_json("list_workspaces", "GET", self.workspace_collection_path(), expected=(200,))

        client.request_json(
            "get_tree",
            "GET",
            self.workspace_path(workspace_id, "/tree") + "?view=head&path=/&depth=2",
            expected=(200,),
        )
        client.request_json(
            "get_file_content",
            "GET",
            self.workspace_path(workspace_id, "/files/content") + "?view=head&path=/README.md",
            expected=(200,),
        )

        session_id = self.create_and_heartbeat_session(client, workspace_id, worker_id, index)
        if session_id:
            client.request_json("list_sessions", "GET", self.workspace_path(workspace_id, "/sessions"), expected=(200,))
            client.request_json(
                "close_session",
                "DELETE",
                self.client_session_path(workspace_id, session_id),
                expected=(204,),
            )

        for checkpoint_index in range(1, self.config.checkpoints_per_workspace + 1):
            checkpoint_id = f"lt-{index:06d}-{checkpoint_index:02d}"
            client.request_json(
                "save_checkpoint",
                "POST",
                self.workspace_path(workspace_id, ":save-from-live"),
                body={
                    "checkpoint_id": checkpoint_id,
                    "description": "Created by cloud workspace load test.",
                    "kind": "manual",
                    "source": "loadtest",
                    "author": "loadtest-cloud-workspaces",
                    "allow_unchanged": True,
                },
                expected=(201,),
            )

        checkpoints = client.request_json(
            "list_checkpoints",
            "GET",
            self.workspace_path(workspace_id, "/checkpoints") + "?limit=5",
            expected=(200,),
        )
        first_checkpoint = first_checkpoint_id(checkpoints) or head_checkpoint
        if first_checkpoint:
            client.request_json(
                "get_checkpoint",
                "GET",
                self.workspace_path(workspace_id, f"/checkpoints/{quote_path(first_checkpoint)}"),
                expected=(200,),
            )

        if self.config.cleanup:
            self.delete_tracked_workspace(client, workspace_name, workspace_id, "delete_workspace")

    def create_and_heartbeat_session(self, client: AFSClient, workspace_id: str, worker_id: int, index: int) -> str:
        body = {
            "agent_id": f"loadtest-worker-{worker_id}",
            "agent_name": "AFS cloud load test",
            "session_name": f"workspace-{index:06d}",
            "client_kind": "loadtest",
            "afs_version": "loadtest",
            "hostname": socket.gethostname(),
            "os": sys.platform,
            "local_path": "",
            "readonly": False,
        }
        session = client.request_json(
            "create_session",
            "POST",
            self.client_sessions_path(workspace_id),
            body=body,
            expected=(201,),
        )
        session_id = str(session.get("session_id") or "")
        if session_id:
            client.request_json(
                "heartbeat_session",
                "POST",
                self.client_session_path(workspace_id, session_id, "/heartbeat"),
                body=body,
                expected=(200,),
            )
        return session_id

    def import_payload(self, workspace_name: str, worker_id: int, index: int) -> dict[str, Any]:
        now_ms = int(time.time() * 1000)
        entries: dict[str, dict[str, Any]] = {
            "/": {"type": "dir", "mode": 0o755, "mtime_ms": now_ms},
            "/docs": {"type": "dir", "mode": 0o755, "mtime_ms": now_ms},
            "/data": {"type": "dir", "mode": 0o755, "mtime_ms": now_ms},
        }
        files = {
            "/README.md": (
                f"# {workspace_name}\n\n"
                f"Generated by scripts/loadtest_cloud_workspaces.py.\n"
                f"worker={worker_id}\nindex={index}\n"
            )
        }
        for file_index in range(1, self.config.files_per_workspace + 1):
            files[f"/docs/file-{file_index:03d}.md"] = (
                f"# File {file_index}\n\n"
                f"workspace={workspace_name}\nworker={worker_id}\nindex={index}\n"
            )
            files[f"/data/item-{file_index:03d}.json"] = json.dumps(
                {
                    "workspace": workspace_name,
                    "worker": worker_id,
                    "index": index,
                    "file": file_index,
                },
                sort_keys=True,
            ) + "\n"

        total_bytes = 0
        for path, content in files.items():
            data = content.encode("utf-8")
            total_bytes += len(data)
            entries[path] = {
                "type": "file",
                "mode": 0o644,
                "mtime_ms": now_ms,
                "size": len(data),
                "inline": base64.b64encode(data).decode("ascii"),
            }

        payload: dict[str, Any] = {
            "name": workspace_name,
            "description": "Temporary workspace created by cloud load test.",
            "manifest": {
                "version": 1,
                "workspace": workspace_name,
                "savepoint": "initial",
                "entries": entries,
            },
            "blobs": {},
            "file_count": len(files),
            "dir_count": 3,
            "total_bytes": total_bytes,
        }
        if self.config.database_id:
            payload["database_id"] = self.config.database_id
        return payload

    def workspace_collection_path(self, suffix: str = "") -> str:
        if self.config.database_id:
            return f"/v1/databases/{quote_path(self.config.database_id)}/workspaces{suffix}"
        return f"/v1/workspaces{suffix}"

    def workspace_path(self, workspace: str, suffix: str = "") -> str:
        return f"{self.workspace_collection_path()}/{quote_path(workspace)}{suffix}"

    def client_sessions_path(self, workspace: str) -> str:
        if self.config.database_id:
            return f"/v1/client/databases/{quote_path(self.config.database_id)}/workspaces/{quote_path(workspace)}/sessions"
        return f"/v1/client/workspaces/{quote_path(workspace)}/sessions"

    def client_session_path(self, workspace: str, session_id: str, suffix: str = "") -> str:
        return f"{self.client_sessions_path(workspace)}/{quote_path(session_id)}{suffix}"

    def track_created(self, workspace_name: str, workspace_id: str) -> None:
        with self.created_lock:
            self.created[workspace_name] = workspace_id

    def untrack_created(self, workspace_name: str) -> None:
        with self.created_lock:
            self.created.pop(workspace_name, None)

    def delete_tracked_workspace(self, client: AFSClient, workspace_name: str, workspace_id: str, op: str) -> None:
        client.request_json(op, "DELETE", self.workspace_path(workspace_id), expected=(204,))
        self.untrack_created(workspace_name)
        self.metrics.bump("deleted")

    def cleanup_created(self) -> None:
        with self.created_lock:
            leftovers = list(self.created.items())
        if not leftovers:
            return
        print(f"\nCleaning up {len(leftovers)} workspace(s) created by this run...")
        with concurrent.futures.ThreadPoolExecutor(max_workers=self.config.concurrency) as pool:
            futures = [pool.submit(self.cleanup_one, name, workspace_id) for name, workspace_id in leftovers]
            for future in concurrent.futures.as_completed(futures):
                try:
                    future.result()
                except BaseException as exc:
                    self.metrics.record_scenario(False, exc)

    def cleanup_by_prefix(self) -> int:
        client = AFSClient(
            self.config.base_url,
            self.config.api_key,
            self.config.timeout,
            self.config.retries,
            self.metrics,
        )
        try:
            response = client.request_json("cleanup_list_workspaces", "GET", self.workspace_collection_path(), expected=(200,))
            items = response.get("items", []) if isinstance(response, dict) else []
            matches = [
                (str(item.get("name") or ""), str(item.get("id") or item.get("name") or ""))
                for item in items
                if str(item.get("name") or "").startswith(self.config.prefix)
            ]
        finally:
            client.close()

        print(f"Found {len(matches)} workspace(s) with prefix {self.config.prefix!r}.")
        with concurrent.futures.ThreadPoolExecutor(max_workers=self.config.concurrency) as pool:
            futures = [pool.submit(self.cleanup_one, name, workspace_id) for name, workspace_id in matches]
            for future in concurrent.futures.as_completed(futures):
                future.result()
        return len(matches)

    def cleanup_one(self, workspace_name: str, workspace_id: str) -> None:
        client = AFSClient(
            self.config.base_url,
            self.config.api_key,
            self.config.timeout,
            self.config.retries,
            self.metrics,
        )
        try:
            client.request_json("cleanup_delete_workspace", "DELETE", self.workspace_path(workspace_id), expected=(204, 404))
            self.untrack_created(workspace_name)
            self.metrics.bump("cleanup_deleted")
        finally:
            client.close()

    def _progress_loop(self) -> None:
        while not self.stop_event.wait(self.config.progress_interval):
            snap = self.metrics.snapshot()
            print(
                "progress: "
                f"attempted={snap['attempted']} succeeded={snap['succeeded']} failed={snap['failed']} "
                f"created={snap['created']} deleted={snap['deleted']}",
                flush=True,
            )

    def print_summary(self, elapsed: float) -> None:
        snap = self.metrics.snapshot()
        print("\nAFS cloud workspace load test summary")
        print(f"Target: {self.config.base_url}")
        if self.config.database_id:
            print(f"Database: {self.config.database_id}")
        print(f"Prefix: {self.config.prefix}")
        print(f"Elapsed: {elapsed:.2f}s")
        print(
            "Scenarios: "
            f"attempted={snap['attempted']} succeeded={snap['succeeded']} failed={snap['failed']} "
            f"created={snap['created']} deleted={snap['deleted']} cleanup_deleted={snap['cleanup_deleted']}"
        )
        print()
        print(f"{'operation':28} {'count':>7} {'errors':>7} {'p50 ms':>10} {'p95 ms':>10} {'p99 ms':>10} {'max ms':>10}")
        print("-" * 86)
        for op in sorted(self.metrics.durations_ms):
            values = sorted(self.metrics.durations_ms[op])
            if not values:
                continue
            print(
                f"{op:28} {len(values):7d} {self.metrics.op_errors.get(op, 0):7d} "
                f"{percentile(values, 50):10.1f} {percentile(values, 95):10.1f} "
                f"{percentile(values, 99):10.1f} {values[-1]:10.1f}"
            )
        if self.metrics.scenario_errors:
            print("\nFirst scenario errors:")
            for message in self.metrics.scenario_errors:
                print(f"- {message}")

    def summary_json(self, elapsed: float) -> dict[str, Any]:
        return {
            "target": self.config.base_url,
            "database_id": self.config.database_id,
            "prefix": self.config.prefix,
            "elapsed_seconds": elapsed,
            "metrics": self.metrics.snapshot(),
            "operations": {
                op: {
                    "count": len(values),
                    "errors": self.metrics.op_errors.get(op, 0),
                    "p50_ms": percentile(sorted(values), 50),
                    "p95_ms": percentile(sorted(values), 95),
                    "p99_ms": percentile(sorted(values), 99),
                    "max_ms": max(values),
                }
                for op, values in self.metrics.durations_ms.items()
                if values
            },
            "status_counts": dict(self.metrics.status_counts),
            "errors": list(self.metrics.scenario_errors),
        }


def quote_path(value: str) -> str:
    return urllib.parse.quote(value, safe="")


def first_checkpoint_id(response: Any) -> str:
    if not isinstance(response, dict):
        return ""
    items = response.get("items")
    if not isinstance(items, list) or not items:
        return ""
    first = items[0]
    if not isinstance(first, dict):
        return ""
    return str(first.get("id") or first.get("name") or "")


def percentile(sorted_values: list[float], percent: int) -> float:
    if not sorted_values:
        return 0.0
    rank = (percent / 100.0) * (len(sorted_values) - 1)
    lower = int(rank)
    upper = min(lower + 1, len(sorted_values) - 1)
    if lower == upper:
        return sorted_values[lower]
    weight = rank - lower
    return sorted_values[lower] * (1 - weight) + sorted_values[upper] * weight


def default_prefix() -> str:
    stamp = time.strftime("%Y%m%d-%H%M%S", time.gmtime())
    suffix = random.randrange(1000, 9999)
    return f"load-{stamp}-{suffix}"


def is_local_base_url(raw: str) -> bool:
    parsed = urllib.parse.urlparse(raw)
    return (parsed.hostname or "") in LOCAL_HOSTS


def validate_name(kind: str, value: str) -> None:
    if not value:
        raise LoadTestError(f"{kind} is required")
    if not NAME_RE.match(value):
        raise LoadTestError(f"{kind} {value!r} is invalid; use letters, numbers, dot, dash, and underscore")


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", default=os.environ.get("AFS_API_BASE_URL", DEFAULT_BASE_URL))
    parser.add_argument("--api-key", default=os.environ.get("AFS_API_KEY") or os.environ.get("AFS_TOKEN") or "")
    parser.add_argument("--database-id", default=os.environ.get("AFS_DATABASE_ID", ""))
    parser.add_argument("--prefix", default=default_prefix())
    parser.add_argument("--workspaces", type=int, default=25)
    parser.add_argument("--concurrency", type=int, default=5)
    parser.add_argument("--duration-seconds", type=float, default=0.0)
    parser.add_argument("--files-per-workspace", type=int, default=5)
    parser.add_argument("--checkpoints-per-workspace", type=int, default=1)
    parser.add_argument("--list-every", type=int, default=5, help="Run GET /workspaces every N scenarios; 0 disables it.")
    parser.add_argument("--timeout", type=float, default=30.0)
    parser.add_argument("--retries", type=int, default=2)
    parser.add_argument("--progress-interval", type=float, default=5.0)
    parser.add_argument("--max-workspaces", type=int, default=1000)
    parser.add_argument("--no-cleanup", action="store_true", help="Leave created workspaces behind.")
    parser.add_argument("--cleanup-only", action="store_true", help="Delete existing workspaces whose names start with --prefix.")
    parser.add_argument("--allow-anonymous", action="store_true", help="Allow requests without a bearer token for local/dev targets.")
    parser.add_argument("--yes", action="store_true", help="Confirm that this may create and delete workspaces on the target.")
    parser.add_argument("--plan-only", action="store_true", help="Print the resolved plan without making requests.")
    parser.add_argument("--output-json", help="Write a machine-readable summary to this path.")
    return parser.parse_args(argv)


def config_from_args(args: argparse.Namespace) -> RunConfig:
    base_url = args.base_url.rstrip("/")
    prefix = args.prefix.strip()
    validate_name("workspace prefix", prefix)
    if len(prefix) < 6:
        raise LoadTestError("--prefix must be at least 6 characters so cleanup cannot match broad names")
    if args.workspaces <= 0:
        raise LoadTestError("--workspaces must be positive")
    if args.workspaces > args.max_workspaces and args.duration_seconds <= 0:
        raise LoadTestError(f"--workspaces={args.workspaces} exceeds --max-workspaces={args.max_workspaces}")
    if args.concurrency <= 0:
        raise LoadTestError("--concurrency must be positive")
    if args.files_per_workspace < 0:
        raise LoadTestError("--files-per-workspace cannot be negative")
    if args.checkpoints_per_workspace < 0:
        raise LoadTestError("--checkpoints-per-workspace cannot be negative")
    if args.list_every < 0:
        raise LoadTestError("--list-every cannot be negative")
    if args.timeout <= 0:
        raise LoadTestError("--timeout must be positive")
    if args.retries < 0:
        raise LoadTestError("--retries cannot be negative")
    if args.duration_seconds < 0:
        raise LoadTestError("--duration-seconds cannot be negative")

    remote = not is_local_base_url(base_url)
    if remote and not args.yes and not args.plan_only:
        raise LoadTestError("remote targets require --yes")
    if not args.api_key.strip() and remote and not args.allow_anonymous and not args.plan_only:
        raise LoadTestError("remote targets require --api-key or AFS_API_KEY")

    if args.database_id:
        validate_name("database id", args.database_id)

    return RunConfig(
        base_url=base_url,
        api_key=args.api_key,
        database_id=args.database_id.strip(),
        prefix=prefix,
        workspaces=args.workspaces,
        concurrency=args.concurrency,
        duration_seconds=args.duration_seconds,
        files_per_workspace=args.files_per_workspace,
        checkpoints_per_workspace=args.checkpoints_per_workspace,
        list_every=args.list_every,
        timeout=args.timeout,
        retries=args.retries,
        cleanup=not args.no_cleanup,
        cleanup_only=args.cleanup_only,
        plan_only=args.plan_only,
        progress_interval=args.progress_interval,
    )


def print_plan(config: RunConfig) -> None:
    mode = "cleanup-only" if config.cleanup_only else "load"
    if config.duration_seconds > 0:
        volume = f"duration={config.duration_seconds:.1f}s"
    else:
        volume = f"workspaces={config.workspaces}"
    print("AFS cloud workspace load test plan")
    print(f"Mode: {mode}")
    print(f"Target: {config.base_url}")
    if config.database_id:
        print(f"Database: {config.database_id}")
    print(f"Prefix: {config.prefix}")
    print(f"Volume: {volume}")
    print(f"Concurrency: {config.concurrency}")
    print(f"Files per workspace: {config.files_per_workspace * 2 + 1}")
    print(f"Checkpoints per workspace: {config.checkpoints_per_workspace}")
    print(f"Cleanup: {'yes' if config.cleanup else 'no'}")


def write_summary(path: str, summary: dict[str, Any]) -> None:
    target = Path(path)
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(json.dumps(summary, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    try:
        config = config_from_args(args)
        print_plan(config)
        if config.plan_only:
            return 0
        runner = LoadRunner(config)
        started = time.perf_counter()
        try:
            exit_code = runner.run()
        except KeyboardInterrupt:
            runner.stop_event.set()
            if config.cleanup:
                runner.cleanup_created()
            print("\nInterrupted.")
            exit_code = 130
        elapsed = time.perf_counter() - started
        if args.output_json:
            write_summary(args.output_json, runner.summary_json(elapsed))
        return exit_code
    except LoadTestError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))

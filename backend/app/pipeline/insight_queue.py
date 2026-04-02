"""
Shared in-process queue for signalling insight generation.

PipelineWorker pushes kg_ids here after enriching artifacts.
InsightWorker drains it and runs generate_and_store per unique kg_id.
"""
import queue
import threading

_q: queue.Queue[str] = queue.Queue()


def push(kg_id: str) -> None:
    _q.put(kg_id)


def drain() -> set[str]:
    """Return all pending kg_ids, deduplicated."""
    ids: set[str] = set()
    try:
        while True:
            ids.add(_q.get_nowait())
    except queue.Empty:
        pass
    return ids


def wait_and_drain(timeout: float = 30.0) -> set[str]:
    """Block until at least one kg_id arrives, then drain the rest."""
    ids: set[str] = set()
    ids.add(_q.get(timeout=timeout))  # blocks until item or timeout
    return ids | drain()

"""Write the language pack SQLite file.

Everything is written in one transaction per table with `executemany`, indexes
created after the bulk load, then VACUUM. On a table with ~1.4 M wordforms the
difference between indexing-before and indexing-after is minutes.
"""

from __future__ import annotations

import hashlib
import json
import sqlite3
from collections.abc import Iterable
from pathlib import Path

SCHEMA = Path(__file__).resolve().parent / "schema.sql"

# Bumped only for an incompatible schema change. The Go binary refuses to start
# on a major mismatch rather than failing confusingly later.
SCHEMA_VERSION = 1


def create(path: Path) -> sqlite3.Connection:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.unlink(missing_ok=True)
    conn = sqlite3.connect(path)
    conn.executescript(SCHEMA.read_text(encoding="utf-8"))
    return conn


def _split_indexes(conn: sqlite3.Connection) -> list[str]:
    """Drop indexes for the bulk load, returning their DDL so they can be rebuilt."""
    rows = conn.execute(
        "SELECT sql FROM sqlite_master WHERE type='index' AND sql IS NOT NULL"
    ).fetchall()
    ddl = [r[0] for r in rows]
    for r in conn.execute(
        "SELECT name FROM sqlite_master WHERE type='index' AND sql IS NOT NULL"
    ).fetchall():
        conn.execute(f"DROP INDEX IF EXISTS {r[0]}")
    return ddl


def bulk(conn: sqlite3.Connection, table: str, columns: list[str], rows: Iterable[tuple]) -> int:
    placeholders = ",".join("?" * len(columns))
    sql = f"INSERT OR IGNORE INTO {table} ({','.join(columns)}) VALUES ({placeholders})"
    n = 0
    batch: list[tuple] = []
    for row in rows:
        batch.append(row)
        if len(batch) >= 20_000:
            conn.executemany(sql, batch)
            n += len(batch)
            batch.clear()
    if batch:
        conn.executemany(sql, batch)
        n += len(batch)
    return n


def finalise(conn: sqlite3.Connection, index_ddl: list[str], meta: dict[str, object]) -> None:
    for ddl in index_ddl:
        conn.execute(ddl)
    conn.executemany(
        "INSERT OR REPLACE INTO pack_meta (key, value) VALUES (?, ?)",
        [(k, json.dumps(v) if not isinstance(v, str) else v) for k, v in meta.items()],
    )
    conn.commit()
    conn.execute("VACUUM")
    conn.execute("ANALYZE")
    conn.commit()


def checksum(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as fh:
        for chunk in iter(lambda: fh.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def write_manifest(pack: Path, meta: dict[str, object]) -> Path:
    """A sidecar manifest so the pack can be verified before it is opened."""
    manifest = pack.with_suffix(".manifest.json")
    payload = dict(meta)
    payload["file"] = pack.name
    payload["sha256"] = checksum(pack)
    payload["bytes"] = pack.stat().st_size
    manifest.write_text(json.dumps(payload, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    return manifest

# MCP Lens Event Storage Architecture

## Overview

This document describes the event storage architecture for MCP Lens, including the research into database approaches that inspired our design decisions.

## Problem Statement

MCP Lens needs to collect events from multiple concurrent Claude Code sessions:

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Claude Code    │     │  Claude Code    │     │  Claude Code    │
│   Session A     │     │   Session B     │     │   Session C     │
└────────┬────────┘     └────────┬────────┘     └────────┬────────┘
         │                       │                       │
         │ Hook Events           │ Hook Events           │ Hook Events
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Event Storage Layer                          │
│                         (???)                                    │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
                    ┌─────────────────────┐
                    │   SQLite Database   │
                    │   (Aggregations)    │
                    └─────────────────────┘
```

**Challenge**: How do we safely handle concurrent writes from multiple sessions?

---

## Database Approaches Studied

### 1. SQLite WAL (Write-Ahead Log)

```
┌─────────────────────────────────────────────────────────────┐
│                      SQLite WAL Mode                         │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Writer A ──┐                                                │
│             │    ┌─────────┐      ┌──────────────┐          │
│  Writer B ──┼───►│   WAL   │─────►│   Database   │          │
│             │    │  File   │      │    File      │          │
│  Writer C ──┘    └─────────┘      └──────────────┘          │
│                       │                   ▲                  │
│                       │   Checkpoint      │                  │
│                       └───────────────────┘                  │
│                                                              │
│  Lock: WAL_WRITE_LOCK (exclusive - one writer at a time)    │
│  Readers: Can proceed concurrently (SHARED lock)            │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

**How it works:**
- All writes append to WAL file first
- Single exclusive write lock (`WAL_WRITE_LOCK`)
- Multiple readers can proceed without blocking
- Periodic checkpoint transfers WAL → main database

**Concurrency Model:**
```
Writer A: [====LOCK====][write][unlock]
Writer B:              [wait...][====LOCK====][write][unlock]
Writer C:                       [wait........][====LOCK====][write]
Reader 1: [read────────────────────────────────────────────────]
Reader 2:        [read─────────────────────────────]
```

**Pros:** Battle-tested, ACID compliant, readers never block
**Cons:** Writers block each other, doesn't work over NFS

---

### 2. Apache Kafka Commit Log

```
┌─────────────────────────────────────────────────────────────┐
│                    Kafka Partition Model                     │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Topic: "events"                                             │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ Partition 0    [msg0][msg1][msg2][msg3][msg4]──►    │    │
│  │ (Leader: Broker A)           ▲                      │    │
│  │                              │                      │    │
│  │                         Only ONE writer             │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ Partition 1    [msg0][msg1][msg2]──►                │    │
│  │ (Leader: Broker B)     ▲                            │    │
│  │                        │                            │    │
│  │                   Only ONE writer                   │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                              │
│  Consumer tracks offset: "I've read up to msg2"             │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

**How it works:**
- Topics split into partitions
- Each partition has ONE leader (single writer)
- Producers route by key (e.g., session_id → partition)
- Consumers track offset (position)

**Concurrency Model:**
```
Partition 0: Session A writes ───────────────────►
Partition 1: Session B writes ───────────────────►
Partition 2: Session C writes ───────────────────►
                              (No contention!)
```

**Pros:** Horizontally scalable, no write contention within partition
**Cons:** Complex infrastructure, overkill for local tool

---

### 3. Redis AOF (Append-Only File)

```
┌─────────────────────────────────────────────────────────────┐
│                      Redis AOF Mode                          │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────┐                                           │
│  │    Redis     │──── Single-threaded event loop            │
│  │   Server     │                                           │
│  └──────┬───────┘                                           │
│         │                                                    │
│         │ Append                                             │
│         ▼                                                    │
│  ┌──────────────────────────────────────┐                   │
│  │         appendonly.aof               │                   │
│  │  SET key1 value1                     │                   │
│  │  SET key2 value2                     │                   │
│  │  INCR counter                        │                   │
│  │  ...                                 │                   │
│  └──────────────────────────────────────┘                   │
│                                                              │
│  fsync policy: always | everysec | no                       │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

**How it works:**
- Single-threaded: no locks needed!
- All commands serialized through event loop
- AOF records every write operation
- Background rewrite for compaction

**Concurrency Model:**
```
Client A: ──req──►│
Client B: ──req──►│  Event   │──►[write to AOF]──►[write to AOF]──►
Client C: ──req──►│  Loop    │
                  (serialized)
```

**Pros:** Simple, no lock contention, predictable latency
**Cons:** Single point of serialization, requires daemon

---

### 4. LevelDB/RocksDB LSM Tree

```
┌─────────────────────────────────────────────────────────────┐
│                   LSM Tree Architecture                      │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Writes ──►  ┌────────────┐                                 │
│              │  MemTable  │  (in-memory, fast writes)       │
│              └─────┬──────┘                                 │
│                    │ flush when full                        │
│                    ▼                                        │
│              ┌────────────┐                                 │
│              │   WAL      │  (durability)                   │
│              └─────┬──────┘                                 │
│                    │                                        │
│                    ▼                                        │
│  Level 0:   [SST][SST][SST]                                │
│                    │ compaction                             │
│  Level 1:   [====SST====][====SST====]                     │
│                    │                                        │
│  Level 2:   [========SST========][========SST========]     │
│                                                              │
│  Write Lock: Single writer (mutex)                          │
│  File Lock: flock() for multi-process                       │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

**How it works:**
- Writes go to MemTable (memory) + WAL (durability)
- MemTable flushes to SST files (sorted string tables)
- Background compaction merges levels
- Single writer with mutex

**Pros:** High write throughput, efficient range scans
**Cons:** Write amplification, complex compaction

---

### 5. PostgreSQL WAL + MVCC

```
┌─────────────────────────────────────────────────────────────┐
│              PostgreSQL Concurrency Control                  │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Transaction A ──┐                                          │
│                  │     ┌─────────────┐                      │
│  Transaction B ──┼────►│    WAL      │                      │
│                  │     │  Segments   │                      │
│  Transaction C ──┘     └──────┬──────┘                      │
│                               │                              │
│                               ▼                              │
│                        ┌─────────────┐                      │
│                        │   Heap      │                      │
│                        │  (Tables)   │                      │
│                        └─────────────┘                      │
│                                                              │
│  MVCC: Each row has xmin, xmax (transaction visibility)     │
│  Writers don't block readers (snapshot isolation)           │
│  Row-level locks for write conflicts                        │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

**How it works:**
- WAL for durability (write-ahead)
- MVCC for concurrency (multiple versions of rows)
- Snapshot isolation: each transaction sees consistent view
- Row-level locking (not table-level)

**Pros:** True concurrent writes, ACID, sophisticated
**Cons:** Complex, requires server process, vacuum overhead

---

## Design Decision Matrix

| Approach | Write Contention | Complexity | Durability | Fits MCP Lens? |
|----------|-----------------|------------|------------|----------------|
| SQLite WAL | High (single writer) | Low | High | Partial ⚠️ |
| Kafka Partitions | None (per-partition) | High | High | Overkill ❌ |
| Redis AOF | None (single-threaded) | Medium | Configurable | Needs daemon ❌ |
| LevelDB/RocksDB | Medium (mutex) | High | High | Overkill ❌ |
| PostgreSQL | Low (MVCC) | Very High | High | Overkill ❌ |
| **Per-Session Files** | **None** | **Low** | **High** | **Perfect ✅** |

---

## Chosen Architecture: Per-Session Files

Inspired by **Kafka's partition model**, we chose per-session files:

```
┌─────────────────────────────────────────────────────────────────────┐
│                    MCP Lens Event Storage                            │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌─────────────┐   ┌─────────────┐   ┌─────────────┐               │
│  │ Claude Code │   │ Claude Code │   │ Claude Code │               │
│  │  Session A  │   │  Session B  │   │  Session C  │               │
│  └──────┬──────┘   └──────┬──────┘   └──────┬──────┘               │
│         │                 │                 │                        │
│         │ Hooks           │ Hooks           │ Hooks                  │
│         ▼                 ▼                 ▼                        │
│  ┌─────────────┐   ┌─────────────┐   ┌─────────────┐               │
│  │sess-A.jsonl │   │sess-B.jsonl │   │sess-C.jsonl │               │
│  │             │   │             │   │             │  ◄── Like Kafka│
│  │ [event]     │   │ [event]     │   │ [event]     │      Partitions│
│  │ [event]     │   │ [event]     │   │ [event]     │                │
│  │ [event]     │   │ [event]     │   │ [event]     │                │
│  └──────┬──────┘   └──────┬──────┘   └──────┬──────┘               │
│         │                 │                 │                        │
│         │    NO CONTENTION - Each session owns its file!            │
│         │                 │                 │                        │
│         └────────────────┬┴─────────────────┘                       │
│                          │                                           │
│                          ▼                                           │
│              ┌─────────────────────┐                                │
│              │    Sync Engine      │  ◄── Like Kafka Consumer       │
│              │  (reads all files)  │      tracking offsets          │
│              └──────────┬──────────┘                                │
│                         │                                            │
│                         │  Merge + Deduplicate                       │
│                         │  (fingerprint-based)                       │
│                         ▼                                            │
│              ┌─────────────────────┐                                │
│              │      SQLite         │  ◄── Like SQLite WAL           │
│              │   (aggregations)    │      for final storage         │
│              │                     │                                 │
│              │  • tool_stats       │                                 │
│              │  • sessions         │                                 │
│              │  • recent_events    │                                 │
│              │  • fingerprints     │                                 │
│              └─────────────────────┘                                │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### Why Per-Session Files?

**Inspired by Kafka:**
- Each session = one partition
- One writer per partition = no contention
- Consumer (sync engine) reads from all partitions

**Inspired by SQLite WAL:**
- Append-only writes
- fsync for durability
- Position tracking for incremental reads

**Inspired by Redis AOF:**
- Simple JSONL format (like AOF commands)
- Easy to debug/inspect
- Human-readable

---

## Data Flow Detail

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Write Path                                   │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  1. Claude Code fires hook event                                     │
│     │                                                                │
│     ▼                                                                │
│  2. SessionWriter receives event                                     │
│     │                                                                │
│     ├── Determine file: ~/.mcp-lens/events/{session_id}.jsonl       │
│     │                                                                │
│     ├── Acquire flock (LOCK_EX) ◄── Safety for same session         │
│     │                                                                │
│     ├── Append JSON line                                             │
│     │                                                                │
│     ├── fsync() ◄── Durability (like Redis AOF "always")            │
│     │                                                                │
│     └── Release flock                                                │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                         Read Path (Sync)                             │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  1. SyncEngine.Sync() called (manual or periodic)                    │
│     │                                                                │
│     ▼                                                                │
│  2. List all session files: ~/.mcp-lens/events/*.jsonl              │
│     │                                                                │
│     ▼                                                                │
│  3. For each file:                                                   │
│     │                                                                │
│     ├── Get last sync position (like Kafka consumer offset)         │
│     │                                                                │
│     ├── Read new lines from position                                 │
│     │                                                                │
│     ├── Parse JSONL events                                           │
│     │                                                                │
│     ├── Validate events ◄── Skip malformed/invalid                  │
│     │                                                                │
│     ├── Check fingerprint ◄── Skip duplicates                       │
│     │                                                                │
│     ├── Update SQLite aggregations                                   │
│     │                                                                │
│     ├── Store fingerprint                                            │
│     │                                                                │
│     └── Update sync position                                         │
│                                                                      │
│  4. Return SyncResult with stats                                     │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Concurrency Safety Analysis

### Scenario: Multiple Claude Sessions (Different Sessions)

```
Time ──────────────────────────────────────────────────────────►

Session A: [write sess-A.jsonl][write][write][write]
Session B:    [write sess-B.jsonl][write][write][write]
Session C:       [write sess-C.jsonl][write][write]

                     ▲
                     │
              NO CONTENTION!
         Each session writes to its own file
```

### Scenario: Same Session, Multiple Processes (Edge Case)

```
Time ──────────────────────────────────────────────────────────►

Process 1: [flock][write sess-A.jsonl][unlock]
Process 2:        [wait...............][flock][write][unlock]

                     ▲
                     │
              flock() serializes writes
              (like SQLite WAL_WRITE_LOCK)
```

### Scenario: Sync While Writes Happening

```
Time ──────────────────────────────────────────────────────────►

Writer:     [write][write][write][write][write]
                      │
Sync:       [read position][read to position][update SQLite]
                      │
                      ▼
              Sync reads up to a point
              Next sync picks up new events
              (like Kafka consumer lag)
```

---

## File Format

### Session Event File (JSONL)

```
~/.mcp-lens/events/sess-abc123.jsonl
```

```json
{"ts":"2026-01-10T10:00:00Z","sid":"sess-abc123","type":"SessionStart","cwd":"/home/user/project"}
{"ts":"2026-01-10T10:01:00Z","sid":"sess-abc123","type":"PostToolUse","tool":"Read","ok":true,"dur_ms":15}
{"ts":"2026-01-10T10:02:00Z","sid":"sess-abc123","type":"PostToolUse","tool":"mcp__github__create_issue","ok":true,"dur_ms":200}
{"ts":"2026-01-10T10:03:00Z","sid":"sess-abc123","type":"Stop"}
```

### SQLite Schema (Aggregations)

```sql
-- Sync position per session file
CREATE TABLE sync_positions (
    file_path TEXT PRIMARY KEY,
    position INTEGER NOT NULL,
    updated_at TEXT NOT NULL
);

-- Event fingerprints for deduplication
CREATE TABLE event_fingerprints (
    fingerprint TEXT PRIMARY KEY,
    created_at TEXT NOT NULL
);

-- Aggregated tool statistics
CREATE TABLE tool_stats (
    date TEXT NOT NULL,
    tool_name TEXT NOT NULL,
    server_name TEXT NOT NULL DEFAULT '',
    call_count INTEGER DEFAULT 0,
    error_count INTEGER DEFAULT 0,
    total_latency_ms INTEGER DEFAULT 0,
    PRIMARY KEY (date, tool_name)
);
```

---

## Comparison: Before vs After

### Before (Single File)

```
Problem:
┌──────────┐    ┌──────────┐    ┌──────────┐
│Session A │    │Session B │    │Session C │
└────┬─────┘    └────┬─────┘    └────┬─────┘
     │               │               │
     └───────────────┼───────────────┘
                     │
                     ▼
              ┌─────────────┐
              │events.jsonl │ ◄── CONTENTION!
              └─────────────┘      Writers block each other
```

### After (Per-Session Files)

```
Solution:
┌──────────┐    ┌──────────┐    ┌──────────┐
│Session A │    │Session B │    │Session C │
└────┬─────┘    └────┬─────┘    └────┬─────┘
     │               │               │
     ▼               ▼               ▼
┌─────────┐    ┌─────────┐    ┌─────────┐
│ A.jsonl │    │ B.jsonl │    │ C.jsonl │  ◄── NO CONTENTION!
└─────────┘    └─────────┘    └─────────┘
     │               │               │
     └───────────────┼───────────────┘
                     │
                     ▼
              ┌─────────────┐
              │ Sync Engine │ (merges all)
              └─────────────┘
```

---

## References

1. **SQLite WAL Mode** - https://sqlite.org/wal.html
2. **SQLite File Locking** - https://www.sqlite.org/lockingv3.html
3. **Kafka Design** - https://kafka.apache.org/documentation/#design
4. **Redis Persistence** - https://redis.io/docs/management/persistence/
5. **LevelDB Implementation** - https://github.com/google/leveldb/blob/main/doc/impl.md
6. **PostgreSQL WAL** - https://www.postgresql.org/docs/current/wal-intro.html

---

## Future Considerations

1. **Automatic Cleanup**: Delete session files older than retention period
2. **Compression**: Gzip old session files to save space
3. **Merge Files**: Periodically merge small session files into archives
4. **Shared Memory Coordination**: Like SQLite's `-shm` file for faster sync

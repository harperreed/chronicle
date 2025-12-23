# Sync Repair Command Design

## Overview

Add database repair functionality to chronicle via a new `kv.Repair()` function in the charm library, exposed through `chronicle sync repair` and `chronicle sync reset` commands.

## Problem

SQLite databases using WAL mode can become corrupted, particularly the SHM (shared memory) file. Users currently need to manually run sqlite3 commands to recover. This should be a built-in repair command.

## Charm KV Library API

```go
// In github.com/charmbracelet/charm/kv

// RepairResult contains details of repair operations performed.
type RepairResult struct {
    WalCheckpointed   bool   // WAL was checkpointed into main DB
    ShmRemoved        bool   // Stale SHM file was removed
    IntegrityOK       bool   // Database passed integrity check
    Vacuumed          bool   // Database was vacuumed
    RecoveryAttempted bool   // REINDEX recovery was attempted
    ResetFromCloud    bool   // Local DB was reset from cloud
    Error             error  // Non-fatal warning (e.g., vacuum skipped)
}

// Repair attempts to fix a corrupted database.
// Steps: checkpoint WAL -> remove SHM -> integrity check -> vacuum
// If force=true and integrity fails, attempts REINDEX recovery,
// then resets from cloud as last resort.
func Repair(name string, force bool, opts ...Option) (*RepairResult, error)
```

## Chronicle CLI Commands

### `chronicle sync repair`

```
Usage: chronicle sync repair [--force]

Repair a corrupted local database.

Steps performed:
  1. Checkpoint WAL (merge pending writes into main DB)
  2. Remove stale SHM file
  3. Run integrity check
  4. Vacuum database

Flags:
  --force   If corruption persists, attempt recovery and reset from cloud

Examples:
  chronicle sync repair          # Safe repair
  chronicle sync repair --force  # Aggressive repair with cloud reset fallback
```

### `chronicle sync reset`

```
Usage: chronicle sync reset

Discard local database and re-download from Charm Cloud.
Useful when local data is corrupted beyond repair.

Requires confirmation prompt.
```

Note: This differs from `sync wipe` which deletes data from cloud too.

## Repair Implementation Logic

```
Repair(name, force) flow:

1. Locate database files:
   - {dataDir}/kv/{name}.db
   - {dataDir}/kv/{name}.db-wal
   - {dataDir}/kv/{name}.db-shm

2. Open DB with sqlite3 (direct, not through KV):
   - PRAGMA wal_checkpoint(TRUNCATE)
   - Close connection

3. Remove SHM file if exists (it's recreated on next open)

4. Reopen and check integrity:
   - PRAGMA integrity_check
   - If "ok" -> continue
   - If not "ok" and !force -> return error with suggestion
   - If not "ok" and force -> goto step 5

5. Recovery attempt (force only):
   - PRAGMA writable_schema=ON
   - REINDEX
   - PRAGMA writable_schema=OFF
   - Re-check integrity
   - If still broken -> goto step 6

6. Cloud reset (force only):
   - Close and delete local DB files
   - Open fresh KV with same name
   - Call Sync() to pull from cloud

7. Vacuum (if integrity OK):
   - VACUUM
   - Return success
```

## CLI Output Examples

**Successful repair:**
```
$ chronicle sync repair
Repairing chronicle database...
  ✓ WAL checkpointed
  ✓ SHM file removed
  ✓ Integrity check passed
  ✓ Database vacuumed

Repair complete.
```

**Corruption found, no --force:**
```
$ chronicle sync repair
Repairing chronicle database...
  ✓ WAL checkpointed
  ✓ SHM file removed
  ✗ Integrity check failed: database disk image is malformed

Repair incomplete. Run with --force to attempt recovery and cloud reset.
```

**Force repair with cloud reset:**
```
$ chronicle sync repair --force
Repairing chronicle database...
  ✓ WAL checkpointed
  ✓ SHM file removed
  ✗ Integrity check failed
  ✗ Recovery attempt failed
  ✓ Reset from cloud (47 entries restored)

Repair complete.
```

## Implementation Plan

1. **Charm library** (separate PR to 2389-research/charm):
   - Add `Repair(name string, force bool, opts ...Option) (*RepairResult, error)`
   - Add `Reset(name string, opts ...Option) error` for cloud-only reset

2. **Chronicle CLI**:
   - Add `syncRepairCmd` with `--force` flag
   - Add `syncResetCmd` with confirmation prompt
   - Wire up to charm's `kv.Repair()` and display results

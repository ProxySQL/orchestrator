# PostgreSQL Graceful Primary Switchover

**Date:** 2026-04-18
**Status:** Approved

## Summary

Implement planned/graceful primary switchover for PostgreSQL in orchestrator. Currently only unplanned failover (dead primary) is supported. This adds the ability to gracefully demote a running primary and promote a designated standby with zero data loss, reusing the same CLI commands and API endpoints as MySQL's graceful master takeover.

## Background

Orchestrator already supports PostgreSQL unplanned failover via `checkAndRecoverDeadPrimary()` in `topology_recovery_postgresql.go`. The graceful takeover commands (`graceful-master-takeover`, `graceful-master-takeover-auto`) exist but dispatch only to MySQL-specific code in `GracefulMasterTakeover()`. PostgreSQL needs its own implementation following the established pattern of separate provider-specific recovery functions.

## Design

### New Instance Operations

Three new functions in `go/inst/instance_topology_postgresql.go`:

#### `PostgreSQLSetReadOnly(instanceKey *InstanceKey, readOnly bool) (*Instance, error)`

- Connects to the instance via SQL
- Executes `ALTER SYSTEM SET default_transaction_read_only = on|off`
- Calls `SELECT pg_reload_conf()`
- When setting read-only: queries `pg_stat_activity` for non-replication, non-orchestrator backends and calls `pg_terminate_backend()` on each to close the write window
- Re-reads and returns the instance

#### `PostgreSQLGetCurrentWALLSN(instanceKey *InstanceKey) (string, error)`

- Connects to the primary
- Returns result of `SELECT pg_current_wal_lsn()::text`

#### `PostgreSQLWaitForStandbyLSN(standbyKey *InstanceKey, targetLSN string, timeout time.Duration) error`

- Polls `SELECT pg_last_wal_replay_lsn()` on the standby
- Compares using `SELECT '%s'::pg_lsn >= '%s'::pg_lsn` (PostgreSQL-native LSN comparison)
- Polls every 500ms up to the configured timeout
- Returns error on timeout

#### `PostgreSQLRepositionAsStandby(instanceKey *InstanceKey, newPrimaryKey *InstanceKey) error`

- Connects to the old primary
- Sets `primary_conninfo` via `ALTER SYSTEM` pointing to the new primary (same logic as existing `PostgreSQLReconfigureStandby`)
- Calls `SELECT pg_reload_conf()`
- Does NOT restart the instance — the actual restart and `standby.signal` creation is the operator's responsibility via `PostGracefulTakeoverProcesses` hook

### Switchover Orchestration

New function `PostgreSQLGracefulPrimarySwitchover(clusterName string, designatedKey *InstanceKey, auto bool) (*TopologyRecovery, *inst.BinlogCoordinates, error)` in `go/logic/topology_recovery_postgresql.go`:

**Flow:**

1. **Validate** — Read cluster primary, read standbys, determine designated standby. When `auto=true`, use `PostgreSQLGetBestStandbyForPromotion()`. When manual, require the user to specify or have exactly one standby.
2. **Execute `PreGracefulTakeoverProcesses`** — Abort on failure.
3. **Set primary read-only** — `PostgreSQLSetReadOnly(primary, true)`. Terminates existing non-replication connections.
4. **Capture target LSN** — `PostgreSQLGetCurrentWALLSN(primary)`.
5. **Wait for standby catch-up** — `PostgreSQLWaitForStandbyLSN(designated, targetLSN, timeout)`.
6. **Promote designated standby** — `PostgreSQLPromoteStandby(designated)` (existing function).
7. **Reconfigure remaining standbys** — `PostgreSQLReconfigureStandby(standby, newPrimary)` for each non-designated standby (existing function).
8. **Configure demoted primary** — `PostgreSQLRepositionAsStandby(oldPrimary, newPrimary)`. Sets `primary_conninfo` only; operator handles restart via hook.
9. **Execute `PostGracefulTakeoverProcesses`**.
10. **Return** TopologyRecovery with successor key.

**Error handling:**

- If promotion fails after setting read-only: undo via `PostgreSQLSetReadOnly(primary, false)`, abort.
- If standby catch-up times out: undo read-only, abort with error.

**Note on flat topology:** PostgreSQL topologies are flat (no cascading replication detected), so there is no sibling relocation step (unlike MySQL which may need to reparent replicas under the designated replica).

### CLI/API Dispatch

No changes to `cli.go` or `api.go`. The existing commands and endpoints are reused.

**In `go/logic/topology_recovery.go`:**

Add a provider check in `GracefulMasterTakeover()`:

```go
if clusterMaster.DatabaseProvider == inst.PostgreSQL {
    return PostgreSQLGracefulPrimarySwitchover(clusterName, designatedKey, auto)
}
```

Add a PostgreSQL branch in `getGracefulMasterTakeoverDesignatedInstance()` to use `PostgreSQLGetBestStandbyForPromotion()` when `auto=true`.

### Testing

- Unit tests for the 3 new instance operations
- Unit test for `PostgreSQLGracefulPrimarySwitchover` flow
- Functional test in `tests/functional/` — run a graceful switchover against a real PostgreSQL topology and verify the new primary is writable and old primary is configured for standby

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Both manual and auto variants | Yes | Match MySQL parity for consistent UX |
| Read-only mechanism | `ALTER SYSTEM` + `pg_terminate_backend()` | Mirrors MySQL `SET GLOBAL read_only=1` behavior |
| Standby catch-up | Poll `pg_last_wal_replay_lsn()` on standby | Simple, doesn't depend on primary staying up during wait |
| Implementation location | Separate `PostgreSQLGracefulPrimarySwitchover()` | Follows existing pattern of separate provider functions |
| Demoted primary restart | Operator's responsibility via hook | Orchestrator connects via SQL only, no shell access assumed |
| Old primary repositioning | `ALTER SYSTEM SET primary_conninfo` only | No timeline divergence since primary is read-only before switchover, so `pg_rewind` is unnecessary |

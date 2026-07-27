# MariaDB DeadMaster False Detection and Relay Drain Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix false DeadMaster when replica IO is running, and drain relay logs safely on MariaDB before promotion so AFTER_SYNC committed data is not lost.

**Architecture:** Add `CountValidIORunningReplicas` to analysis and prefer `UnreachableMaster` when IO is up. Add `DrainRelayLogs` (SQL-thread only) for MariaDB-safe catch-up. In candidate selection, skip undrainable replicas. Split GTID regroup so siblings are re-pointed only after a drained, validated candidate exists.

**Tech Stack:** Go, existing `go/inst` + `go/logic` packages, unit tests via `go test`, functional tests under `tests/functional/`.

**Spec:** `docs/superpowers/specs/2026-07-16-mariadb-deadmaster-relay-drain-design.md`

---

### File map

| File | Responsibility |
|------|----------------|
| `go/inst/analysis.go` | Field `CountValidIORunningReplicas` |
| `go/inst/analysis_dao.go` | SQL aggregate + DeadMaster guards + UnreachableMaster when IO>0 |
| `go/inst/analysis_classification.go` (new) | Pure helper `classifyUnreachableMasterAnalysis` for unit tests |
| `go/inst/analysis_classification_test.go` (new) | Table tests for IO-up vs DeadMaster |
| `go/inst/instance_topology_dao.go` | `DrainRelayLogs`, MariaDB path in `StopReplicas` |
| `go/inst/instance_topology.go` | Skip undrainable candidates; split select vs move in GTID regroup |
| `go/logic/topology_recovery.go` | Validate before re-point; keep `startReplicationOnCandidate=false` |
| `tests/functional/test-mariadb-relay-drain.sh` | Functional CI |
| `.github/workflows/functional.yml` | Wire MariaDB job |
| `docs/failure-detection.md` | Document IO-running behavior |

---

### Task 1: Analysis — CountValidIORunningReplicas + classification

**Files:**
- Modify: `go/inst/analysis.go`
- Modify: `go/inst/analysis_dao.go`
- Create: `go/inst/analysis_classification.go`
- Create: `go/inst/analysis_classification_test.go`

- [ ] **Step 1: Add field and failing classification tests**

Add to `ReplicationAnalysis`:
```go
CountValidIORunningReplicas uint
```

Create pure function used by DAO after filling counts (or call from classification site):
```go
// shouldAvoidDeadMasterDueToIORunning reports whether any valid replica still
// has IO running, which proves the master is accepting replica connections.
func shouldAvoidDeadMasterDueToIORunning(countValidIORunningReplicas uint) bool {
	return countValidIORunningReplicas > 0
}
```

Test: IO>0 → avoid dead master; IO==0 → do not avoid.

- [ ] **Step 2: SQL + mapping + branch order**

In analysis query add:
```sql
IFNULL(SUM(
  replica_instance.last_checked <= replica_instance.last_seen
  AND replica_instance.slave_io_running != 0
), 0) AS count_valid_io_running_replicas
```

Map into struct. Before `DeadMaster` branches when `IsMaster && !LastCheckValid && CountValidIORunningReplicas > 0`, set `UnreachableMaster`. Add `&& CountValidIORunningReplicas == 0` to all `DeadMaster*` master branches.

- [ ] **Step 3: `go test ./go/inst/ -run Classification -count=1`**

- [ ] **Step 4: Commit** `fix(analysis): treat IO-running replicas as UnreachableMaster not DeadMaster`

---

### Task 2: DrainRelayLogs + MariaDB StopReplicas

**Files:**
- Modify: `go/inst/instance_topology_dao.go`
- Test: unit tests for `SQLThreadUpToDate` already exist; add `DrainRelayLogs` documentation tests if no DB mock

- [ ] **Step 1: Implement `DrainRelayLogs`**

```go
func DrainRelayLogs(instanceKey *InstanceKey, timeout time.Duration) (*Instance, error) {
	// start SQL only via QSP.start_slave_sql_thread()
	// WaitForSQLThreadUpToDate
	// stop slave both threads
}
```

- [ ] **Step 2: In `StopReplicas`, MariaDB uses `DrainRelayLogs` instead of skipping nice stop**

- [ ] **Step 3: Commit** `fix(inst): MariaDB-safe DrainRelayLogs for nice stop`

---

### Task 3: Skip undrainable candidates

**Files:**
- Modify: `go/inst/instance_topology.go` (`GetCandidateReplica`)

- [ ] **Step 1: After chooseCandidateReplica when forRematchPurposes, ensure drained**

Loop: if candidate not SQL up to date, call `DrainRelayLogs`; on failure remove candidate from list and `chooseCandidateReplica` again; if none left return error.

- [ ] **Step 2: Unit test with mocked drain is hard without interfaces — test remove-and-retry logic via small helper `pickDrainableCandidate` if extracted**

- [ ] **Step 3: Commit** `fix(inst): skip candidates that cannot drain relay logs`

---

### Task 4: Re-point only after validation

**Files:**
- Modify: `go/inst/instance_topology.go` (`RegroupReplicasGTID`)
- Modify: `go/logic/topology_recovery.go`

- [ ] **Step 1: Split GTID regroup**

`RegroupReplicasGTID` selects drained candidate first; run optional `validateCandidate func(*Instance) error` before `moveReplicasViaGTID`. On validate error, return without moving.

- [ ] **Step 2: Dead master passes validation for SQL up-to-date (always) and geo/lag config**

Move critical checks from `overrideMasterPromotion` into pre-move validate for GTID path, or call shared helper before move inside `recoverDeadMaster` via new API:

```go
SelectGTIDFailoverCandidate(...)
// validate
MoveReplicasViaGTIDUnderCandidate(...)
```

- [ ] **Step 3: Commit** `fix(recovery): validate drained candidate before re-pointing siblings`

---

### Task 5: Functional CI MariaDB

**Files:**
- Create: `tests/functional/test-mariadb-relay-drain.sh`
- Modify: compose / workflow for MariaDB image
- Modify: `.github/workflows/functional.yml`

- [ ] **Step 1: Script** — MariaDB topology, stop SQL / leave IO, assert no false DeadMaster when possible; forced failover asserts no data loss or clean failure

- [ ] **Step 2: Wire workflow job**

- [ ] **Step 3: Commit** `test: MariaDB relay drain / false DeadMaster functional coverage`

---

### Task 6: Docs + final verification

- [ ] Update `docs/failure-detection.md` DeadMaster note
- [ ] `go test ./go/inst/ ./go/logic/ -count=1`
- [ ] `gofmt -s -w` on touched files
- [ ] Commit docs; open PR against issue #106

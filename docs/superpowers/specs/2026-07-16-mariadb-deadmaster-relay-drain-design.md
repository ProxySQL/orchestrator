# MariaDB DeadMaster False Detection and Relay Drain

**Date:** 2026-07-16  
**Status:** Approved  
**Issue:** https://github.com/ProxySQL/orchestrator/issues/106

## Summary

Fix false `DeadMaster` detection when replica SQL threads are stopped but IO threads still prove the master is reachable, and prevent data loss on MariaDB (and generally under semi-sync AFTER_SYNC) by draining relay logs with a MariaDB-safe SQL-thread-only path before promotion. Recovery must skip candidates that cannot drain, try the next, fail if none can, and only re-point siblings after a drainable candidate is validated.

## Background

Issue #106 documents five coupled defects (verified on current `master`):

1. **False DeadMaster** — `count_valid_replicating_replicas` requires both IO and SQL running. IO-up/SQL-down yields zero “replicating” replicas; if orchestrator cannot reach the master, analysis becomes `DeadMaster` instead of `UnreachableMaster`.
2. **MariaDB skips `StopReplicationNicely`** — intentional (`b381c698`) because MariaDB GTID can drop relay logs on IO thread restart; side effect is relay logs are never drained.
3. **`startReplicationOnCandidate=false`** on dead-master GTID recovery — correct (must not replicate from a dead master), but combined with (2) means unapplied relay events are never applied.
4. **`WaitForSQLThreadUpToDate` is poll-only** — never starts SQL thread; useless if SQL is stopped.
5. **Siblings re-pointed before promotion validation** — `RegroupReplicasGTID` moves replicas, then `overrideMasterPromotion` may fail with no rollback.

With `rpl_semi_sync_master_wait_point=AFTER_SYNC`, every committed transaction is guaranteed in at least one replica’s relay log. Failing to apply those events before promotion turns zero-data-loss into data loss.

## Goals

- Do not declare `DeadMaster` when any valid replica still has IO running.
- Before promoting a replica in dead-master recovery, apply remaining relay-log events without MariaDB-unsafe IO restart.
- Prefer the next candidate when the best candidate cannot drain; fail recovery if none can.
- Re-point siblings only after a drainable candidate is selected and validated.
- Cover with unit tests and a functional CI scenario for MariaDB AFTER_SYNC + SQL stopped.

## Non-goals

- New public analysis codes (reuse `UnreachableMaster`).
- New configuration flags for this behavior (always-on safety).
- Changing PostgreSQL recovery paths.
- Automatically starting SQL in every historical caller of `WaitForSQLThreadUpToDate`.
- Setting `startReplicationOnCandidate=true` on dead-master recovery (would point at dead master).

## Design

### 1. Detection: independent IO-running signal

**Files:** `go/inst/analysis.go`, `go/inst/analysis_dao.go` (MySQL path; PG path only if the same metric is needed for shared structs).

Add:

```sql
IFNULL(
  SUM(
    replica_instance.last_checked <= replica_instance.last_seen
    AND replica_instance.slave_io_running != 0
  ),
  0
) AS count_valid_io_running_replicas
```

Map to `AnalysisInstance.CountValidIORunningReplicas` (name aligned with existing `CountValid*` fields).

**Master analysis when `!LastCheckValid`:**

- If `CountValidIORunningReplicas > 0`, classify as **`UnreachableMaster`** (same meaning: master likely alive; network/orchestrator reachability issue), **before** any `DeadMaster*` branch that requires `CountValidReplicatingReplicas == 0`.
- Existing `DeadMaster*` paths remain only when there is no valid IO-running replica (and current replica-count conditions hold).

Co-master / intermediate-master: apply the same IO-running guard only where the identical IO+SQL AND conflation would force a dead analysis; keep scope minimal.

**Rationale:** Reuse `UnreachableMaster` so recovery hooks, UI, and docs stay stable. IO running without connect errors is positive evidence the master accepts replica connections.

### 2. Drain relay logs without restarting IO

**File:** `go/inst/instance_topology_dao.go` (and thin wrappers if needed).

New function:

```go
// DrainRelayLogs starts the SQL thread only (does not touch IO thread),
// waits until SQL is aligned with IO/relay, then stops replication.
// Safe for MariaDB GTID: does not stop/start IO (which can drop relay logs).
func DrainRelayLogs(instanceKey *InstanceKey, timeout time.Duration) (*Instance, error)
```

**Steps:**

1. `ReadTopologyInstance`
2. If already `SQLThreadUpToDate()`, return success (optionally still stop both threads if caller requires stopped state for rematch — match `StopReplicationNicely` end state: replication stopped and aligned).
3. Issue `START SLAVE SQL_THREAD` / provider equivalent (`QSP.start_slave_sql_thread()`).
4. Wait for progress using existing wait logic (SQL up to date vs IO/relay coordinates), with overall and stale timeouts.
5. Issue full `STOP SLAVE` (both threads) so the instance ends stopped and aligned, same as nice-stop.
6. Re-read and return instance.

**Do not** call `stop slave io_thread` as part of drain.

**Relationship to existing helpers:**

- `StopReplicationNicely` remains for non-MariaDB paths that already work; MariaDB path in `StopReplicas` should call `DrainRelayLogs` instead of skipping nice-stop entirely:

```go
if stopReplicationMethod == StopReplicationNice {
  if replica.IsMariaDB() {
    DrainRelayLogs(&replica.Key, timeout)
  } else {
    StopReplicationNicely(&replica.Key, timeout)
  }
}
replica, _ = StopReplication(&replica.Key) // idempotent if already stopped
```

Alternatively, implement `StopReplicationNicely` to branch internally (MariaDB → drain-only; others → current IO stop + SQL start). Prefer one public entry point if it reduces call-site drift.

**`WaitForSQLThreadUpToDate`:** remains poll-only. Drain path starts SQL before calling it. Optional `ensureSQLRunning` parameter is allowed only if it keeps existing callers unchanged (default false).

### 3. Candidate selection: skip undrainable replicas

**Files:** `go/inst/instance_topology.go` (`GetCandidateReplica`, `chooseCandidateReplica`, `RegroupReplicasGTID`).

When `forRematchPurposes` / dead-master regroup:

1. Sort replicas as today.
2. For each candidate in preference order:
   - Attempt drain (via nice-stop / `DrainRelayLogs` as above).
   - If drain fails (SQL error e.g. 1062, timeout, coordinates never advance): mark skipped, try next.
3. If no candidate drains successfully: return error; **do not** promote and **do not** re-point siblings.
4. Winner is the first drainable candidate that also satisfies existing promotion rules (must-not-promote, etc.).

Audit log each skip reason.

### 4. Dead-master recovery ordering

**Files:** `go/logic/topology_recovery.go`, `go/inst/instance_topology.go`.

Restructure GTID dead-master recovery into phases:

| Phase | Action | On failure |
|-------|--------|------------|
| 1 | Select + drain candidate (try next on drain failure) | Abort recovery; no topology mutation of siblings |
| 2 | Promotion validation (geo constraints, lag config, SQL up-to-date) | Abort; no sibling re-point |
| 3 | `moveReplicasViaGTID` re-point siblings under candidate | Existing error handling |
| 4 | Apply MySQL promotion (`read_only`, detach, etc.) as today | Existing error handling |

Concrete approach:

- Split or parameterize `RegroupReplicasGTID` so dead-master can:
  - obtain drained candidate without moving siblings, then
  - run validation, then
  - move siblings.
- Or: `RegroupReplicasGTID` performs drain+select first, validates via callback, then moves.

**Do not** set `startReplicationOnCandidate=true` for dead master (would start replication toward the failed master). Drain applies local relay only; promotion path detaches the new master.

Keep comment “no going back” only after successful promote decision (after phase 2/3 success), not after a failed override with already-repointed replicas.

### 5. Interaction with existing promotion knobs

- `FailMasterPromotionIfSQLThreadNotUpToDate` / `DelayMasterPromotionIfSQLThreadNotUpToDate` remain valid; after drain, SQL should be up to date so delay path is rarely needed.
- Drain failure already excludes the candidate; these flags are a second line of defense, not the primary fix.
- No default config change required for correctness of the new path.

## Testing

### Unit tests

- **Analysis:** fixtures where master unreachable + replicas IO up/SQL down → `UnreachableMaster`; both threads down → `DeadMaster` (existing conditions).
- **DrainRelayLogs:** mock/DAO-level tests if pattern exists; otherwise testable helpers for “already up to date”, “starts SQL then waits”.
- **Candidate skip:** with a list where first replica fails drain and second succeeds, second is chosen; all fail → error.
- **Ordering:** regroup does not call move-via-GTID when no drainable candidate (table-driven / interface seams as needed).

### Functional CI

Add a MariaDB-focused functional test (new script or matrix job), e.g. `tests/functional/test-mariadb-relay-drain.sh`, wired into `.github/workflows/functional.yml`:

**Scenario A — false DeadMaster avoided**

1. MariaDB 10.6 or 10.11 topology: 1 master, 2 replicas, GTID, semi-sync AFTER_SYNC if available in image.
2. Stop SQL thread on replicas (or inject 1062) while leaving IO running.
3. Make master unreachable to orchestrator without killing replica IO paths if possible; if full master kill is used, document that IO will also fail — for false-positive focus, prefer orchestrator-side unreachability or short connection failure while replicas still show IO=Yes.
4. Assert analysis is **not** `DeadMaster` when IO still runs; expect `UnreachableMaster` or non-dead analysis.

**Scenario B — forced failover does not lose relay events**

1. Topology with AFTER_SYNC semi-sync.
2. Write committed transactions; stop SQL on preferred candidate while IO has received events (or stop SQL after events are in relay).
3. Kill master; trigger/wait for recovery.
4. Assert promoted master contains the committed rows that were only in relay at failure time (or assert recovery refused if no candidate could drain — both are acceptable; data loss is not).

If full AFTER_SYNC matrix is heavy, minimum bar: MariaDB image + SQL stopped + master kill + assert either successful drain+promote with data present, or clean recovery failure without promoting a stale applied state.

Reuse `tests/functional/lib.sh` helpers and compose patterns; add MariaDB service definitions as needed (separate compose override or image env like MySQL matrix).

## Error handling

| Condition | Behavior |
|-----------|----------|
| IO running on any valid replica, master check invalid | `UnreachableMaster`, not `DeadMaster` |
| Candidate drain fails | Skip candidate, try next |
| All candidates fail drain | Recovery fails; no sibling re-point |
| Drain succeeds, later promotion apply fails | Existing recovery failure metrics; siblings may already be re-pointed only if phase 3 completed — prefer fail before phase 3 |
| MariaDB IO restart elsewhere | Unchanged avoidance in emergency restart path |

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Scope | Full issue #106 (all five problems) | User request; problems are coupled |
| Approach | Targeted surgical fix | Minimal surface vs new analysis/recovery pipeline |
| Undrainable candidate | Skip and try next | Prefer another replica over hard-fail on first broken SQL thread |
| Analysis code | Reuse `UnreachableMaster` | IO-up means master likely alive; avoid hook matrix growth |
| MariaDB drain | SQL thread only | Avoid known relay-log drop on IO stop/start |
| `startReplicationOnCandidate` | Stay false on dead master | Must not replicate from dead master; drain is local |
| Config flags | None new | Always-on correctness |
| Functional CI | Required | MariaDB AFTER_SYNC / SQL-stopped scenario |

## Implementation notes

- Follow repo conventions: `gofmt -s`, DCO sign-off (`git commit -s`), tests in package `inst` / functional scripts.
- Line numbers in issue #106 may drift; match symbols and conditions, not exact lines.
- Document behavior briefly in `docs/failure-detection.md` and/or `docs/topology-recovery.md` if those describe DeadMaster criteria.

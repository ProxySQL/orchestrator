# Final safety-fix report

Date: 2026-08-12 (Asia/Bangkok)

## Outcome

All three Important safety findings are addressed in one focused change set.
No live failover was run and no MySQL container was started, stopped, or
recreated during this correction.

## TDD evidence

Initial RED command:

```text
go test ./go/http -run 'Test(AuditFailoverHarnessSafetyContracts|SmokeEndsOnlyMaintenanceCreatedByItsBeginCall|MaintenanceBegunResponseReturnsCreatedMaintenanceKey)$' -count=1
```

It failed at compile time with:

```text
go/http/api_test.go:60:14: undefined: maintenanceBegunResponse
```

After the minimal handler response helper was introduced, the same command
failed on the shell regressions: missing `deadline=$((SECONDS + 90))`, curl
max-time two, deadline loop, cleanup early-return, and keyed end-maintenance;
it also detected `start mysql2 mysql3` and instance-based maintenance cleanup.

A focused boundary RED then failed because the deadline loop did not budget its
final curl against the remaining seconds. That test named the missing
`remaining=$((deadline - SECONDS))` and reduced curl argument.

Final focused GREEN:

```text
ok github.com/proxysql/orchestrator/go/http
```

## Implemented contracts

1. Recovery polling is bounded by an actual 90-second wall-clock deadline.
   Every curl has a two-second maximum, reduced to the remaining deadline
   budget when necessary, and success/failure output uses actual elapsed time.
2. `restore_lab` is a true no-op while `MYSQL1_STOPPED=false`. Once mysql1 was
   stopped, cleanup starts mysql1 only. It never starts mysql2/mysql3; replica
   repair uses `docker compose exec` and therefore operates only on replicas
   that are already running.
3. BeginMaintenance success preserves the historical `Details.Hostname` and
   `Details.Port` fields and adds its new maintenance ID as
   `Details.MaintenanceKey`, while preserving `Code: OK` and the existing
   Message. Smoke validates the direct response's status, code, exact instance
   message, instance details, and positive integer key, then calls only
   `/api/end-maintenance/$MAINTENANCE_KEY`. Failed or unrelated responses cause
   no cleanup call.

## Additive API compatibility correction

The initial safety correction represented `Details` as the maintenance-key
number, which regressed the successful BeginMaintenance response contract for
clients that read `Details.Hostname` and `Details.Port`. A focused TDD test
against that implementation failed with:

```text
json: cannot unmarshal number into Go struct field .Details of type struct { Hostname string; Port int; MaintenanceKey int64 }
```

The response now embeds the original `inst.InstanceKey` fields in its details
object and exposes `MaintenanceKey` additively. Failure responses were not
changed.

## Verification

- `go test ./go/http -count=1`: pass.
- Node UI state tests: 23/23 pass across four files.
- `bash -n tests/functional/test-audit-ui-failover.sh tests/functional/test-smoke.sh`: pass.
- `bash tests/functional/test-smoke.sh`: 35 passed, 0 failed, 0 skipped;
  begin returned `Details.MaintenanceKey` 1 alongside `Hostname`/`Port`, and
  cleanup ended that exact key.
- `git diff --check`: pass.
- Live failover: intentionally not run.

Only Orchestrator was recreated for smoke; a before/after comparison confirmed
all three MySQL container IDs were unchanged. An initial run failed at the
readiness gate because the mounted binary was Darwin rather than Linux; no
maintenance began. Rebuilding with the existing Linux/arm64 Go image resolved
the environment mismatch, after which smoke passed.

## Concerns

None for the three corrected findings. Recreating Orchestrator resets the
functional SQLite audit database by design, so historical live failover rows
from the earlier review are no longer resident; their captured evidence remains
in `final-report.md`. No new failover was run.

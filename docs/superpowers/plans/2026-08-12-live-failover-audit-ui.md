# Live Failover and Audit UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Generate a real MySQL `DeadMaster` recovery, restore the original three-node topology, and verify Orchestrator's populated audit, failure-detection, and recovery UI states.

**Architecture:** Enable audit persistence in the functional SQLite backend, then use a dedicated shell harness built from the repository's existing functional-test helpers. The harness owns the complete stop/recovery/restore lifecycle through a cleanup trap, while browser inspection validates the real APIs and triggers focused TDD fixes only when an observable populated-state defect is found.

**Tech Stack:** Bash, Docker Compose, MySQL 8.4, ProxySQL, Orchestrator HTTP API, Go template/static tests, Node `node:test`, in-app Browser.

## Global Constraints

- Preserve all MySQL volumes and container identities.
- Never run `docker compose down`, volume removal, or unscoped `docker compose up`.
- Recreate only Orchestrator and always use `--no-deps`.
- Stop and restart only the resolved current primary container.
- Use bounded waits and restore the original topology from a cleanup trap on success or failure.
- Final topology is `mysql1` writable primary with `mysql2` and `mysql3` running replicas.
- Keep generated Orchestrator SQLite history available for browser inspection.
- Production fixes require a focused failing test observed before implementation.

---

### Task 1: Enable functional audit persistence

**Files:**
- Modify: `tests/functional/orchestrator-test.conf.json`
- Test: `tests/functional/test-smoke.sh`

**Interfaces:**
- Consumes: the existing SQLite functional backend at `/tmp/orchestrator-test.sqlite3`.
- Produces: persisted `/api/audit/0` entries for topology and recovery operations in the current Orchestrator session.

- [ ] **Step 1: Add the functional configuration value**

Add this top-level JSON property next to `BackendDB`:

```json
"AuditToBackendDB": true,
```

- [ ] **Step 2: Validate the configuration and recreate only Orchestrator**

Run:

```bash
python3 -m json.tool tests/functional/orchestrator-test.conf.json >/dev/null
docker ps --filter 'name=functional-mysql' --format '{{.ID}} {{.Names}}' >/tmp/orchestrator-audit-mysql-before
docker compose -f tests/functional/docker-compose.yml up -d --no-deps --force-recreate orchestrator
```

Expected: valid JSON; only `functional-orchestrator-1` is recreated.

- [ ] **Step 3: Wait for readiness and generate a baseline audit entry**

Run:

```bash
for attempt in $(seq 1 60); do
  curl -fsS http://localhost:3099/api/clusters >/dev/null && break
  sleep 1
done
curl -fsS http://localhost:3099/api/discover/mysql1/3306 >/dev/null
```

Expected: Orchestrator becomes reachable and discovery succeeds.

- [ ] **Step 4: Verify MySQL identities did not change**

Run:

```bash
docker ps --filter 'name=functional-mysql' --format '{{.ID}} {{.Names}}' >/tmp/orchestrator-audit-mysql-after
diff -u /tmp/orchestrator-audit-mysql-before /tmp/orchestrator-audit-mysql-after
```

Expected: no diff.

- [ ] **Step 5: Commit**

```bash
git add tests/functional/orchestrator-test.conf.json
git commit -m "test(ui): persist audit history in functional lab"
```

---

### Task 2: Add and run the controlled failover harness

**Files:**
- Create: `tests/functional/test-audit-ui-failover.sh`
- Reuse: `tests/functional/lib.sh`

**Interfaces:**
- Consumes: `wait_for_orchestrator`, `discover_topology`, `mysql_read_only`, `mysql_stop_replica_sql`, `mysql_reset_replica_all_sql`, `mysql_change_source_sql`, `mysql_start_replica_sql`, and `proxysql_servers` from `tests/functional/lib.sh`.
- Produces: a restored three-node topology plus non-empty `/api/audit/0`, `/api/audit-failure-detection/0`, and `/api/audit-recovery/0` responses.

- [ ] **Step 1: Create the safety-gated harness**

The script must:

```bash
#!/bin/bash
set -uo pipefail
cd "$(dirname "$0")/../.."
source tests/functional/lib.sh

COMPOSE="docker compose -f tests/functional/docker-compose.yml"
BEFORE_IDS="$(mktemp)"
AFTER_IDS="$(mktemp)"
MYSQL1_STOPPED=false

restore_lab() {
  if [ "$MYSQL1_STOPPED" = true ]; then
    $COMPOSE start mysql1 >/dev/null
  fi
  for attempt in $(seq 1 60); do
    $COMPOSE exec -T mysql1 mysqladmin ping -h localhost -uroot -ptestpass >/dev/null 2>&1 && break
    sleep 1
  done

  local stop_sql reset_sql change_sql start_sql
  stop_sql=$(mysql_stop_replica_sql)
  reset_sql=$(mysql_reset_replica_all_sql)
  change_sql=$(mysql_change_source_sql mysql1 3306 repl repl_pass)
  start_sql=$(mysql_start_replica_sql)

  $COMPOSE exec -T mysql1 mysql -uroot -ptestpass \
    -e "$stop_sql $reset_sql SET GLOBAL read_only=0;" >/dev/null 2>&1 || true
  for replica in mysql2 mysql3; do
    $COMPOSE exec -T "$replica" mysql -uroot -ptestpass \
      -e "$stop_sql $change_sql $start_sql SET GLOBAL read_only=1;" >/dev/null 2>&1 || true
  done
  $COMPOSE exec -T proxysql mysql -h127.0.0.1 -P6032 -uradmin -pradmin \
    -e "DELETE FROM mysql_servers WHERE hostgroup_id IN (10,20); INSERT INTO mysql_servers (hostgroup_id,hostname,port) VALUES (10,'mysql1',3306),(20,'mysql2',3306),(20,'mysql3',3306); LOAD MYSQL SERVERS TO RUNTIME; SAVE MYSQL SERVERS TO DISK;" >/dev/null 2>&1 || true
  curl -fsS "$ORC_URL/api/discover/mysql1/3306" >/dev/null || true
  curl -fsS "$ORC_URL/api/discover/mysql2/3306" >/dev/null || true
  curl -fsS "$ORC_URL/api/discover/mysql3/3306" >/dev/null || true
}

trap 'restore_lab' EXIT
```

After the cleanup definition, the script must:

1. record MySQL container IDs;
2. call `wait_for_orchestrator` and `discover_topology mysql1`;
3. require `mysql1` read-only `0`, `mysql2` read-only `1`, and ProxySQL writer `mysql1`;
4. stop `mysql1` and set `MYSQL1_STOPPED=true`;
5. poll `/api/v2/recoveries` for at most 90 seconds until a successful `DeadMaster` recovery has a non-empty successor;
6. call `restore_lab` explicitly and set `MYSQL1_STOPPED=false`;
7. poll until `mysql1` is writable and both replicas report `mysql1` as their source;
8. require all three audit APIs to return non-empty JSON arrays;
9. compare the final MySQL IDs with the initial IDs;
10. exit non-zero on any failed contract.

- [ ] **Step 2: Validate shell syntax**

Run:

```bash
bash -n tests/functional/test-audit-ui-failover.sh
```

Expected: exit 0.

- [ ] **Step 3: Run the harness**

Run:

```bash
bash tests/functional/test-audit-ui-failover.sh
```

Expected: successful `DeadMaster` recovery, original topology restored, audit APIs populated, MySQL IDs unchanged.

- [ ] **Step 4: Capture API evidence**

Run:

```bash
curl -fsS http://localhost:3099/api/audit/0 | python3 -m json.tool >/tmp/orchestrator-audit.json
curl -fsS http://localhost:3099/api/audit-failure-detection/0 | python3 -m json.tool >/tmp/orchestrator-detections.json
curl -fsS http://localhost:3099/api/audit-recovery/0 | python3 -m json.tool >/tmp/orchestrator-recoveries.json
```

Expected: each file contains at least one record; the detection/recovery files contain `DeadMaster`.

- [ ] **Step 5: Commit**

```bash
git add tests/functional/test-audit-ui-failover.sh
git commit -m "test(ui): exercise populated audit history"
```

---

### Task 3: Browser-audit populated history states

**Files:**
- Inspect: `resources/templates/audit.tmpl`
- Inspect: `resources/templates/audit_failure_detection.tmpl`
- Inspect: `resources/templates/audit_recovery.tmpl`
- Inspect: `resources/public/js/audit.js`
- Inspect: `resources/public/js/audit-failure-detection.js`
- Inspect: `resources/public/js/audit-recovery.js`
- Modify only if a reproduced browser defect requires it.

**Interfaces:**
- Consumes: populated live APIs from Task 2.
- Produces: verified populated rows, expandable detection context, recovery detail, related-history links, and narrow-screen behavior.

- [ ] **Step 1: Audit desktop routes**

Navigate through:

```text
http://localhost:3099/web/audit
http://localhost:3099/web/audit-failure-detection
http://localhost:3099/web/audit-recovery
```

Verify visible rows, mutually exclusive empty/error states, working pager state, no console errors, and no document-level horizontal overflow.

- [ ] **Step 2: Exercise linked detail behavior**

Click the `DeadMaster` detection, its related recovery link, and the recovery's related detection link. Verify the recovery summary includes failed instance, successor, start/end times, acknowledgement state, and recovery steps.

- [ ] **Step 3: Audit at 390px**

Set the Browser viewport to 390 by 844. Repeat all three history routes and the recovery detail route. Verify tables scroll inside their shells while `document.documentElement.scrollWidth === window.innerWidth`.

- [ ] **Step 4: If a defect appears, perform one focused TDD cycle**

For each defect, add a behavior test to the closest existing file:

```text
go/http/render_test.go
go/http/static_assets_test.go
go/http/testdata/<focused>_test.js
```

Run the focused test and observe the expected failure. Implement the minimal template, JavaScript, or scoped CSS change. Rerun the focused test and the affected browser interaction before continuing. Commit each independently reviewable correction as:

```bash
git commit -m "fix(ui): <observable populated-history behavior>"
```

- [ ] **Step 5: Reset the Browser viewport**

Return the Browser viewport to its default size and leave the populated recovery detail page open for review.

---

### Task 4: Final verification and evidence report

**Files:**
- Create: `.superpowers/sdd/2026-08-12-live-failover-audit-ui/final-report.md`

**Interfaces:**
- Consumes: restored lab, generated history, and any Task 3 fixes.
- Produces: one evidence-backed handoff recording automated, browser, and safety results.

- [ ] **Step 1: Run the complete automated verification**

```bash
go test ./go/http -count=1
for file in go/http/testdata/*_test.js; do node --test "$file" || exit 1; done
for file in resources/public/js/*.js; do node --check "$file" || exit 1; done
bash tests/functional/test-smoke.sh
git diff --check
```

Expected: every command exits 0; smoke reports zero failures.

- [ ] **Step 2: Verify restored topology and container identity**

```bash
docker ps --filter 'name=functional-mysql' --format '{{.ID}} {{.Names}} {{.Status}}'
docker compose -f tests/functional/docker-compose.yml exec -T mysql1 mysql -uroot -ptestpass -Nse 'SELECT @@read_only'
docker compose -f tests/functional/docker-compose.yml exec -T mysql2 mysql -uroot -ptestpass -e 'SHOW REPLICA STATUS\G'
docker compose -f tests/functional/docker-compose.yml exec -T mysql3 mysql -uroot -ptestpass -e 'SHOW REPLICA STATUS\G'
```

Expected: all three healthy; mysql1 writable; mysql2/mysql3 source `mysql1` with IO and SQL threads running.

- [ ] **Step 3: Write the report**

Record:

- commits created;
- successful recovery analysis and successor;
- final topology and unchanged container IDs;
- audit API record counts;
- desktop and narrow browser route results;
- console errors/warnings;
- exact automated test counts;
- unresolved concerns, or explicitly `none`.

- [ ] **Step 4: Commit the report and any remaining tracked changes**

```bash
git add .superpowers/sdd/2026-08-12-live-failover-audit-ui/final-report.md
git commit -m "docs(ui): record populated audit verification"
git status --short
```

Expected: report committed and tracked worktree clean.

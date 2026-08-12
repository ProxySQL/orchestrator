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
  $COMPOSE start mysql2 mysql3 >/dev/null 2>&1 || true
  for attempt in $(seq 1 60); do
    $COMPOSE exec -T mysql1 mysqladmin ping -h localhost -uroot -ptestpass >/dev/null 2>&1 && break
    sleep 1
  done
  for replica in mysql2 mysql3; do
    for attempt in $(seq 1 60); do
      $COMPOSE exec -T "$replica" mysqladmin ping -h localhost -uroot -ptestpass >/dev/null 2>&1 && break
      sleep 1
    done
  done

  local stop_sql reset_sql change_sql start_sql
  stop_sql=$(mysql_stop_replica_sql)
  reset_sql=$(mysql_reset_replica_all_sql)
  change_sql=$(mysql_change_source_sql mysql1 3306 repl repl_pass)
  start_sql=$(mysql_start_replica_sql)

  $COMPOSE exec -T mysql1 mysql -uroot -ptestpass \
    -e "$stop_sql $reset_sql SET GLOBAL read_only=0;" >/dev/null 2>&1 || true
  # STOP REPLICA can fail when mysql1 has no replica metadata. Set the
  # writable contract independently so that failure cannot skip it.
  $COMPOSE exec -T mysql1 mysql -uroot -ptestpass \
    -e "SET GLOBAL read_only=0;" >/dev/null 2>&1 || true
  for replica in mysql2 mysql3; do
    $COMPOSE exec -T "$replica" mysql -uroot -ptestpass \
      -e "$stop_sql $change_sql $start_sql SET GLOBAL read_only=1;" >/dev/null 2>&1 || true
    # A promoted replica may no longer have replica metadata, making the
    # combined STOP/CHANGE command abort early. Retry each required state
    # transition separately so cleanup remains effective on every exit path.
    $COMPOSE exec -T "$replica" mysql -uroot -ptestpass \
      -e "$stop_sql" >/dev/null 2>&1 || true
    $COMPOSE exec -T "$replica" mysql -uroot -ptestpass \
      -e "$change_sql" >/dev/null 2>&1 || true
    $COMPOSE exec -T "$replica" mysql -uroot -ptestpass \
      -e "$start_sql SET GLOBAL read_only=1;" >/dev/null 2>&1 || true
  done
  $COMPOSE exec -T proxysql mysql -h127.0.0.1 -P6032 -uradmin -pradmin \
    -e "DELETE FROM mysql_servers WHERE hostgroup_id IN (10,20); INSERT INTO mysql_servers (hostgroup_id,hostname,port) VALUES (10,'mysql1',3306),(20,'mysql2',3306),(20,'mysql3',3306); LOAD MYSQL SERVERS TO RUNTIME; SAVE MYSQL SERVERS TO DISK;" >/dev/null 2>&1 || true
  curl -fsS --max-time 10 "$ORC_URL/api/discover/mysql1/3306" >/dev/null || true
  curl -fsS --max-time 10 "$ORC_URL/api/discover/mysql2/3306" >/dev/null || true
  curl -fsS --max-time 10 "$ORC_URL/api/discover/mysql3/3306" >/dev/null || true
}

trap 'restore_lab' EXIT

abort_contract() {
  fail "$1" "${2:-}"
  exit 1
}

record_mysql_ids() {
  local output_file="$1" service container_id
  : > "$output_file"
  for service in mysql1 mysql2 mysql3; do
    container_id=$($COMPOSE ps -q "$service" 2>/dev/null || true)
    if [ -z "$container_id" ]; then
      return 1
    fi
    printf '%s %s\n' "$service" "$container_id" >> "$output_file"
  done
}

mysql_replica_threads() {
  local replica="$1" status_sql
  if mysql_is_57 || mysql_is_mariadb; then
    status_sql="SHOW SLAVE STATUS"
  else
    status_sql="SHOW REPLICA STATUS"
  fi
  $COMPOSE exec -T "$replica" mysql -uroot -ptestpass -Nse "$status_sql" 2>/dev/null \
    | awk -F'\t' '{print $11 ":" $12; exit}'
}

require_audit_records() {
  local endpoint="$1" label="$2" required_analysis="${3:-}" body count
  body=$(curl -fsS --max-time 10 "$ORC_URL/$endpoint" 2>/dev/null) || \
    abort_contract "$label API is reachable" "$endpoint request failed"
  count=$(printf '%s' "$body" | python3 -c '
import json
import sys

required = sys.argv[1]
records = json.load(sys.stdin)
if not isinstance(records, list) or not records:
    raise SystemExit(1)
if required and not any(required in json.dumps(record) for record in records):
    raise SystemExit(2)
print(len(records))
' "$required_analysis" 2>/dev/null) || \
    abort_contract "$label API has required records" "empty, invalid JSON, or missing $required_analysis"
  pass "$label API returned $count record(s)${required_analysis:+ including $required_analysis}"
}

echo "=== AUDIT UI CONTROLLED FAILOVER ==="

record_mysql_ids "$BEFORE_IDS" || abort_contract "Record initial MySQL container IDs"
pass "Recorded initial MySQL container IDs"

wait_for_orchestrator || abort_contract "Orchestrator is ready"
discover_topology mysql1 || abort_contract "Three-node topology is discovered"

RO1=$(mysql_read_only mysql1 || true)
RO2=$(mysql_read_only mysql2 || true)
HG10=$(proxysql_servers 10 || true)
WRITERS=$(printf '%s\n' "$HG10" | awk '$3 == "ONLINE" {print $1}')

[ "$RO1" = "0" ] || abort_contract "mysql1 starts writable" "read_only=$RO1"
[ "$RO2" = "1" ] || abort_contract "mysql2 starts read-only" "read_only=$RO2"
[ "$WRITERS" = "mysql1" ] || abort_contract "ProxySQL writer is mysql1" "writers=$WRITERS"
pass "Pre-flight topology is mysql1 writer with mysql2 read-only"

LAST_RECOVERY_ID=$(curl -fsS --max-time 10 "$ORC_URL/api/v2/recoveries" 2>/dev/null \
  | python3 -c 'import json,sys; print(max([int(r.get("Id", 0)) for r in json.load(sys.stdin).get("data", [])] or [0]))' \
  2>/dev/null) || abort_contract "Read recovery baseline"

echo "Stopping mysql1 to trigger DeadMaster recovery..."
MYSQL1_STOPPED=true
$COMPOSE stop mysql1 >/dev/null || abort_contract "Stop mysql1"

RECOVERED=false
SUCCESSOR=""
echo "Waiting for a new successful DeadMaster recovery (max 90s)..."
for attempt in $(seq 1 90); do
  RECOVERY=$(curl -fsS --max-time 10 "$ORC_URL/api/v2/recoveries" 2>/dev/null \
    | python3 -c '
import json
import sys

baseline = int(sys.argv[1])
for recovery in json.load(sys.stdin).get("data", []):
    analysis = recovery.get("AnalysisEntry", {}).get("Analysis", "")
    successor = recovery.get("SuccessorKey") or {}
    recovery_id = int(recovery.get("Id", 0))
    if recovery_id > baseline and analysis == "DeadMaster" and recovery.get("IsSuccessful") and successor.get("Hostname"):
        print(successor["Hostname"])
        raise SystemExit(0)
raise SystemExit(1)
' "$LAST_RECOVERY_ID" 2>/dev/null) || RECOVERY=""
  if [ -n "$RECOVERY" ]; then
    SUCCESSOR="$RECOVERY"
    RECOVERED=true
    echo "DeadMaster recovery succeeded after ${attempt}s with successor $SUCCESSOR"
    break
  fi
  sleep 1
done

[ "$RECOVERED" = true ] || abort_contract "DeadMaster recovery completes within 90s"
pass "DeadMaster recovered successfully to $SUCCESSOR"

echo "Restoring mysql1 as writable primary and mysql2/mysql3 as replicas..."
restore_lab
if ! $COMPOSE exec -T mysql1 mysqladmin ping -h localhost -uroot -ptestpass >/dev/null 2>&1; then
  abort_contract "mysql1 restarts during explicit restoration"
fi
MYSQL1_STOPPED=false

RESTORED=false
for attempt in $(seq 1 60); do
  RO1=$(mysql_read_only mysql1 || true)
  SOURCE2=$(mysql_source_host mysql2 || true)
  SOURCE3=$(mysql_source_host mysql3 || true)
  THREADS2=$(mysql_replica_threads mysql2 || true)
  THREADS3=$(mysql_replica_threads mysql3 || true)
  if [ "$RO1" = "0" ] && [ "$SOURCE2" = "mysql1" ] && [ "$SOURCE3" = "mysql1" ] \
    && [ "$THREADS2" = "Yes:Yes" ] && [ "$THREADS3" = "Yes:Yes" ]; then
    RESTORED=true
    break
  fi
  sleep 1
done

[ "$RESTORED" = true ] || abort_contract "Restore writable mysql1 with two running replicas" \
  "mysql1 read_only=$RO1; mysql2 source=$SOURCE2 threads=$THREADS2; mysql3 source=$SOURCE3 threads=$THREADS3"
pass "Restored mysql1 writer and two running replicas"

HG10=$(proxysql_servers 10 || true)
WRITERS=$(printf '%s\n' "$HG10" | awk '$3 == "ONLINE" {print $1}')
[ "$WRITERS" = "mysql1" ] || abort_contract "Restore ProxySQL writer mysql1" "writers=$WRITERS"
pass "Restored ProxySQL writer mysql1"

require_audit_records "api/audit/0" "General audit"
require_audit_records "api/audit-failure-detection/0" "Failure detection audit" "DeadMaster"
require_audit_records "api/audit-recovery/0" "Recovery audit" "DeadMaster"

record_mysql_ids "$AFTER_IDS" || abort_contract "Record final MySQL container IDs"
if ! cmp -s "$BEFORE_IDS" "$AFTER_IDS"; then
  abort_contract "MySQL container IDs are unchanged" \
    "before=$(tr '\n' ';' < "$BEFORE_IDS") after=$(tr '\n' ';' < "$AFTER_IDS")"
fi
pass "MySQL container IDs are unchanged"

rm -f "$BEFORE_IDS" "$AFTER_IDS"
summary

#!/bin/bash
# Issue #106: MariaDB / SQL-stopped relay drain and false DeadMaster avoidance.
# Works on MariaDB and MySQL topologies (SQL_THREAD stop + unapplied relay + failover).
set -uo pipefail
cd "$(dirname "$0")/../.."
source tests/functional/lib.sh

COMPOSE="docker compose -f tests/functional/docker-compose.yml"
if [ -f tests/functional/docker-compose.mariadb.yml ] && mysql_is_mariadb 2>/dev/null; then
    COMPOSE="docker compose -f tests/functional/docker-compose.yml -f tests/functional/docker-compose.mariadb.yml"
fi

echo "=== MARIADB / RELAY DRAIN FAILOVER TESTS (issue #106) ==="
echo "Version: $(mysql_full_version 2>/dev/null || echo unknown)"

wait_for_orchestrator || { echo "FATAL: Orchestrator not reachable"; exit 1; }
discover_topology "mysql1"

STOP_SQL_THREAD=$(mysql_stop_sql_thread_sql)
START_SQL=$(mysql_start_replica_sql)

# ----------------------------------------------------------------
echo ""
echo "--- Test 1: SQL stopped + IO running is not DeadMaster while master is up ---"

$COMPOSE exec -T mysql2 mysql -uroot -ptestpass -e "$STOP_SQL_THREAD" 2>/dev/null
curl -s --max-time 10 "$ORC_URL/api/discover/mysql2/3306" > /dev/null 2>&1
sleep 3

ANALYSIS=$(curl -s --max-time 10 "$ORC_URL/api/replication-analysis" 2>/dev/null || echo "[]")
if echo "$ANALYSIS" | python3 -c "
import json, sys
data = json.load(sys.stdin)
if not isinstance(data, list):
    data = data.get('Details', data.get('Code') and [] or [])
    if not isinstance(data, list):
        sys.exit(0)
for a in data:
    code = a.get('AnalysisCode') or a.get('Analysis') or ''
    if code == 'DeadMaster':
        sys.exit(1)
sys.exit(0)
" 2>/dev/null; then
    pass "No DeadMaster while master is reachable (SQL stopped on replica)"
else
    fail "Unexpected DeadMaster while master is still up"
fi

# Restore SQL thread for next phase
$COMPOSE exec -T mysql2 mysql -uroot -ptestpass -e "$START_SQL" 2>/dev/null
sleep 2

# ----------------------------------------------------------------
echo ""
echo "--- Test 2: Unapplied relay events survive failover (drain before promote) ---"

$COMPOSE exec -T mysql1 mysql -uroot -ptestpass -e "
CREATE DATABASE IF NOT EXISTS orch_relay_test;
CREATE TABLE IF NOT EXISTS orch_relay_test.t (id INT PRIMARY KEY, v VARCHAR(64));
INSERT INTO orch_relay_test.t (id, v) VALUES (1, 'before_stop') ON DUPLICATE KEY UPDATE v=VALUES(v);
" 2>/dev/null

# Wait for both replicas to apply
for r in mysql2 mysql3; do
    for i in $(seq 1 30); do
        COUNT=$($COMPOSE exec -T "$r" mysql -uroot -ptestpass -Nse "SELECT COUNT(*) FROM orch_relay_test.t WHERE id=1" 2>/dev/null | tr -d '[:space:]')
        [ "$COUNT" = "1" ] && break
        sleep 1
    done
done

# Stop SQL on both replicas so subsequent master writes sit in relay logs only
$COMPOSE exec -T mysql2 mysql -uroot -ptestpass -e "$STOP_SQL_THREAD" 2>/dev/null
$COMPOSE exec -T mysql3 mysql -uroot -ptestpass -e "$STOP_SQL_THREAD" 2>/dev/null
sleep 1

# Committed on master; IO should receive into relay while SQL is stopped
$COMPOSE exec -T mysql1 mysql -uroot -ptestpass -e "
INSERT INTO orch_relay_test.t (id, v) VALUES (2, 'in_relay_only') ON DUPLICATE KEY UPDATE v=VALUES(v);
" 2>/dev/null
sleep 2

# Confirm replicas have not applied id=2 yet
for r in mysql2 mysql3; do
    COUNT=$($COMPOSE exec -T "$r" mysql -uroot -ptestpass -Nse "SELECT COUNT(*) FROM orch_relay_test.t WHERE id=2" 2>/dev/null | tr -d '[:space:]')
    if [ "$COUNT" = "0" ]; then
        pass "$r has not applied id=2 (sitting in relay)"
    else
        fail "$r already applied id=2 before failover (COUNT=$COUNT)"
    fi
done

echo "Stopping mysql1 (master) to force failover..."
$COMPOSE stop mysql1

echo "Waiting for recovery (max 90s)..."
RECOVERED=false
SUCCESSOR=""
for i in $(seq 1 90); do
    RECOVERIES=$(curl -s --max-time 10 "$ORC_URL/api/v2/recoveries" 2>/dev/null || echo "{}")
    SUCCESSOR=$(echo "$RECOVERIES" | python3 -c "
import json, sys
d = json.load(sys.stdin)
details = d.get('Details') or d.get('details') or []
if isinstance(d, list):
    details = d
for rec in details:
    a = rec.get('AnalysisEntry', {}).get('AnalysisCode') or rec.get('Analysis') or ''
    s = rec.get('IsSuccessful') or rec.get('isSuccessful')
    succ = rec.get('SuccessorKey') or rec.get('successorKey') or {}
    host = succ.get('Hostname') or succ.get('hostname') or ''
    if a in ('DeadMaster', 'DeadMasterAndSomeReplicas', 'DeadMasterAndReplicas') and s and host:
        print(host)
        sys.exit(0)
sys.exit(1)
" 2>/dev/null || true)
    if [ -n "$SUCCESSOR" ]; then
        RECOVERED=true
        echo "Recovery succeeded after ${i}s; successor=$SUCCESSOR"
        break
    fi
    sleep 1
done

if [ "$RECOVERED" = "true" ]; then
    pass "Failover recovered with successor $SUCCESSOR"
    # Map hostname to container
    NEW_MASTER="mysql2"
    if echo "$SUCCESSOR" | grep -q mysql3; then
        NEW_MASTER="mysql3"
    fi
    sleep 3
    COUNT=$($COMPOSE exec -T "$NEW_MASTER" mysql -uroot -ptestpass -Nse "SELECT COUNT(*) FROM orch_relay_test.t WHERE id=2" 2>/dev/null | tr -d '[:space:]')
    if [ "$COUNT" = "1" ]; then
        pass "Promoted master $NEW_MASTER applied relay event id=2 (no data loss)"
    else
        fail "Promoted master $NEW_MASTER missing id=2 (COUNT=$COUNT) — data loss"
    fi
else
    # Accept clean failure (no undrainable promote) but not silent data loss promote
    ANALYSIS=$(curl -s --max-time 10 "$ORC_URL/api/replication-analysis" 2>/dev/null || echo "[]")
    fail "No successful recovery within 90s (may be environment); analysis=$(echo "$ANALYSIS" | head -c 200)"
fi

summary

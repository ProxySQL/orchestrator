#!/bin/bash
# Issue #106: MariaDB / SQL-stopped relay drain and false DeadMaster avoidance.
# Works on MariaDB and MySQL topologies (SQL_THREAD stop + unapplied relay + failover).
set -uo pipefail
cd "$(dirname "$0")/../.."
source tests/functional/lib.sh

COMPOSE="docker compose -f tests/functional/docker-compose.yml"
if mysql_is_mariadb 2>/dev/null; then
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
if isinstance(data, dict):
    data = data.get('Details') or data.get('data') or []
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

# Wait for both replicas to apply id=1 (fail hard if not)
for r in mysql2 mysql3; do
    APPLIED=false
    for i in $(seq 1 45); do
        COUNT=$($COMPOSE exec -T "$r" mysql -uroot -ptestpass -Nse "SELECT COUNT(*) FROM orch_relay_test.t WHERE id=1" 2>/dev/null | tr -d '[:space:]')
        if [ "$COUNT" = "1" ]; then
            APPLIED=true
            break
        fi
        sleep 1
    done
    if [ "$APPLIED" = "true" ]; then
        pass "$r applied id=1"
    else
        fail "$r never applied id=1 (COUNT=${COUNT:-empty})"
        summary
    fi
done

# Stop SQL on both replicas so subsequent master writes sit in relay logs only
$COMPOSE exec -T mysql2 mysql -uroot -ptestpass -e "$STOP_SQL_THREAD" 2>/dev/null
$COMPOSE exec -T mysql3 mysql -uroot -ptestpass -e "$STOP_SQL_THREAD" 2>/dev/null
sleep 1

# Committed on master; wait until IO has received into relay while SQL stays stopped
$COMPOSE exec -T mysql1 mysql -uroot -ptestpass -e "
INSERT INTO orch_relay_test.t (id, v) VALUES (2, 'in_relay_only') ON DUPLICATE KEY UPDATE v=VALUES(v);
" 2>/dev/null

# Poll until both replicas have id=2 still unapplied (SQL stopped) but IO has advanced:
# Seconds_Behind_Master becomes NULL when SQL is stopped; check Exec vs Read positions differ
# or that a temporary START SQL would apply — we only assert table still missing id=2 and IO=Yes.
for r in mysql2 mysql3; do
    READY=false
    for i in $(seq 1 30); do
        COUNT=$($COMPOSE exec -T "$r" mysql -uroot -ptestpass -Nse "SELECT COUNT(*) FROM orch_relay_test.t WHERE id=2" 2>/dev/null | tr -d '[:space:]')
        if mysql_is_mariadb || mysql_is_57; then
            IO=$($COMPOSE exec -T "$r" mysql -uroot -ptestpass -e "SHOW SLAVE STATUS\G" 2>/dev/null | awk '/Slave_IO_Running:/{print $2; exit}')
            SQL=$($COMPOSE exec -T "$r" mysql -uroot -ptestpass -e "SHOW SLAVE STATUS\G" 2>/dev/null | awk '/Slave_SQL_Running:/{print $2; exit}')
            READ_POS=$($COMPOSE exec -T "$r" mysql -uroot -ptestpass -e "SHOW SLAVE STATUS\G" 2>/dev/null | awk '/Read_Master_Log_Pos:/{print $2; exit}')
            EXEC_POS=$($COMPOSE exec -T "$r" mysql -uroot -ptestpass -e "SHOW SLAVE STATUS\G" 2>/dev/null | awk '/Exec_Master_Log_Pos:/{print $2; exit}')
        else
            IO=$($COMPOSE exec -T "$r" mysql -uroot -ptestpass -e "SHOW REPLICA STATUS\G" 2>/dev/null | awk '/Replica_IO_Running:/{print $2; exit}')
            SQL=$($COMPOSE exec -T "$r" mysql -uroot -ptestpass -e "SHOW REPLICA STATUS\G" 2>/dev/null | awk '/Replica_SQL_Running:/{print $2; exit}')
            READ_POS=$($COMPOSE exec -T "$r" mysql -uroot -ptestpass -e "SHOW REPLICA STATUS\G" 2>/dev/null | awk '/Read_Source_Log_Pos:/{print $2; exit}')
            EXEC_POS=$($COMPOSE exec -T "$r" mysql -uroot -ptestpass -e "SHOW REPLICA STATUS\G" 2>/dev/null | awk '/Exec_Source_Log_Pos:/{print $2; exit}')
        fi
        if [ "$COUNT" = "0" ] && [ "$IO" = "Yes" ] && [ "$SQL" = "No" ] && [ -n "$READ_POS" ] && [ -n "$EXEC_POS" ] && [ "$READ_POS" != "$EXEC_POS" ]; then
            READY=true
            echo "$r: id=2 unapplied, IO=$IO SQL=$SQL Read=$READ_POS Exec=$EXEC_POS"
            break
        fi
        sleep 1
    done
    if [ "$READY" = "true" ]; then
        pass "$r has id=2 only in relay (SQL stopped, Read!=Exec)"
    else
        fail "$r not ready for failover drain test (COUNT=${COUNT:-?} IO=${IO:-?} SQL=${SQL:-?} Read=${READ_POS:-?} Exec=${EXEC_POS:-?})"
        summary
    fi
done

echo "Stopping mysql1 (master) to force failover..."
$COMPOSE stop mysql1

echo "Waiting for recovery (max 90s)..."
RECOVERED=false
SUCCESSOR=""
for i in $(seq 1 90); do
    RECOVERIES=$(curl -s --max-time 10 "$ORC_URL/api/v2/recoveries" 2>/dev/null || echo "{}")
    HAS_RECOVERY=$(echo "$RECOVERIES" | python3 -c "
import json, sys
d = json.load(sys.stdin)
data = d.get('data', [])
if isinstance(d, list):
    data = d
for r in data:
    a = r.get('AnalysisEntry', {}).get('Analysis', '') or r.get('AnalysisEntry', {}).get('AnalysisCode', '')
    s = r.get('IsSuccessful', False)
    successor = r.get('SuccessorKey', {}).get('Hostname', '')
    if a in ('DeadMaster', 'DeadMasterAndSomeReplicas', 'DeadMasterAndReplicas') and s and successor:
        print(successor)
        sys.exit(0)
sys.exit(1)
" 2>/dev/null || true)
    if [ -n "$HAS_RECOVERY" ]; then
        SUCCESSOR="$HAS_RECOVERY"
        RECOVERED=true
        echo "Recovery detected after ${i}s; successor=$SUCCESSOR"
        break
    fi
    sleep 1
done

if [ "$RECOVERED" = "true" ]; then
    pass "Failover recovered with successor $SUCCESSOR"
    NEW_MASTER="mysql2"
    if echo "$SUCCESSOR" | grep -q mysql3; then
        NEW_MASTER="mysql3"
    fi
    sleep 3
    COUNT=$($COMPOSE exec -T "$NEW_MASTER" mysql -uroot -ptestpass -Nse "SELECT COUNT(*) FROM orch_relay_test.t WHERE id=2" 2>/dev/null | tr -d '[:space:]')
    if [ "$COUNT" = "1" ]; then
        pass "Promoted master $NEW_MASTER applied relay event id=2 (no data loss)"
    else
        fail "Promoted master $NEW_MASTER missing id=2 (COUNT=${COUNT:-empty}) — data loss"
    fi
else
    fail "No successful recovery within 90s"
fi

summary

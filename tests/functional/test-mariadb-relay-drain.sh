#!/bin/bash
# Issue #106: MariaDB / SQL-stopped relay drain and false DeadMaster avoidance.
# Works on MariaDB and MySQL topologies (SQL_THREAD stop + unapplied relay + failover).
set -uo pipefail
cd "$(dirname "$0")/../.."
source tests/functional/lib.sh

# Prefer MYSQL_IMAGE (set by CI) so we select the MariaDB compose overlay before
# probing VERSION(); a plain compose exec can fail and mis-detect as MySQL.
COMPOSE="docker compose -f tests/functional/docker-compose.yml"
if echo "${MYSQL_IMAGE:-}" | grep -qi mariadb; then
    COMPOSE="docker compose -f tests/functional/docker-compose.yml -f tests/functional/docker-compose.mariadb.yml"
fi
export COMPOSE
# Clear cached version probes so they use the correct COMPOSE.
MYSQL_MAJOR_VERSION=""
MYSQL_FULL_VERSION_CACHE=""
if mysql_is_mariadb 2>/dev/null; then
    COMPOSE="docker compose -f tests/functional/docker-compose.yml -f tests/functional/docker-compose.mariadb.yml"
    export COMPOSE
fi

echo "=== MARIADB / RELAY DRAIN FAILOVER TESTS (issue #106) ==="
echo "Version: $(mysql_full_version 2>/dev/null || echo unknown)"
echo "COMPOSE: $COMPOSE"

wait_for_orchestrator || { echo "FATAL: Orchestrator not reachable"; exit 1; }

STOP_SQL_THREAD=$(mysql_stop_sql_thread_sql)
START_SQL_THREAD=$(mysql_start_sql_thread_sql)
START_REPLICA=$(mysql_start_replica_sql)
STOP_REPLICA=$(mysql_stop_replica_sql)
RESET_REPLICA=$(mysql_reset_replica_all_sql)
CHANGE_TO_MYSQL1=$(mysql_change_source_sql mysql1 3306 repl repl_pass)
echo "Using SQL dialect: stop_sql='$STOP_SQL_THREAD' start_replica='$START_REPLICA'"

# Parse one SHOW SLAVE/REPLICA STATUS\G blob. Prints: IO|SQL|READ|EXEC
replica_status_fields() {
    local container="$1"
    local blob io sql readpos execpos
    if mysql_is_mariadb || mysql_is_57; then
        blob=$($COMPOSE exec -T "$container" mysql -uroot -ptestpass -e "SHOW SLAVE STATUS\G" 2>/dev/null || true)
        io=$(echo "$blob" | awk -F': *' '/Slave_IO_Running:/{print $2; exit}' | tr -d '[:space:]')
        sql=$(echo "$blob" | awk -F': *' '/Slave_SQL_Running:/{print $2; exit}' | tr -d '[:space:]')
        readpos=$(echo "$blob" | awk -F': *' '/Read_Master_Log_Pos:/{print $2; exit}' | tr -d '[:space:]')
        execpos=$(echo "$blob" | awk -F': *' '/Exec_Master_Log_Pos:/{print $2; exit}' | tr -d '[:space:]')
    else
        blob=$($COMPOSE exec -T "$container" mysql -uroot -ptestpass -e "SHOW REPLICA STATUS\G" 2>/dev/null || true)
        io=$(echo "$blob" | awk -F': *' '/Replica_IO_Running:/{print $2; exit}' | tr -d '[:space:]')
        sql=$(echo "$blob" | awk -F': *' '/Replica_SQL_Running:/{print $2; exit}' | tr -d '[:space:]')
        readpos=$(echo "$blob" | awk -F': *' '/Read_Source_Log_Pos:/{print $2; exit}' | tr -d '[:space:]')
        execpos=$(echo "$blob" | awk -F': *' '/Exec_Source_Log_Pos:/{print $2; exit}' | tr -d '[:space:]')
        # Fall back to legacy names if present
        [ -z "$io" ] && io=$(echo "$blob" | awk -F': *' '/Slave_IO_Running:/{print $2; exit}' | tr -d '[:space:]')
        [ -z "$sql" ] && sql=$(echo "$blob" | awk -F': *' '/Slave_SQL_Running:/{print $2; exit}' | tr -d '[:space:]')
        [ -z "$readpos" ] && readpos=$(echo "$blob" | awk -F': *' '/Read_Master_Log_Pos:/{print $2; exit}' | tr -d '[:space:]')
        [ -z "$execpos" ] && execpos=$(echo "$blob" | awk -F': *' '/Exec_Master_Log_Pos:/{print $2; exit}' | tr -d '[:space:]')
    fi
    echo "${io}|${sql}|${readpos}|${execpos}"
}

replica_threads_ok() {
    local r="$1"
    local fields io sql
    fields=$(replica_status_fields "$r")
    io=$(echo "$fields" | cut -d'|' -f1)
    sql=$(echo "$fields" | cut -d'|' -f2)
    [ "$io" = "Yes" ] && [ "$sql" = "Yes" ]
}

# Soft-start SQL/IO if already pointed at mysql1.
nudge_replica_threads() {
    local r="$1"
    $COMPOSE exec -T "$r" mysql -uroot -ptestpass -e "$START_SQL_THREAD" 2>/dev/null || true
    $COMPOSE exec -T "$r" mysql -uroot -ptestpass -e "$START_REPLICA" 2>/dev/null || true
}

# Full reconfigure: point r at mysql1 with GTID auto-position / MariaDB slave_pos.
repoint_replica_to_mysql1() {
    local r="$1"
    $COMPOSE exec -T "$r" mysql -uroot -ptestpass -e "STOP REPLICA; STOP SLAVE;" 2>/dev/null || true
    # RESET drops errant/local GTID history from prior failover tests.
    # MariaDB errno 1950: out-of-order GTID when gtid_slave_pos / strict mode conflict.
    $COMPOSE exec -T "$r" mysql -uroot -ptestpass -e "$RESET_REPLICA" 2>/dev/null || true
    if mysql_is_mariadb; then
        $COMPOSE exec -T "$r" mysql -uroot -ptestpass -e "SET GLOBAL gtid_slave_pos=''; SET GLOBAL gtid_strict_mode=OFF;" 2>/dev/null || true
    fi
    $COMPOSE exec -T "$r" mysql -uroot -ptestpass -e "
        $CHANGE_TO_MYSQL1
        SET GLOBAL read_only=1;
        $START_REPLICA
    " 2>/dev/null || true
    nudge_replica_threads "$r"
}

# Ensure mysql1 is master and mysql2/mysql3 are healthy direct replicas.
# Required after earlier failover suites that promote mysql2 and stop mysql1.
ensure_flat_topology() {
    echo "Ensuring flat topology (mysql1 master, mysql2/mysql3 replicas)..."
    $COMPOSE start mysql1 mysql2 mysql3 >/dev/null 2>&1 || true
    sleep 2

    for HOST in mysql1 mysql2 mysql3; do
        for i in $(seq 1 30); do
            if $COMPOSE exec -T "$HOST" mysqladmin -uroot -ptestpass ping --silent 2>/dev/null; then
                break
            fi
            sleep 1
        done
    done

    $COMPOSE exec -T mysql1 mysql -uroot -ptestpass -e "
        SET GLOBAL read_only=0;
        SET GLOBAL super_read_only=0;
    " 2>/dev/null || true

    # Fast path: already healthy — common on MariaDB job (only smoke ran before us).
    if replica_threads_ok mysql2 && replica_threads_ok mysql3; then
        RO1=$($COMPOSE exec -T mysql1 mysql -uroot -ptestpass -Nse "SELECT @@read_only" 2>/dev/null | tr -d '[:space:]')
        if [ "$RO1" = "0" ]; then
            pass "Flat topology already healthy"
            return 0
        fi
    fi

    for r in mysql2 mysql3; do
        repoint_replica_to_mysql1 "$r"
    done

    # ProxySQL writer back to mysql1 (best-effort)
    docker compose -f tests/functional/docker-compose.yml exec -T proxysql \
        mysql -h127.0.0.1 -P6032 -uradmin -pradmin -e \
        "DELETE FROM mysql_servers WHERE hostgroup_id IN (10,20); INSERT INTO mysql_servers (hostgroup_id,hostname,port) VALUES (10,'mysql1',3306),(20,'mysql2',3306),(20,'mysql3',3306); LOAD MYSQL SERVERS TO RUNTIME; SAVE MYSQL SERVERS TO DISK;" \
        2>/dev/null || true

    sleep 2
    curl -s --max-time 10 "$ORC_URL/api/discover/mysql1/3306" >/dev/null 2>&1
    curl -s --max-time 10 "$ORC_URL/api/discover/mysql2/3306" >/dev/null 2>&1
    curl -s --max-time 10 "$ORC_URL/api/discover/mysql3/3306" >/dev/null 2>&1

    for r in mysql2 mysql3; do
        OK=false
        for i in $(seq 1 60); do
            if replica_threads_ok "$r"; then
                OK=true
                break
            fi
            fields=$(replica_status_fields "$r")
            IO=$(echo "$fields" | cut -d'|' -f1)
            SQL=$(echo "$fields" | cut -d'|' -f2)
            # IO up but SQL down: start SQL thread (common after STOP SQL_THREAD / partial start)
            if [ "$IO" = "Yes" ] && [ "$SQL" != "Yes" ]; then
                nudge_replica_threads "$r"
            elif [ -z "$IO" ] || [ "$IO" = "No" ]; then
                # Empty status or not configured — full repoint
                if [ "$((i % 15))" = "0" ]; then
                    repoint_replica_to_mysql1 "$r"
                else
                    nudge_replica_threads "$r"
                fi
            else
                nudge_replica_threads "$r"
            fi
            sleep 1
        done
        if [ "$OK" != "true" ]; then
            fail "Could not restore $r as replicating replica of mysql1 (status=$(replica_status_fields "$r"))"
            $COMPOSE exec -T "$r" mysql -uroot -ptestpass -e "SHOW SLAVE STATUS\G; SHOW REPLICA STATUS\G;" 2>/dev/null | head -60 || true
            summary
        fi
    done
    pass "Flat topology ready (mysql1 master, replicas IO/SQL=Yes)"
}

ensure_flat_topology

# ----------------------------------------------------------------
echo ""
echo "--- Test 1: SQL stopped + IO running is not DeadMaster while master is up ---"

$COMPOSE exec -T mysql2 mysql -uroot -ptestpass -e "$STOP_SQL_THREAD" 2>/dev/null
curl -s --max-time 10 "$ORC_URL/api/discover/mysql2/3306" >/dev/null 2>&1
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

# Restore SQL thread only (IO was still running)
$COMPOSE exec -T mysql2 mysql -uroot -ptestpass -e "$START_SQL_THREAD" 2>/dev/null || \
    $COMPOSE exec -T mysql2 mysql -uroot -ptestpass -e "$START_REPLICA" 2>/dev/null || true
sleep 2
for i in $(seq 1 30); do
    fields=$(replica_status_fields mysql2)
    IO=$(echo "$fields" | cut -d'|' -f1)
    SQL=$(echo "$fields" | cut -d'|' -f2)
    if [ "$IO" = "Yes" ] && [ "$SQL" = "Yes" ]; then
        break
    fi
    sleep 1
done

# ----------------------------------------------------------------
echo ""
echo "--- Test 2: Unapplied relay events survive failover (drain before promote) ---"

$COMPOSE exec -T mysql1 mysql -uroot -ptestpass -e "
CREATE DATABASE IF NOT EXISTS orch_relay_test;
CREATE TABLE IF NOT EXISTS orch_relay_test.t (id INT PRIMARY KEY, v VARCHAR(64));
INSERT INTO orch_relay_test.t (id, v) VALUES (1, 'before_stop')
  ON DUPLICATE KEY UPDATE v='before_stop';
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
        # If replica not applying, try restarting SQL thread
        if [ "$((i % 10))" = "0" ]; then
            $COMPOSE exec -T "$r" mysql -uroot -ptestpass -e "$START_SQL_THREAD" 2>/dev/null || \
                $COMPOSE exec -T "$r" mysql -uroot -ptestpass -e "$START_REPLICA" 2>/dev/null || true
        fi
        sleep 1
    done
    if [ "$APPLIED" = "true" ]; then
        pass "$r applied id=1"
    else
        fail "$r never applied id=1 (COUNT=${COUNT:-empty} status=$(replica_status_fields "$r"))"
        summary
    fi
done

# Stop SQL on both replicas so subsequent master writes sit in relay logs only
$COMPOSE exec -T mysql2 mysql -uroot -ptestpass -e "$STOP_SQL_THREAD" 2>/dev/null
$COMPOSE exec -T mysql3 mysql -uroot -ptestpass -e "$STOP_SQL_THREAD" 2>/dev/null
sleep 1

# Committed on master; wait until IO has received into relay while SQL stays stopped
$COMPOSE exec -T mysql1 mysql -uroot -ptestpass -e "
INSERT INTO orch_relay_test.t (id, v) VALUES (2, 'in_relay_only')
  ON DUPLICATE KEY UPDATE v='in_relay_only';
" 2>/dev/null

for r in mysql2 mysql3; do
    READY=false
    for i in $(seq 1 45); do
        COUNT=$($COMPOSE exec -T "$r" mysql -uroot -ptestpass -Nse "SELECT COUNT(*) FROM orch_relay_test.t WHERE id=2" 2>/dev/null | tr -d '[:space:]')
        fields=$(replica_status_fields "$r")
        IO=$(echo "$fields" | cut -d'|' -f1)
        SQL=$(echo "$fields" | cut -d'|' -f2)
        READ_POS=$(echo "$fields" | cut -d'|' -f3)
        EXEC_POS=$(echo "$fields" | cut -d'|' -f4)
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
        fail "$r not ready for failover drain test (COUNT=${COUNT:-?} status=$(replica_status_fields "$r"))"
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

# Best-effort restore so a mid-suite run does not poison later tests.
echo "--- Cleanup: restart mysql1 (best-effort) ---"
$COMPOSE start mysql1 >/dev/null 2>&1 || true

summary

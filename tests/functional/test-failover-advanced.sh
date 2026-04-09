#!/bin/bash
# Tier B+: Advanced failover tests — intermediate master, errant GTID, co-master
set -uo pipefail  # no -e: we handle failures ourselves
cd "$(dirname "$0")/../.."
source tests/functional/lib.sh

COMPOSE="docker compose -f tests/functional/docker-compose.yml"

echo "=== TIER B+: ADVANCED FAILOVER TESTS ==="

# ----------------------------------------------------------------
echo ""
echo "--- Restore clean topology ---"

# Ensure mysql1 is up
$COMPOSE start mysql1 mysql2 mysql3 2>/dev/null || true
sleep 5

# Wait for all MySQL containers to be reachable
for HOST in mysql1 mysql2 mysql3; do
    for i in $(seq 1 30); do
        if $COMPOSE exec -T "$HOST" mysql -uroot -ptestpass -e "SELECT 1" >/dev/null 2>&1; then
            break
        fi
        sleep 1
    done
done

# Restore flat topology: mysql1 is master, mysql2/mysql3 are direct replicas
STOP_SQL=$(mysql_stop_replica_sql)
RESET_SQL=$(mysql_reset_replica_all_sql)
START_SQL=$(mysql_start_replica_sql)
CHANGE_TO_MYSQL1=$(mysql_change_source_sql mysql1 3306 repl repl_pass)

$COMPOSE exec -T mysql1 \
    mysql -uroot -ptestpass -e "$STOP_SQL $RESET_SQL SET GLOBAL read_only=0; SET GLOBAL super_read_only=0;" 2>/dev/null
$COMPOSE exec -T mysql2 \
    mysql -uroot -ptestpass -e "$STOP_SQL $RESET_SQL $CHANGE_TO_MYSQL1 $START_SQL SET GLOBAL read_only=1; SET GLOBAL super_read_only=1;" 2>/dev/null
$COMPOSE exec -T mysql3 \
    mysql -uroot -ptestpass -e "$STOP_SQL $RESET_SQL $CHANGE_TO_MYSQL1 $START_SQL SET GLOBAL read_only=1; SET GLOBAL super_read_only=1;" 2>/dev/null

# Reset ProxySQL
$COMPOSE exec -T proxysql \
    mysql -h127.0.0.1 -P6032 -uradmin -pradmin -e \
    "DELETE FROM mysql_servers WHERE hostgroup_id IN (10,20); INSERT INTO mysql_servers (hostgroup_id,hostname,port) VALUES (10,'mysql1',3306),(20,'mysql2',3306),(20,'mysql3',3306); LOAD MYSQL SERVERS TO RUNTIME; SAVE MYSQL SERVERS TO DISK;" 2>/dev/null

sleep 5

wait_for_orchestrator || { echo "FATAL: Orchestrator not reachable"; exit 1; }

# Re-seed discovery
curl -s --max-time 10 "$ORC_URL/api/discover/mysql1/3306" > /dev/null
curl -s --max-time 10 "$ORC_URL/api/discover/mysql2/3306" > /dev/null
curl -s --max-time 10 "$ORC_URL/api/discover/mysql3/3306" > /dev/null
sleep 10

discover_topology "mysql1"
pass "Clean topology restored (mysql1 master, mysql2/mysql3 replicas)"

# ================================================================
echo ""
echo "--- Test 1: Intermediate Master Failure (Chain Topology) ---"

# Setup chain: mysql1 -> mysql2 -> mysql3
CHANGE_TO_MYSQL2=$(mysql_change_source_sql mysql2 3306 repl repl_pass)
$COMPOSE exec -T mysql3 \
    mysql -uroot -ptestpass -e "$STOP_SQL $CHANGE_TO_MYSQL2 $START_SQL" 2>/dev/null

sleep 5

# Re-seed discovery so orchestrator picks up the new topology
curl -s --max-time 10 "$ORC_URL/api/discover/mysql1/3306" > /dev/null
curl -s --max-time 10 "$ORC_URL/api/discover/mysql2/3306" > /dev/null
curl -s --max-time 10 "$ORC_URL/api/discover/mysql3/3306" > /dev/null
sleep 15

# Verify chain topology
SOURCE3=$(mysql_source_host mysql3)
SOURCE2=$(mysql_source_host mysql2)
if [ "$SOURCE3" = "mysql2" ]; then
    pass "Chain topology: mysql3 replicates from mysql2"
else
    fail "Chain topology: mysql3 source=$SOURCE3 (expected mysql2)"
fi
if [ "$SOURCE2" = "mysql1" ]; then
    pass "Chain topology: mysql2 replicates from mysql1"
else
    fail "Chain topology: mysql2 source=$SOURCE2 (expected mysql1)"
fi

# Kill intermediate master (mysql2)
echo "Stopping mysql2 (intermediate master)..."
$COMPOSE stop mysql2

echo "Waiting for orchestrator to detect DeadIntermediateMaster and recover (max 90s)..."
RECOVERED=false
for i in $(seq 1 90); do
    RECOVERIES=$(curl -s --max-time 10 "$ORC_URL/api/v2/recoveries" 2>/dev/null)
    HAS_RECOVERY=$(echo "$RECOVERIES" | python3 -c "
import json, sys
d = json.load(sys.stdin)
data = d.get('data', [])
for r in data:
    a = r.get('AnalysisEntry', {}).get('Analysis', '')
    s = r.get('IsSuccessful', False)
    if 'DeadIntermediateMaster' in a and s:
        print('RECOVERED')
        sys.exit(0)
print('WAITING')
" 2>/dev/null)
    if [ "$HAS_RECOVERY" = "RECOVERED" ]; then
        echo "DeadIntermediateMaster recovery detected after ${i}s"
        RECOVERED=true
        break
    fi
    sleep 1
done

if [ "$RECOVERED" = "true" ]; then
    pass "DeadIntermediateMaster detected and recovered"
else
    fail "DeadIntermediateMaster: no recovery detected within 90s"
    echo "  DEBUG: Recent recoveries:"
    curl -s --max-time 10 "$ORC_URL/api/v2/recoveries" 2>/dev/null | python3 -m json.tool 2>/dev/null | head -30
fi

# Verify mysql3 now replicates from mysql1
sleep 5
SOURCE3=$(mysql_source_host mysql3)
if [ "$SOURCE3" = "mysql1" ]; then
    pass "mysql3 relocated under mysql1 after intermediate master failure"
else
    fail "mysql3 source=$SOURCE3 after recovery (expected mysql1)"
fi

# Cleanup: restart mysql2, restore flat topology
echo "Restarting mysql2..."
$COMPOSE start mysql2
sleep 10

# Wait for mysql2 to be reachable
for i in $(seq 1 30); do
    if $COMPOSE exec -T mysql2 mysql -uroot -ptestpass -e "SELECT 1" >/dev/null 2>&1; then
        break
    fi
    sleep 1
done

$COMPOSE exec -T mysql2 \
    mysql -uroot -ptestpass -e "$STOP_SQL $RESET_SQL $CHANGE_TO_MYSQL1 $START_SQL SET GLOBAL read_only=1; SET GLOBAL super_read_only=1;" 2>/dev/null
$COMPOSE exec -T mysql3 \
    mysql -uroot -ptestpass -e "$STOP_SQL $CHANGE_TO_MYSQL1 $START_SQL" 2>/dev/null

sleep 5
curl -s --max-time 10 "$ORC_URL/api/discover/mysql1/3306" > /dev/null
curl -s --max-time 10 "$ORC_URL/api/discover/mysql2/3306" > /dev/null
curl -s --max-time 10 "$ORC_URL/api/discover/mysql3/3306" > /dev/null
sleep 10

pass "Test 1 cleanup: flat topology restored"

# ================================================================
echo ""
echo "--- Test 2: Errant GTID Detection ---"

# Inject an errant transaction on mysql2
$COMPOSE exec -T mysql2 \
    mysql -uroot -ptestpass -e "
SET GLOBAL read_only=0;
SET GLOBAL super_read_only=0;
CREATE DATABASE IF NOT EXISTS errant_test;
SET GLOBAL read_only=1;
SET GLOBAL super_read_only=1;
" 2>/dev/null

echo "Waiting for orchestrator to detect errant GTID (max 60s)..."
ERRANT_DETECTED=false
for i in $(seq 1 60); do
    # Force a refresh
    curl -s --max-time 10 "$ORC_URL/api/discover/mysql2/3306" > /dev/null 2>&1
    sleep 2
    GTID_ERRANT=$(curl -s --max-time 10 "$ORC_URL/api/instance/mysql2/3306" 2>/dev/null | python3 -c "
import json, sys
d = json.load(sys.stdin)
errant = d.get('GtidErrant', '')
print(errant)
" 2>/dev/null || echo "")
    if [ -n "$GTID_ERRANT" ] && [ "$GTID_ERRANT" != "" ]; then
        echo "Errant GTID detected after ${i}s: $GTID_ERRANT"
        ERRANT_DETECTED=true
        break
    fi
done

if [ "$ERRANT_DETECTED" = "true" ]; then
    pass "Errant GTID detected on mysql2: $GTID_ERRANT"
else
    fail "Errant GTID not detected on mysql2 within 60s"
fi

# Cleanup: remove the errant database
$COMPOSE exec -T mysql2 \
    mysql -uroot -ptestpass -e "
SET GLOBAL read_only=0;
SET GLOBAL super_read_only=0;
DROP DATABASE IF EXISTS errant_test;
SET GLOBAL read_only=1;
SET GLOBAL super_read_only=1;
" 2>/dev/null

pass "Test 2 cleanup: errant database removed"

# ================================================================
echo ""
echo "--- Test 3: Co-Master Failover ---"

# Setup co-master: mysql1 <-> mysql2 (circular), mysql3 under mysql1
# mysql2 already replicates from mysql1; make mysql1 also replicate from mysql2
$COMPOSE exec -T mysql1 \
    mysql -uroot -ptestpass -e "$CHANGE_TO_MYSQL2 $START_SQL" 2>/dev/null

sleep 5

# Re-seed discovery
curl -s --max-time 10 "$ORC_URL/api/discover/mysql1/3306" > /dev/null
curl -s --max-time 10 "$ORC_URL/api/discover/mysql2/3306" > /dev/null
curl -s --max-time 10 "$ORC_URL/api/discover/mysql3/3306" > /dev/null
sleep 15

# Verify co-master detected
COMASTER_INFO=$(curl -s --max-time 10 "$ORC_URL/api/instance/mysql1/3306" 2>/dev/null | python3 -c "
import json, sys
d = json.load(sys.stdin)
is_comaster = d.get('IsCoMaster', False)
master = d.get('MasterKey', {}).get('Hostname', '')
print(f'{is_comaster}:{master}')
" 2>/dev/null || echo "unknown:unknown")

IS_COMASTER=$(echo "$COMASTER_INFO" | cut -d: -f1)
MASTER_OF_M1=$(echo "$COMASTER_INFO" | cut -d: -f2)

if [ "$IS_COMASTER" = "True" ] && [ "$MASTER_OF_M1" = "mysql2" ]; then
    pass "Co-master topology detected: mysql1 <-> mysql2"
else
    fail "Co-master detection: IsCoMaster=$IS_COMASTER, mysql1 master=$MASTER_OF_M1"
fi

# Kill mysql2 (one co-master)
echo "Stopping mysql2 (co-master)..."
$COMPOSE stop mysql2

echo "Waiting for orchestrator to detect DeadCoMaster and recover (max 90s)..."
RECOVERED=false
for i in $(seq 1 90); do
    RECOVERIES=$(curl -s --max-time 10 "$ORC_URL/api/v2/recoveries" 2>/dev/null)
    HAS_RECOVERY=$(echo "$RECOVERIES" | python3 -c "
import json, sys
d = json.load(sys.stdin)
data = d.get('data', [])
for r in data:
    a = r.get('AnalysisEntry', {}).get('Analysis', '')
    s = r.get('IsSuccessful', False)
    if 'DeadCoMaster' in a and s:
        print('RECOVERED')
        sys.exit(0)
print('WAITING')
" 2>/dev/null)
    if [ "$HAS_RECOVERY" = "RECOVERED" ]; then
        echo "DeadCoMaster recovery detected after ${i}s"
        RECOVERED=true
        break
    fi
    sleep 1
done

if [ "$RECOVERED" = "true" ]; then
    pass "DeadCoMaster detected and recovered"
else
    fail "DeadCoMaster: no recovery detected within 90s"
    echo "  DEBUG: Recent recoveries:"
    curl -s --max-time 10 "$ORC_URL/api/v2/recoveries" 2>/dev/null | python3 -m json.tool 2>/dev/null | head -30
fi

# Cleanup: restart mysql2, restore flat topology
echo "Restarting mysql2 and restoring flat topology..."
$COMPOSE start mysql2
sleep 10

# Wait for mysql2 to be reachable
for i in $(seq 1 30); do
    if $COMPOSE exec -T mysql2 mysql -uroot -ptestpass -e "SELECT 1" >/dev/null 2>&1; then
        break
    fi
    sleep 1
done

# Remove replication on mysql1 (undo co-master)
$COMPOSE exec -T mysql1 \
    mysql -uroot -ptestpass -e "$STOP_SQL $RESET_SQL SET GLOBAL read_only=0; SET GLOBAL super_read_only=0;" 2>/dev/null
# Reconfigure mysql2 as replica of mysql1
$COMPOSE exec -T mysql2 \
    mysql -uroot -ptestpass -e "$STOP_SQL $RESET_SQL $CHANGE_TO_MYSQL1 $START_SQL SET GLOBAL read_only=1; SET GLOBAL super_read_only=1;" 2>/dev/null
# Ensure mysql3 replicates from mysql1
$COMPOSE exec -T mysql3 \
    mysql -uroot -ptestpass -e "$STOP_SQL $CHANGE_TO_MYSQL1 $START_SQL SET GLOBAL read_only=1; SET GLOBAL super_read_only=1;" 2>/dev/null

sleep 5
curl -s --max-time 10 "$ORC_URL/api/discover/mysql1/3306" > /dev/null
curl -s --max-time 10 "$ORC_URL/api/discover/mysql2/3306" > /dev/null
curl -s --max-time 10 "$ORC_URL/api/discover/mysql3/3306" > /dev/null
sleep 5

pass "Test 3 cleanup: flat topology restored"

summary

#!/bin/bash
# Problem detection tests — verify orchestrator detects and clears
# replication problems correctly
set -uo pipefail  # no -e: we handle failures ourselves
cd "$(dirname "$0")/../.."
source tests/functional/lib.sh

echo "=== PROBLEM DETECTION TESTS ==="

COMPOSE="docker compose -f tests/functional/docker-compose.yml"
STOP_SQL=$(mysql_stop_replica_sql)
START_SQL=$(mysql_start_replica_sql)

wait_for_orchestrator || { echo "FATAL: Orchestrator not reachable"; exit 1; }
discover_topology "mysql1"

# ----------------------------------------------------------------
echo ""
echo "--- Test 1: Detect stopped replication ---"

# Stop replication on mysql2
echo "Stopping replication on mysql2..."
$COMPOSE exec -T mysql2 mysql -uroot -ptestpass -e "$STOP_SQL" 2>/dev/null

# Force re-discovery so orchestrator refreshes instance state immediately
curl -s --max-time 10 "$ORC_URL/api/discover/mysql2/3306" > /dev/null 2>&1
sleep 2

# Wait for orchestrator to detect the problem (poll up to 30s)
echo "Waiting for orchestrator to detect stopped replication..."
DETECTED=false
for i in $(seq 1 30); do
    PROBLEMS=$(curl -s --max-time 10 "$ORC_URL/api/problems" 2>/dev/null)
    if echo "$PROBLEMS" | python3 -c "
import json, sys
problems = json.load(sys.stdin)
for p in problems:
    h = p.get('Key', {}).get('Hostname', '')
    if 'mysql2' in h:
        sys.exit(0)
sys.exit(1)
" 2>/dev/null; then
        DETECTED=true
        echo "Problem detected after ${i}s"
        break
    fi
    sleep 1
done

if [ "$DETECTED" = "true" ]; then
    pass "Orchestrator detected stopped replication on mysql2"
else
    fail "Orchestrator did not detect stopped replication on mysql2 within 30s"
fi

# Force another re-discovery to ensure instance data is fresh
curl -s --max-time 10 "$ORC_URL/api/discover/mysql2/3306" > /dev/null 2>&1
sleep 2

# Verify the specific problem shows replication threads stopped
REPL_STATE=$(curl -s --max-time 10 "$ORC_URL/api/instance/mysql2/3306" 2>/dev/null | python3 -c "
import json, sys
inst = json.load(sys.stdin)
sql = inst.get('ReplicationSQLThreadRuning', 'unknown')
io = inst.get('ReplicationIOThreadRuning', 'unknown')
print(f'SQL={sql},IO={io}')
" 2>/dev/null || echo "unknown")

if echo "$REPL_STATE" | grep -q "SQL=False"; then
    pass "Orchestrator reports SQL thread stopped on mysql2"
else
    fail "Orchestrator replication state for mysql2: $REPL_STATE"
fi

# ----------------------------------------------------------------
echo ""
echo "--- Test 1b: Clear stopped replication ---"

# Restart replication
echo "Restarting replication on mysql2..."
$COMPOSE exec -T mysql2 mysql -uroot -ptestpass -e "$START_SQL" 2>/dev/null

# Force re-discovery so orchestrator refreshes instance state
curl -s --max-time 10 "$ORC_URL/api/discover/mysql2/3306" > /dev/null 2>&1
sleep 2

# Wait for orchestrator to see replication running again
echo "Waiting for replication to recover..."
CLEARED=false
for i in $(seq 1 30); do
    # Force re-discovery each iteration
    curl -s --max-time 10 "$ORC_URL/api/discover/mysql2/3306" > /dev/null 2>&1
    REPL_STATE=$(curl -s --max-time 10 "$ORC_URL/api/instance/mysql2/3306" 2>/dev/null | python3 -c "
import json, sys
inst = json.load(sys.stdin)
sql = inst.get('ReplicationSQLThreadRuning', False)
io = inst.get('ReplicationIOThreadRuning', False)
print(f'{sql}:{io}')
" 2>/dev/null || echo "False:False")
    SQL_RUNNING=$(echo "$REPL_STATE" | cut -d: -f1)
    if [ "$SQL_RUNNING" = "True" ]; then
        CLEARED=true
        echo "Replication recovered after ${i}s (SQL=True, IO=$IO)"
        break
    fi
    sleep 1
done

if [ "$CLEARED" = "true" ]; then
    pass "Stopped replication problem cleared after restart"
else
    fail "Replication SQL thread not running on mysql2 after 30s (state: $REPL_STATE)"
fi

# ----------------------------------------------------------------
echo ""
echo "--- Test 2: Detect read_only mismatch (writable replica) ---"

# Make mysql2 writable (it should be read-only as a replica)
echo "Setting mysql2 read_only=0 (simulating writable replica)..."
$COMPOSE exec -T mysql2 mysql -uroot -ptestpass -e "SET GLOBAL read_only=0" 2>/dev/null

# Force re-discovery so orchestrator refreshes instance state
curl -s --max-time 10 "$ORC_URL/api/discover/mysql2/3306" > /dev/null 2>&1
sleep 2

# Wait for orchestrator to detect the problem
echo "Waiting for orchestrator to detect writable replica..."
DETECTED=false
for i in $(seq 1 30); do
    INST=$(curl -s --max-time 10 "$ORC_URL/api/instance/mysql2/3306" 2>/dev/null)
    IS_RO=$(echo "$INST" | python3 -c "import json,sys; print(json.load(sys.stdin).get('ReadOnly', True))" 2>/dev/null || echo "True")
    if [ "$IS_RO" = "False" ]; then
        DETECTED=true
        echo "Writable replica detected after ${i}s"
        break
    fi
    sleep 1
done

if [ "$DETECTED" = "true" ]; then
    pass "Orchestrator detected mysql2 is writable (read_only=false while replicating)"
else
    fail "Orchestrator did not detect writable replica within 30s"
fi

# Restore read_only
echo "Restoring read_only=1 on mysql2..."
$COMPOSE exec -T mysql2 mysql -uroot -ptestpass -e "SET GLOBAL read_only=1" 2>/dev/null

# Force re-discovery
curl -s --max-time 10 "$ORC_URL/api/discover/mysql2/3306" > /dev/null 2>&1
sleep 2

# Wait for it to clear
CLEARED=false
for i in $(seq 1 15); do
    IS_RO=$(curl -s --max-time 10 "$ORC_URL/api/instance/mysql2/3306" 2>/dev/null | python3 -c "import json,sys; print(json.load(sys.stdin).get('ReadOnly', False))" 2>/dev/null || echo "False")
    if [ "$IS_RO" = "True" ]; then
        CLEARED=true
        break
    fi
    sleep 1
done

if [ "$CLEARED" = "true" ]; then
    pass "Writable replica problem cleared after restoring read_only"
else
    fail "read_only still reported as false after 15s"
fi

# ----------------------------------------------------------------
echo ""
echo "--- Test 3: Detect errant GTID ---"

# Inject an errant transaction on mysql3
echo "Injecting errant transaction on mysql3..."
$COMPOSE exec -T mysql3 mysql -uroot -ptestpass -e "
    SET GLOBAL read_only=0;
    SET GLOBAL super_read_only=0;
    CREATE DATABASE IF NOT EXISTS errant_detect_test;
    SET GLOBAL read_only=1;
    SET GLOBAL super_read_only=1;
" 2>/dev/null

# Force re-discovery so orchestrator picks up the errant GTID
curl -s --max-time 10 "$ORC_URL/api/discover/mysql3/3306" > /dev/null 2>&1
sleep 2

# Wait for orchestrator to detect errant GTID
echo "Waiting for orchestrator to detect errant GTID..."
DETECTED=false
for i in $(seq 1 30); do
    GTID_ERRANT=$(curl -s --max-time 10 "$ORC_URL/api/instance/mysql3/3306" 2>/dev/null | python3 -c "
import json, sys
inst = json.load(sys.stdin)
errant = inst.get('GtidErrant', '')
print(errant if errant else '')
" 2>/dev/null || echo "")
    if [ -n "$GTID_ERRANT" ]; then
        DETECTED=true
        echo "Errant GTID detected after ${i}s: $GTID_ERRANT"
        break
    fi
    sleep 1
done

if [ "$DETECTED" = "true" ]; then
    pass "Orchestrator detected errant GTID on mysql3"
else
    fail "Orchestrator did not detect errant GTID within 30s"
fi

# Cleanup errant DB (GTID remains but that's OK)
$COMPOSE exec -T mysql3 mysql -uroot -ptestpass -e "
    SET GLOBAL read_only=0;
    DROP DATABASE IF EXISTS errant_detect_test;
    SET GLOBAL read_only=1;
" 2>/dev/null

summary

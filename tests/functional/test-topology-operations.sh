#!/bin/bash
# Topology refactoring operation tests — verify orchestrator can correctly
# move replicas between masters using GTID-based operations
set -uo pipefail  # no -e: we handle failures ourselves
cd "$(dirname "$0")/../.."
source tests/functional/lib.sh

echo "=== TOPOLOGY REFACTORING OPERATIONS TESTS ==="

COMPOSE="docker compose -f tests/functional/docker-compose.yml"
STOP_SQL=$(mysql_stop_replica_sql)
START_SQL=$(mysql_start_replica_sql)
CHANGE_TO_MYSQL1=$(mysql_change_source_sql mysql1 3306 repl repl_pass)

wait_for_orchestrator || { echo "FATAL: Orchestrator not reachable"; exit 1; }
discover_topology "mysql1"

# Helper: wait for orchestrator to see a specific replication source
wait_for_source() {
    local INSTANCE="$1" EXPECTED_SOURCE="$2" TIMEOUT="${3:-30}"
    for i in $(seq 1 "$TIMEOUT"); do
        SOURCE=$(curl -s --max-time 10 "$ORC_URL/api/instance/$INSTANCE/3306" 2>/dev/null | python3 -c "
import json, sys
inst = json.load(sys.stdin)
print(inst.get('MasterKey', {}).get('Hostname', ''))
" 2>/dev/null || echo "")
        if echo "$SOURCE" | grep -qi "$EXPECTED_SOURCE"; then
            return 0
        fi
        sleep 1
    done
    return 1
}

# Helper: restore flat topology (all replicas directly under mysql1)
restore_flat_topology() {
    echo "Restoring flat topology..."
    for REPLICA in mysql2 mysql3; do
        $COMPOSE exec -T "$REPLICA" mysql -uroot -ptestpass -e "
            $STOP_SQL;
            $CHANGE_TO_MYSQL1;
            $START_SQL;
        " 2>/dev/null
    done
    # Re-seed discovery
    curl -s --max-time 10 "$ORC_URL/api/discover/mysql1/3306" > /dev/null 2>&1
    curl -s --max-time 10 "$ORC_URL/api/discover/mysql2/3306" > /dev/null 2>&1
    curl -s --max-time 10 "$ORC_URL/api/discover/mysql3/3306" > /dev/null 2>&1
    sleep 5
}

# ----------------------------------------------------------------
echo ""
echo "--- Test 1: Relocate replica (mysql3 under mysql2) ---"

# Use the relocate API to move mysql3 under mysql2
echo "Relocating mysql3 under mysql2 via API..."
RESULT=$(curl -s --max-time 30 "$ORC_URL/api/relocate/mysql3/3306/mysql2/3306" 2>/dev/null)
CODE=$(echo "$RESULT" | python3 -c "import json,sys; print(json.load(sys.stdin).get('Code',''))" 2>/dev/null || echo "")

if [ "$CODE" = "OK" ]; then
    pass "Relocate API returned OK"
else
    fail "Relocate API returned: $CODE" "$(echo "$RESULT" | head -c 200)"
fi

# Verify mysql3 now replicates from mysql2
if wait_for_source mysql3 mysql2 15; then
    pass "mysql3 now replicates from mysql2 (verified via API)"
else
    fail "mysql3 did not move under mysql2 within 15s"
fi

# Cross-reference with direct MySQL
ACTUAL_SOURCE=$(mysql_source_host mysql3)
if echo "$ACTUAL_SOURCE" | grep -qi "mysql2"; then
    pass "Direct MySQL confirms mysql3 replicates from mysql2"
else
    fail "Direct MySQL: mysql3 source is '$ACTUAL_SOURCE' (expected mysql2)"
fi

# ----------------------------------------------------------------
echo ""
echo "--- Test 2: Move-up (mysql3 back under mysql1) ---"

# Use move-up to move mysql3 from under mysql2 back to mysql1
echo "Moving mysql3 up via API..."
RESULT=$(curl -s --max-time 30 "$ORC_URL/api/move-up/mysql3/3306" 2>/dev/null)
CODE=$(echo "$RESULT" | python3 -c "import json,sys; print(json.load(sys.stdin).get('Code',''))" 2>/dev/null || echo "")

if [ "$CODE" = "OK" ]; then
    pass "Move-up API returned OK"
else
    fail "Move-up API returned: $CODE" "$(echo "$RESULT" | head -c 200)"
fi

# Verify mysql3 now replicates from mysql1
if wait_for_source mysql3 mysql1 15; then
    pass "mysql3 now replicates from mysql1 after move-up"
else
    fail "mysql3 did not move under mysql1 within 15s"
fi

ACTUAL_SOURCE=$(mysql_source_host mysql3)
if echo "$ACTUAL_SOURCE" | grep -qi "mysql1"; then
    pass "Direct MySQL confirms mysql3 replicates from mysql1 after move-up"
else
    fail "Direct MySQL: mysql3 source is '$ACTUAL_SOURCE' (expected mysql1)"
fi

# ----------------------------------------------------------------
echo ""
echo "--- Test 3: Repoint replica ---"

# Move mysql3 under mysql2 using repoint (GTID-based)
echo "Repointing mysql3 to mysql2 via API..."
RESULT=$(curl -s --max-time 30 "$ORC_URL/api/repoint/mysql3/3306/mysql2/3306" 2>/dev/null)
CODE=$(echo "$RESULT" | python3 -c "import json,sys; print(json.load(sys.stdin).get('Code',''))" 2>/dev/null || echo "")

if [ "$CODE" = "OK" ]; then
    pass "Repoint API returned OK"
else
    fail "Repoint API returned: $CODE" "$(echo "$RESULT" | head -c 200)"
fi

if wait_for_source mysql3 mysql2 15; then
    pass "mysql3 repoints to mysql2 (verified via API)"
else
    fail "mysql3 did not repoint to mysql2 within 15s"
fi

ACTUAL_SOURCE=$(mysql_source_host mysql3)
if echo "$ACTUAL_SOURCE" | grep -qi "mysql2"; then
    pass "Direct MySQL confirms mysql3 replicates from mysql2 after repoint"
else
    fail "Direct MySQL: mysql3 source is '$ACTUAL_SOURCE' (expected mysql2)"
fi

# ----------------------------------------------------------------
echo ""
echo "--- Test 4: Set read-only via API ---"

# Verify we can toggle read_only via the API
echo "Setting mysql2 writeable via API..."
RESULT=$(curl -s --max-time 10 "$ORC_URL/api/set-writeable/mysql2/3306" 2>/dev/null)

sleep 2
ACTUAL_RO=$($COMPOSE exec -T mysql2 mysql -uroot -ptestpass -Nse "SELECT @@read_only" 2>/dev/null | tr -d '[:space:]')
if [ "$ACTUAL_RO" = "0" ]; then
    pass "set-writeable API: mysql2 is now writable (read_only=0)"
else
    fail "set-writeable API: mysql2 read_only=$ACTUAL_RO (expected 0)"
fi

echo "Setting mysql2 read-only via API..."
RESULT=$(curl -s --max-time 10 "$ORC_URL/api/set-read-only/mysql2/3306" 2>/dev/null)

sleep 2
ACTUAL_RO=$($COMPOSE exec -T mysql2 mysql -uroot -ptestpass -Nse "SELECT @@read_only" 2>/dev/null | tr -d '[:space:]')
if [ "$ACTUAL_RO" = "1" ]; then
    pass "set-read-only API: mysql2 is now read-only (read_only=1)"
else
    fail "set-read-only API: mysql2 read_only=$ACTUAL_RO (expected 1)"
fi

# ----------------------------------------------------------------
echo ""
echo "--- Cleanup: Restore flat topology ---"
restore_flat_topology

# Verify clean state
SOURCE2=$(mysql_source_host mysql2)
SOURCE3=$(mysql_source_host mysql3)
if echo "$SOURCE2" | grep -qi "mysql1" && echo "$SOURCE3" | grep -qi "mysql1"; then
    pass "Flat topology restored (mysql2 and mysql3 under mysql1)"
else
    fail "Topology not fully restored: mysql2->$SOURCE2, mysql3->$SOURCE3"
fi

summary

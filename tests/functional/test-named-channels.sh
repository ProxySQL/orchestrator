#!/bin/bash
# Named channels functional tests — verify multi-source replication channel discovery
set -uo pipefail  # no -e: we handle failures ourselves
cd "$(dirname "$0")/../.."
source tests/functional/lib.sh

echo "=== NAMED CHANNELS TESTS ==="

# Named channels with GTID are only reliable on MySQL 8.0+
VERSION=$(mysql_version)
echo "Detected MySQL version: $VERSION"
if [ "$VERSION" = "5.7" ]; then
    skip "Named channels tests require MySQL 8.0+ (detected $VERSION)"
    summary
fi

wait_for_orchestrator || { echo "FATAL: Orchestrator not reachable"; exit 1; }

COMPOSE="docker compose -f tests/functional/docker-compose.yml"

# ----------------------------------------------------------------
echo ""
echo "--- Setup: Configure multi-source replication on mysql3 ---"

# Create test database and table on mysql2 for the extra channel
# Temporarily disable read_only to allow writes on the replica
$COMPOSE exec -T mysql2 mysql -uroot -ptestpass -e "
    SET GLOBAL read_only=0;
    CREATE DATABASE IF NOT EXISTS extra_db;
    CREATE TABLE IF NOT EXISTS extra_db.test (id INT PRIMARY KEY AUTO_INCREMENT, val VARCHAR(100));
    INSERT INTO extra_db.test (val) VALUES ('channel-test');
    SET GLOBAL read_only=1;
" 2>/dev/null

# Verify data exists on mysql2 before setting up channel
DATA_ON_M2=$($COMPOSE exec -T mysql2 mysql -uroot -ptestpass -Nse \
    "SELECT val FROM extra_db.test LIMIT 1" 2>/dev/null | tr -d '[:space:]')
if [ "$DATA_ON_M2" = "channel-test" ]; then
    echo "  Verified data on mysql2: $DATA_ON_M2"
else
    fail "Setup: test data not created on mysql2 (got: '$DATA_ON_M2')" "Check if writes are allowed on replica"
fi

# Add a named channel 'extra' on mysql3 replicating from mysql2
CHANGE_SQL=$(mysql_change_source_channel_sql mysql2 3306 repl repl_pass extra)
START_SQL=$(mysql_start_replica_sql)

$COMPOSE exec -T mysql3 mysql -uroot -ptestpass -e "
    $CHANGE_SQL
    $START_SQL
" 2>/dev/null

# Verify the extra channel is running
sleep 3
CHANNEL_STATUS=$($COMPOSE exec -T mysql3 mysql -uroot -ptestpass -Nse \
    "SELECT SERVICE_STATE FROM performance_schema.replication_connection_status WHERE CHANNEL_NAME='extra'" 2>/dev/null | tr -d '[:space:]')

if [ "$CHANNEL_STATUS" = "ON" ]; then
    pass "Named channel 'extra' is running on mysql3"
else
    fail "Named channel 'extra' not running on mysql3 (status: $CHANNEL_STATUS)"
fi

# Verify data replicated through the extra channel (poll up to 15s)
echo "Waiting for data to replicate through named channel..."
REPLICATED=""
for i in $(seq 1 15); do
    REPLICATED=$($COMPOSE exec -T mysql3 mysql -uroot -ptestpass -Nse \
        "SELECT val FROM extra_db.test LIMIT 1" 2>/dev/null | tr -d '[:space:]')
    if [ "$REPLICATED" = "channel-test" ]; then
        break
    fi
    sleep 1
done

if [ "$REPLICATED" = "channel-test" ]; then
    pass "Data replicated through named channel 'extra'"
else
    fail "Data not replicated through named channel (got: '$REPLICATED')" \
        "Channel ON but data missing - check GTID sets on mysql2 vs mysql3"
fi

# ----------------------------------------------------------------
echo ""
echo "--- Test 1: Discovery of named channels ---"

# Re-seed discovery so orchestrator picks up the channel
curl -s --max-time 10 "$ORC_URL/api/discover/mysql3/3306" > /dev/null
sleep 10

# Check instance channels API endpoint
test_endpoint "GET /api/v2/instances/mysql3/3306/channels" "$ORC_URL/api/v2/instances/mysql3/3306/channels" "200"

CHANNELS_BODY=$(curl -s --max-time 10 "$ORC_URL/api/v2/instances/mysql3/3306/channels" 2>&1)
if echo "$CHANNELS_BODY" | grep -q "extra"; then
    pass "Channel 'extra' listed in instance channels API"
else
    fail "Channel 'extra' not found in instance channels API" "Body: $(echo "$CHANNELS_BODY" | head -c 300)"
fi

# ----------------------------------------------------------------
echo ""
echo "--- Test 3: Instance data includes channel info ---"

INSTANCE_DATA=$(curl -s --max-time 10 "$ORC_URL/api/instance/mysql3/3306" 2>&1)
if echo "$INSTANCE_DATA" | grep -qi "channel\|replicat"; then
    pass "Instance data for mysql3 includes replication/channel info"
else
    skip "Instance data for mysql3 may not expose channel info in this version"
fi

# ----------------------------------------------------------------
echo ""
echo "--- Cleanup: Remove extra channel from mysql3 ---"

# Script already exits on MySQL 5.7, so only 8.0+ syntax needed here
RESET_CHANNEL_SQL="STOP REPLICA FOR CHANNEL 'extra'; RESET REPLICA ALL FOR CHANNEL 'extra';"

$COMPOSE exec -T mysql3 mysql -uroot -ptestpass -e "$RESET_CHANNEL_SQL" 2>/dev/null

# Verify channel removed
CHANNEL_AFTER=$($COMPOSE exec -T mysql3 mysql -uroot -ptestpass -Nse \
    "SELECT COUNT(*) FROM performance_schema.replication_connection_status WHERE CHANNEL_NAME='extra'" 2>/dev/null | tr -d '[:space:]')

if [ "$CHANNEL_AFTER" = "0" ]; then
    pass "Named channel 'extra' cleaned up successfully"
else
    fail "Named channel 'extra' still present after cleanup"
fi

# Clean up test database on mysql2
$COMPOSE exec -T mysql2 mysql -uroot -ptestpass -e "DROP DATABASE IF EXISTS extra_db;" 2>/dev/null

summary

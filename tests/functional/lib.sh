#!/bin/bash
# Shared test helpers for functional tests

PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0
ORC_URL="http://localhost:3099"

pass() {
    echo "  ✅ PASS: $1"
    ((PASS_COUNT++))
}

fail() {
    echo "  ❌ FAIL: $1"
    [ -n "${2:-}" ] && echo "         $2"
    ((FAIL_COUNT++))
}

skip() {
    echo "  ⚠️  SKIP: $1"
    ((SKIP_COUNT++))
}

summary() {
    echo ""
    echo "=== RESULTS: $PASS_COUNT passed, $FAIL_COUNT failed, $SKIP_COUNT skipped ==="
    [ "$FAIL_COUNT" -gt 0 ] && exit 1
    exit 0
}

# Test that an HTTP endpoint returns expected status code
test_endpoint() {
    local NAME="$1" URL="$2" EXPECT="$3"
    local CODE
    CODE=$(curl -s --max-time 10 -o /dev/null -w "%{http_code}" "$URL" 2>&1)
    if [ "$CODE" = "$EXPECT" ]; then
        pass "$NAME (HTTP $CODE)"
    else
        fail "$NAME (HTTP $CODE, expected $EXPECT)"
    fi
}

# Test that response body contains a string
test_body_contains() {
    local NAME="$1" URL="$2" EXPECT="$3"
    local BODY
    BODY=$(curl -s --max-time 10 "$URL" 2>&1)
    if echo "$BODY" | grep -q "$EXPECT"; then
        pass "$NAME"
    else
        fail "$NAME" "Response does not contain '$EXPECT'"
    fi
}

# Wait for orchestrator to be ready
wait_for_orchestrator() {
    echo "Waiting for orchestrator to be ready..."
    for i in $(seq 1 30); do
        if curl -s --max-time 5 -o /dev/null "$ORC_URL/api/clusters" 2>/dev/null; then
            echo "Orchestrator ready after ${i}s"
            return 0
        fi
        sleep 1
    done
    echo "Orchestrator not ready after 30s"
    return 1
}

# Seed discovery and wait for all instances
# Sets CLUSTER_NAME as a global variable
CLUSTER_NAME=""
discover_topology() {
    local MASTER_HOST="$1"
    echo "Seeding discovery with $MASTER_HOST..."
    curl -s --max-time 10 "$ORC_URL/api/discover/$MASTER_HOST/3306" > /dev/null

    # Also seed replicas directly
    curl -s --max-time 10 "$ORC_URL/api/discover/mysql2/3306" > /dev/null 2>&1
    curl -s --max-time 10 "$ORC_URL/api/discover/mysql3/3306" > /dev/null 2>&1

    echo "Waiting for topology discovery..."
    for i in $(seq 1 60); do
        # Get the cluster name dynamically
        CLUSTER_NAME=$(curl -s --max-time 5 "$ORC_URL/api/clusters" 2>/dev/null | python3 -c "import json,sys; c=json.load(sys.stdin); print(c[0] if c else '')" 2>/dev/null || echo "")
        if [ -n "$CLUSTER_NAME" ]; then
            local COUNT
            COUNT=$(curl -s --max-time 5 "$ORC_URL/api/cluster/$CLUSTER_NAME" 2>/dev/null | python3 -c "import json,sys; print(len(json.load(sys.stdin)))" 2>/dev/null || echo "0")
            if [ "$COUNT" -ge 3 ] 2>/dev/null; then
                echo "Full topology discovered (${COUNT} instances, cluster=$CLUSTER_NAME) after ${i}s"
                return 0
            fi
        fi
        # Re-seed replicas periodically
        if [ "$((i % 10))" = "0" ]; then
            curl -s --max-time 10 "$ORC_URL/api/discover/mysql2/3306" > /dev/null 2>&1
            curl -s --max-time 10 "$ORC_URL/api/discover/mysql3/3306" > /dev/null 2>&1
        fi
        sleep 1
    done
    echo "WARNING: Cluster=$CLUSTER_NAME, instances=${COUNT:-0} after 60s"
    return 1
}

# Detect MySQL major version (e.g., "5.7", "8.0", "8.4", "9.0")
# Caches result in MYSQL_MAJOR_VERSION
MYSQL_MAJOR_VERSION=""
mysql_version() {
    if [ -n "$MYSQL_MAJOR_VERSION" ]; then
        echo "$MYSQL_MAJOR_VERSION"
        return
    fi
    local FULL
    FULL=$(docker compose -f tests/functional/docker-compose.yml exec -T mysql1 \
        mysql -uroot -ptestpass -Nse "SELECT VERSION()" 2>/dev/null | tr -d '[:space:]')
    MYSQL_MAJOR_VERSION=$(echo "$FULL" | sed -E 's/^([0-9]+\.[0-9]+).*/\1/')
    echo "$MYSQL_MAJOR_VERSION"
}

# Check if MySQL version is 5.7 (returns 0 for 5.7, 1 otherwise)
mysql_is_57() {
    [ "$(mysql_version)" = "5.7" ]
}

# Return the correct CHANGE REPLICATION SOURCE / CHANGE MASTER command
# Arguments: HOST PORT USER PASSWORD
mysql_change_source_sql() {
    local HOST="$1" PORT="$2" USER="$3" PASS="$4"
    if mysql_is_57; then
        echo "CHANGE MASTER TO MASTER_HOST='${HOST}', MASTER_PORT=${PORT}, MASTER_USER='${USER}', MASTER_PASSWORD='${PASS}', MASTER_AUTO_POSITION=1;"
    else
        echo "CHANGE REPLICATION SOURCE TO SOURCE_HOST='${HOST}', SOURCE_PORT=${PORT}, SOURCE_USER='${USER}', SOURCE_PASSWORD='${PASS}', SOURCE_AUTO_POSITION=1, GET_SOURCE_PUBLIC_KEY=1;"
    fi
}

# Return the correct CHANGE REPLICATION SOURCE / CHANGE MASTER command with FOR CHANNEL
# Arguments: HOST PORT USER PASSWORD CHANNEL
mysql_change_source_channel_sql() {
    local HOST="$1" PORT="$2" USER="$3" PASS="$4" CHANNEL="$5"
    if mysql_is_57; then
        echo "CHANGE MASTER TO MASTER_HOST='${HOST}', MASTER_PORT=${PORT}, MASTER_USER='${USER}', MASTER_PASSWORD='${PASS}', MASTER_AUTO_POSITION=1 FOR CHANNEL '${CHANNEL}';"
    else
        echo "CHANGE REPLICATION SOURCE TO SOURCE_HOST='${HOST}', SOURCE_PORT=${PORT}, SOURCE_USER='${USER}', SOURCE_PASSWORD='${PASS}', SOURCE_AUTO_POSITION=1, GET_SOURCE_PUBLIC_KEY=1 FOR CHANNEL '${CHANNEL}';"
    fi
}

# Return the correct START REPLICA / START SLAVE command
mysql_start_replica_sql() {
    if mysql_is_57; then echo "START SLAVE;"; else echo "START REPLICA;"; fi
}

# Return the correct STOP REPLICA / STOP SLAVE command
mysql_stop_replica_sql() {
    if mysql_is_57; then echo "STOP SLAVE;"; else echo "STOP REPLICA;"; fi
}

# Return the correct RESET REPLICA ALL / RESET SLAVE ALL command
mysql_reset_replica_all_sql() {
    if mysql_is_57; then echo "RESET SLAVE ALL;"; else echo "RESET REPLICA ALL;"; fi
}

# Get ProxySQL servers for a hostgroup
proxysql_servers() {
    local HG="$1"
    docker compose -f tests/functional/docker-compose.yml exec -T proxysql \
        mysql -h127.0.0.1 -P6032 -uradmin -pradmin -Nse \
        "SELECT hostname, port, status FROM runtime_mysql_servers WHERE hostgroup_id=$HG" 2>/dev/null
}

# Get MySQL read_only status
mysql_read_only() {
    local CONTAINER="$1"
    docker compose -f tests/functional/docker-compose.yml exec -T "$CONTAINER" \
        mysql -uroot -ptestpass -Nse "SELECT @@read_only" 2>/dev/null
}

# Get MySQL replication source host
# Uses tab-separated SHOW STATUS — Source_Host is column 2
mysql_source_host() {
    local CONTAINER="$1"
    if mysql_is_57; then
        docker compose -f tests/functional/docker-compose.yml exec -T "$CONTAINER" \
            mysql -uroot -ptestpass -Nse "SHOW SLAVE STATUS" 2>/dev/null | awk -F'\t' '{print $2; exit}'
    else
        docker compose -f tests/functional/docker-compose.yml exec -T "$CONTAINER" \
            mysql -uroot -ptestpass -Nse "SHOW REPLICA STATUS" 2>/dev/null | awk -F'\t' '{print $2; exit}'
    fi
}

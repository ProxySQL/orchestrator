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
    [ -n "$2" ] && echo "         $2"
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
    CODE=$(curl -s -o /dev/null -w "%{http_code}" "$URL" 2>&1)
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
    BODY=$(curl -s "$URL" 2>&1)
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
        if curl -s -o /dev/null "$ORC_URL/api/clusters" 2>/dev/null; then
            echo "Orchestrator ready after ${i}s"
            return 0
        fi
        sleep 1
    done
    echo "Orchestrator not ready after 30s"
    return 1
}

# Seed discovery and wait for all instances
discover_topology() {
    local MASTER_HOST="$1"
    echo "Seeding discovery with $MASTER_HOST..."
    curl -s "$ORC_URL/api/discover/$MASTER_HOST/3306" > /dev/null

    echo "Waiting for topology discovery..."
    for i in $(seq 1 60); do
        local COUNT
        COUNT=$(curl -s "$ORC_URL/api/cluster/$MASTER_HOST:3306" 2>/dev/null | python3 -c "import json,sys; print(len(json.load(sys.stdin)))" 2>/dev/null)
        if [ "$COUNT" = "3" ]; then
            echo "Full topology discovered (3 instances) after ${i}s"
            return 0
        fi
        # Also try to discover replicas directly
        if [ "$i" = "10" ] || [ "$i" = "20" ]; then
            curl -s "$ORC_URL/api/discover/mysql2/3306" > /dev/null 2>&1
            curl -s "$ORC_URL/api/discover/mysql3/3306" > /dev/null 2>&1
        fi
        sleep 1
    done
    echo "WARNING: Only discovered $COUNT instances after 60s"
    return 1
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

# Get MySQL replication source
mysql_source_host() {
    local CONTAINER="$1"
    docker compose -f tests/functional/docker-compose.yml exec -T "$CONTAINER" \
        mysql -uroot -ptestpass -Nse "SHOW REPLICA STATUS\G" 2>/dev/null | grep "Source_Host" | awk '{print $2}'
}

#!/bin/bash
# Verify MariaDB read_only enum values are normalized correctly by discovery.
set -uo pipefail
cd "$(dirname "$0")/../.." || exit 1
source tests/functional/lib.sh

echo "=== MARIADB READ_ONLY DISCOVERY TESTS ==="

COMPOSE="${COMPOSE:-docker compose -f tests/functional/docker-compose.yml -f tests/functional/docker-compose.mariadb.yml}"
TEST_INSTANCE="mysql2"

restore_read_only() {
    $COMPOSE exec -T "$TEST_INSTANCE" mysql -uroot -ptestpass \
        -e "SET GLOBAL read_only=ON" >/dev/null 2>&1 || true
    curl -s --max-time 10 "$ORC_URL/api/discover/$TEST_INSTANCE/3306" >/dev/null 2>&1 || true
}
trap restore_read_only EXIT

wait_for_orchestrator || { echo "FATAL: Orchestrator not reachable"; exit 1; }
discover_topology "mysql1" || { echo "FATAL: Topology not discovered"; exit 1; }

wait_for_api_read_only() {
    local EXPECTED="$1"
    local ACTUAL=""
    for _ in $(seq 1 20); do
        curl -s --max-time 10 "$ORC_URL/api/discover/$TEST_INSTANCE/3306" >/dev/null 2>&1
        ACTUAL=$(curl -s --max-time 10 "$ORC_URL/api/instance/$TEST_INSTANCE/3306" 2>/dev/null | python3 -c \
            "import json,sys; print(str(json.load(sys.stdin).get('ReadOnly')).lower())" 2>/dev/null || echo "")
        if [ "$ACTUAL" = "$EXPECTED" ]; then
            return 0
        fi
        sleep 1
    done
    echo "last API ReadOnly value: ${ACTUAL:-unavailable}"
    return 1
}

check_mode() {
    local MODE="$1"
    local EXPECTED="$2"
    if ! $COMPOSE exec -T "$TEST_INSTANCE" mysql -uroot -ptestpass \
        -e "SET GLOBAL read_only=$MODE" >/dev/null 2>&1; then
        fail "MariaDB rejected read_only=$MODE"
        return
    fi
    if wait_for_api_read_only "$EXPECTED"; then
        pass "read_only=$MODE is reported as ReadOnly=$EXPECTED"
    else
        fail "read_only=$MODE was not reported as ReadOnly=$EXPECTED"
    fi
}

check_mode OFF false
check_mode ON true

MARIADB_MAJOR="$(mysql_version)"
MARIADB_MAJOR="${MARIADB_MAJOR%%.*}"
if [ "$MARIADB_MAJOR" -ge 12 ]; then
    check_mode NO_LOCK true
    check_mode NO_LOCK_NO_ADMIN true
else
    skip "NO_LOCK modes require MariaDB 12 or newer"
fi

summary

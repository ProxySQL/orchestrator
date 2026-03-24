#!/bin/bash
# Run all functional tests: infrastructure setup, smoke, failover, regression
set -euo pipefail
cd "$(dirname "$0")/../.."

COMPOSE="docker compose -f tests/functional/docker-compose.yml"

echo "=== FUNCTIONAL TEST SUITE ==="
echo ""

# ----------------------------------------------------------------
echo "--- Step 1: Build orchestrator ---"
go build -o bin/orchestrator ./go/cmd/orchestrator
echo "Build OK"

# ----------------------------------------------------------------
echo ""
echo "--- Step 2: Start test infrastructure ---"
$COMPOSE down -v --remove-orphans 2>/dev/null || true
$COMPOSE up -d

echo "Waiting for all services to be healthy..."
for i in $(seq 1 90); do
    HEALTHY=$($COMPOSE ps --format json 2>/dev/null | python3 -c "
import json, sys
healthy = 0
for line in sys.stdin:
    svc = json.loads(line)
    if svc.get('Health','') == 'healthy' or 'healthy' in svc.get('Status','').lower():
        healthy += 1
print(healthy)
" 2>/dev/null || echo "0")
    if [ "$HEALTHY" -ge 4 ]; then
        echo "All 4 services healthy after ${i}s"
        break
    fi
    if [ "$i" -eq 90 ]; then
        echo "FATAL: Services not healthy after 90s"
        $COMPOSE ps
        $COMPOSE logs --tail=20
        exit 1
    fi
    sleep 1
done

# Verify replication is running
echo "Verifying replication..."
for i in $(seq 1 30); do
    REPL_OK=$($COMPOSE exec -T mysql2 mysql -uroot -ptestpass -Nse "SHOW REPLICA STATUS\G" 2>/dev/null | grep "Replica_IO_Running: Yes" | wc -l)
    if [ "$REPL_OK" -ge 1 ]; then
        echo "Replication is running"
        break
    fi
    sleep 1
done

# ----------------------------------------------------------------
echo ""
echo "--- Step 3: Start orchestrator ---"
rm -f /tmp/orchestrator-test.sqlite3
bin/orchestrator -config tests/functional/orchestrator-test.conf.json http > /tmp/orchestrator-test.log 2>&1 &
ORC_PID=$!
echo $ORC_PID > /tmp/orchestrator-test.pid
echo "Orchestrator started (PID: $ORC_PID)"

# ----------------------------------------------------------------
echo ""
echo "--- Step 4: Run smoke tests ---"
bash tests/functional/test-smoke.sh
SMOKE_EXIT=$?

echo ""
echo "--- Step 5: Run regression tests ---"
bash tests/functional/test-regression.sh
REGRESSION_EXIT=$?

echo ""
echo "--- Step 6: Run failover tests ---"
bash tests/functional/test-failover.sh
FAILOVER_EXIT=$?

# ----------------------------------------------------------------
echo ""
echo "--- Cleanup ---"
kill $ORC_PID 2>/dev/null || true
$COMPOSE down -v --remove-orphans 2>/dev/null || true
rm -f /tmp/orchestrator-test.sqlite3 /tmp/orchestrator-test.pid

echo ""
echo "=== FUNCTIONAL TEST SUITE COMPLETE ==="
echo "Smoke:      exit $SMOKE_EXIT"
echo "Regression: exit $REGRESSION_EXIT"
echo "Failover:   exit $FAILOVER_EXIT"

# Exit with failure if any suite failed
[ "$SMOKE_EXIT" -ne 0 ] || [ "$REGRESSION_EXIT" -ne 0 ] || [ "$FAILOVER_EXIT" -ne 0 ] && exit 1
exit 0

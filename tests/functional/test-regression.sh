#!/bin/bash
# Tier C: Regression tests — verify all API endpoints and features
set -uo pipefail  # no -e: we handle failures ourselves
cd "$(dirname "$0")/../.."
source tests/functional/lib.sh

echo "=== TIER C: REGRESSION TESTS ==="

wait_for_orchestrator || { echo "FATAL: Orchestrator not reachable"; exit 1; }

# ----------------------------------------------------------------
echo ""
echo "--- Chi Router v1 API Regression ---"
test_endpoint "GET /api/clusters" "$ORC_URL/api/clusters" "200"
test_endpoint "GET /api/problems" "$ORC_URL/api/problems" "200"
test_endpoint "GET /api/audit-recovery" "$ORC_URL/api/audit-recovery" "200"
test_endpoint "GET /api/maintenance" "$ORC_URL/api/maintenance" "200"

# ----------------------------------------------------------------
echo ""
echo "--- API v2 Validation ---"
test_endpoint "GET /api/v2/clusters" "$ORC_URL/api/v2/clusters" "200"
test_endpoint "GET /api/v2/status" "$ORC_URL/api/v2/status" "200"
test_endpoint "GET /api/v2/recoveries" "$ORC_URL/api/v2/recoveries" "200"
test_endpoint "GET /api/v2/proxysql/servers" "$ORC_URL/api/v2/proxysql/servers" "200"
test_body_contains "V2 envelope: status field" "$ORC_URL/api/v2/clusters" '"status"'
test_body_contains "V2 envelope: data field" "$ORC_URL/api/v2/clusters" '"data"'

# Proper error codes
V2_404=$(curl -s -o /dev/null -w "%{http_code}" "$ORC_URL/api/v2/instances/nonexistent/9999")
if [ "$V2_404" = "404" ]; then
    pass "V2 returns 404 for unknown instance"
else
    fail "V2 returns $V2_404 for unknown instance (expected 404)"
fi

# ----------------------------------------------------------------
echo ""
echo "--- Prometheus Metrics ---"
test_endpoint "GET /metrics" "$ORC_URL/metrics" "200"
test_body_contains "Metric: orchestrator_instances_total" "$ORC_URL/metrics" "orchestrator_instances_total"
test_body_contains "Metric: orchestrator_clusters_total" "$ORC_URL/metrics" "orchestrator_clusters_total"
test_body_contains "Metric: orchestrator_discoveries_total" "$ORC_URL/metrics" "orchestrator_discoveries_total"
test_body_contains "Metric: orchestrator_recoveries_total" "$ORC_URL/metrics" "orchestrator_recoveries_total"
test_body_contains "Prometheus format: HELP line" "$ORC_URL/metrics" "# HELP"
test_body_contains "Prometheus format: TYPE line" "$ORC_URL/metrics" "# TYPE"

# ----------------------------------------------------------------
echo ""
echo "--- Health Endpoints ---"
test_endpoint "GET /health/live" "$ORC_URL/health/live" "200"
test_endpoint "GET /health/ready" "$ORC_URL/health/ready" "200"
test_endpoint "GET /health/leader" "$ORC_URL/health/leader" "200"

# ----------------------------------------------------------------
echo ""
echo "--- ProxySQL API ---"
test_endpoint "GET /api/proxysql/servers" "$ORC_URL/api/proxysql/servers" "200"
test_body_contains "ProxySQL servers: mysql data" "$ORC_URL/api/proxysql/servers" "mysql"

# ----------------------------------------------------------------
echo ""
echo "--- Web UI & Static Files ---"
test_endpoint "GET / (root)" "$ORC_URL/" "302"
test_endpoint "GET /css/orchestrator.css" "$ORC_URL/css/orchestrator.css" "200"
test_endpoint "GET /js/orchestrator.js" "$ORC_URL/js/orchestrator.js" "200"

summary

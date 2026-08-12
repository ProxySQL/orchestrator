#!/bin/bash
# Tier A: Smoke tests — verify basic functionality against real services
set -uo pipefail  # no -e: we handle failures ourselves
cd "$(dirname "$0")/../.."
source tests/functional/lib.sh

echo "=== TIER A: SMOKE TESTS ==="

# Prerequisites
wait_for_orchestrator || { echo "FATAL: Orchestrator not reachable"; exit 1; }
discover_topology "mysql1"

echo ""
echo "--- Discovery ---"

# Get the actual cluster name (may differ from simple "mysql1")
CLUSTERS=$(curl -s --max-time 10 "$ORC_URL/api/clusters" 2>/dev/null)
CLUSTER_NAME=$(echo "$CLUSTERS" | python3 -c "import json,sys; c=json.load(sys.stdin); print(c[0] if c else '')" 2>/dev/null || echo "")
echo "  Cluster name: $CLUSTER_NAME"

if [ -n "$CLUSTER_NAME" ]; then
    pass "Cluster discovered: $CLUSTER_NAME"
else
    fail "No cluster discovered" "Response: $CLUSTERS"
fi

INST_COUNT=$(curl -s --max-time 10 "$ORC_URL/api/cluster/$CLUSTER_NAME" 2>/dev/null | python3 -c "import json,sys; print(len(json.load(sys.stdin)))" 2>/dev/null || echo "0")
if [ "$INST_COUNT" -ge 2 ]; then
    pass "Instances discovered: $INST_COUNT"
else
    fail "Instances discovered: $INST_COUNT (expected >= 2)"
fi

echo ""
echo "--- Web UI ---"
test_endpoint "Web UI root" "$ORC_URL/" "302"
test_endpoint "Static CSS" "$ORC_URL/css/orchestrator.css" "200"
test_endpoint "Static JS" "$ORC_URL/js/orchestrator.js" "200"
test_endpoint "Cluster workspace" "$ORC_URL/web/cluster/mysql1:3306" "200"
test_body_contains "Cluster workspace shell" "$ORC_URL/web/cluster/mysql1:3306" 'id="cluster_workspace"'
test_body_contains "Cluster workspace stylesheet" "$ORC_URL/web/cluster/mysql1:3306" 'cluster-workspace\.css'
test_endpoint "Failure analysis workspace" "$ORC_URL/web/clusters-analysis" "200"
test_body_contains "Failure analysis shell" "$ORC_URL/web/clusters-analysis" 'id="clusters_analysis_workspace"'
test_body_contains "Failure analysis stylesheet" "$ORC_URL/web/clusters-analysis" 'clusters-analysis-workspace\.css'

echo ""
echo "--- API v1 ---"
test_endpoint "GET /api/clusters" "$ORC_URL/api/clusters" "200"
test_endpoint "GET /api/problems" "$ORC_URL/api/problems" "200"
test_endpoint "GET /api/audit-recovery" "$ORC_URL/api/audit-recovery" "200"
test_endpoint "GET /api/maintenance" "$ORC_URL/api/maintenance" "200"

echo ""
echo "--- API v2 ---"
test_endpoint "GET /api/v2/clusters" "$ORC_URL/api/v2/clusters" "200"
test_endpoint "GET /api/v2/status" "$ORC_URL/api/v2/status" "200"
test_endpoint "GET /api/v2/recoveries" "$ORC_URL/api/v2/recoveries" "200"
test_endpoint "GET /api/v2/proxysql/servers" "$ORC_URL/api/v2/proxysql/servers" "200"
test_body_contains "V2 response has status field" "$ORC_URL/api/v2/clusters" '"status"'

V2_404=$(curl -s --max-time 10 -o /dev/null -w "%{http_code}" "$ORC_URL/api/v2/instances/nonexistent/9999")
if [ "$V2_404" = "404" ]; then
    pass "V2 returns 404 for unknown instance"
else
    fail "V2 returns $V2_404 for unknown instance (expected 404)"
fi

echo ""
echo "--- Prometheus ---"
test_endpoint "GET /metrics" "$ORC_URL/metrics" "200"
test_body_contains "Metric: orchestrator_instances_total" "$ORC_URL/metrics" "orchestrator_instances_total"
test_body_contains "Metric: orchestrator_clusters_total" "$ORC_URL/metrics" "orchestrator_clusters_total"
test_body_contains "Metric: orchestrator_discoveries_total" "$ORC_URL/metrics" "orchestrator_discoveries_total"

echo ""
echo "--- Health Endpoints ---"
test_endpoint "GET /health/live" "$ORC_URL/health/live" "200"
test_endpoint "GET /health/ready" "$ORC_URL/health/ready" "200"
test_endpoint "GET /health/leader" "$ORC_URL/health/leader" "200"

echo ""
echo "--- ProxySQL ---"
test_endpoint "GET /api/proxysql/servers" "$ORC_URL/api/proxysql/servers" "200"
test_body_contains "ProxySQL returns server data" "$ORC_URL/api/proxysql/servers" "mysql1"

# CLI tests: run via docker exec inside the orchestrator container
# (CLI needs to reach ProxySQL by Docker hostname)
echo ""
echo "--- ProxySQL CLI ---"
COMPOSE="docker compose -f tests/functional/docker-compose.yml"
PSQL_TEST=$($COMPOSE exec -T -w /orchestrator orchestrator orchestrator -config /orchestrator/orchestrator.conf.json -c proxysql-test 2>&1 || true)
if echo "$PSQL_TEST" | grep -q "connection: OK"; then
    pass "proxysql-test CLI"
else
    fail "proxysql-test CLI" "$(echo "$PSQL_TEST" | tail -1)"
fi

PSQL_SERVERS=$($COMPOSE exec -T -w /orchestrator orchestrator orchestrator -config /orchestrator/orchestrator.conf.json -c proxysql-servers 2>&1 || true)
if echo "$PSQL_SERVERS" | grep -q "mysql1"; then
    pass "proxysql-servers CLI"
else
    fail "proxysql-servers CLI" "$(echo "$PSQL_SERVERS" | tail -1)"
fi

summary

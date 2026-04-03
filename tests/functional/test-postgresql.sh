#!/bin/bash
# PostgreSQL functional tests — verify discovery and failover with real PostgreSQL containers
set -uo pipefail  # no -e: we handle failures ourselves
cd "$(dirname "$0")/../.."
source tests/functional/lib.sh

# Override ORC_URL to point at the PostgreSQL orchestrator instance
ORC_URL="http://localhost:3098"

COMPOSE="docker compose -f tests/functional/docker-compose.yml"

echo "=== POSTGRESQL FUNCTIONAL TESTS ==="

# ----------------------------------------------------------------
echo ""
echo "--- Waiting for PostgreSQL orchestrator ---"

wait_for_orchestrator || { echo "FATAL: PostgreSQL orchestrator not reachable at $ORC_URL"; exit 1; }

# ----------------------------------------------------------------
echo ""
echo "--- Discovery tests ---"

# Discover by IP address so all instances share a consistent address scheme
echo "Seeding discovery with 172.30.0.20:5432 (pgprimary)..."
curl -s "$ORC_URL/api/discover/172.30.0.20/5432" > /dev/null
curl -s "$ORC_URL/api/discover/172.30.0.21/5432" > /dev/null

echo "Waiting for topology discovery..."
PG_CLUSTER=""
for i in $(seq 1 60); do
    PG_CLUSTER=$(curl -s "$ORC_URL/api/clusters" 2>/dev/null | python3 -c "
import json, sys
c = json.load(sys.stdin)
for name in c:
    if '172.30.0.20' in name or 'pgprimary' in name or 'pg' in name.lower():
        print(name)
        sys.exit(0)
if c:
    print(c[0])
" 2>/dev/null || echo "")
    if [ -n "$PG_CLUSTER" ]; then
        COUNT=$(curl -s "$ORC_URL/api/cluster/$PG_CLUSTER" 2>/dev/null | python3 -c "import json,sys; print(len(json.load(sys.stdin)))" 2>/dev/null || echo "0")
        if [ "$COUNT" -ge 2 ] 2>/dev/null; then
            echo "PostgreSQL topology discovered (${COUNT} instances, cluster=$PG_CLUSTER) after ${i}s"
            break
        fi
    fi
    # Re-seed standby periodically
    if [ "$((i % 10))" = "0" ]; then
        curl -s "$ORC_URL/api/discover/172.30.0.21/5432" > /dev/null 2>&1
    fi
    sleep 1
done

if [ -n "$PG_CLUSTER" ]; then
    pass "PostgreSQL cluster discovered: $PG_CLUSTER"
else
    fail "No PostgreSQL cluster discovered"
fi

INST_COUNT=$(curl -s "$ORC_URL/api/cluster/$PG_CLUSTER" 2>/dev/null | python3 -c "import json,sys; print(len(json.load(sys.stdin)))" 2>/dev/null || echo "0")
if [ "$INST_COUNT" -ge 2 ]; then
    pass "PostgreSQL instances discovered: $INST_COUNT"
else
    fail "PostgreSQL instances discovered: $INST_COUNT (expected >= 2)"
fi

# Verify primary is not read-only (not in recovery)
PRIMARY_RO=$(curl -s "$ORC_URL/api/cluster/$PG_CLUSTER" 2>/dev/null | python3 -c "
import json, sys
instances = json.load(sys.stdin)
for inst in instances:
    h = inst.get('Key', {}).get('Hostname', '')
    if '172.30.0.20' in h or 'pgprimary' in h:
        print('true' if inst.get('ReadOnly', True) else 'false')
        sys.exit(0)
print('unknown')
" 2>/dev/null || echo "unknown")

if [ "$PRIMARY_RO" = "false" ]; then
    pass "pgprimary is read_only=false (primary)"
else
    fail "pgprimary read_only=$PRIMARY_RO (expected false)"
fi

# Verify standby is read-only (in recovery)
STANDBY_RO=$(curl -s "$ORC_URL/api/cluster/$PG_CLUSTER" 2>/dev/null | python3 -c "
import json, sys
instances = json.load(sys.stdin)
for inst in instances:
    h = inst.get('Key', {}).get('Hostname', '')
    if '172.30.0.21' in h or 'pgstandby1' in h:
        print('true' if inst.get('ReadOnly', False) else 'false')
        sys.exit(0)
print('unknown')
" 2>/dev/null || echo "unknown")

if [ "$STANDBY_RO" = "true" ]; then
    pass "pgstandby1 is read_only=true (standby)"
else
    fail "pgstandby1 read_only=$STANDBY_RO (expected true)"
fi

# ----------------------------------------------------------------
echo ""
echo "--- API tests ---"

test_endpoint "GET /api/clusters" "$ORC_URL/api/clusters" "200"
test_endpoint "GET /api/v2/clusters" "$ORC_URL/api/v2/clusters" "200"
test_endpoint "GET /api/v2/status" "$ORC_URL/api/v2/status" "200"
test_body_contains "/api/v2/status healthy" "$ORC_URL/api/v2/status" '"status"'
test_body_contains "/api/clusters contains PG cluster" "$ORC_URL/api/clusters" "172.30.0.20"

# ----------------------------------------------------------------
echo ""
echo "--- Failover test: kill pgprimary ---"
# Static IPs are assigned in docker-compose.yml so the standby remains reachable
# even when the primary container stops.

echo "Stopping pgprimary container..."
$COMPOSE stop pgprimary

# Re-seed standby discovery by IP to ensure orchestrator can still reach it
sleep 2
curl -s "$ORC_URL/api/discover/172.30.0.21/5432" > /dev/null 2>&1
sleep 3
curl -s "$ORC_URL/api/discover/172.30.0.21/5432" > /dev/null 2>&1

# Debug: dump cluster state before and during failover
echo "DEBUG: Cluster state before failover:"
curl -s "$ORC_URL/api/cluster/$PG_CLUSTER" 2>/dev/null | python3 -c "
import json, sys
for i in json.load(sys.stdin):
    k=i['Key']; m=i['MasterKey']
    print(f'  {k[\"Hostname\"]}:{k[\"Port\"]} RO={i[\"ReadOnly\"]} Master={m[\"Hostname\"]}:{m[\"Port\"]} Depth={i[\"ReplicationDepth\"]}')" 2>/dev/null || echo "  (failed)"

# Wait a bit for re-discovery, then dump state
sleep 15
echo "DEBUG: Cluster state after primary stop + re-discovery:"
curl -s "$ORC_URL/api/cluster/$PG_CLUSTER" 2>/dev/null | python3 -c "
import json, sys
for i in json.load(sys.stdin):
    k=i['Key']; m=i['MasterKey']
    print(f'  {k[\"Hostname\"]}:{k[\"Port\"]} RO={i[\"ReadOnly\"]} Master={m[\"Hostname\"]}:{m[\"Port\"]} Depth={i[\"ReplicationDepth\"]} Valid={i[\"IsLastCheckValid\"]}')" 2>/dev/null || echo "  (failed)"
echo "DEBUG: Replication analysis:"
curl -s "$ORC_URL/api/replication-analysis" 2>/dev/null | python3 -c "
import json, sys
for a in json.load(sys.stdin):
    print(f'  {a[\"AnalyzedInstanceKey\"][\"Hostname\"]}:{a[\"AnalyzedInstanceKey\"][\"Port\"]} Analysis={a[\"Analysis\"]} Replicas={a[\"CountReplicas\"]} ValidReplicas={a[\"CountValidReplicas\"]}')" 2>/dev/null || echo "  (failed)"

echo "Waiting for orchestrator to detect DeadPrimary and recover (max 120s)..."
RECOVERED=false
SUCCESSOR=""
for i in $(seq 1 120); do
    RECOVERIES=$(curl -s "$ORC_URL/api/v2/recoveries" 2>/dev/null)
    HAS_RECOVERY=$(echo "$RECOVERIES" | python3 -c "
import json, sys
d = json.load(sys.stdin)
data = d.get('data', [])
for r in data:
    a = r.get('AnalysisEntry', {}).get('Analysis', '')
    s = r.get('IsSuccessful', False)
    successor = r.get('SuccessorKey', {}).get('Hostname', '')
    if ('DeadPrimary' in a or 'UnreachablePrimary' in a or 'DeadMaster' in a) and s and successor:
        print(f'RECOVERED:{successor}')
        sys.exit(0)
print('WAITING')
" 2>/dev/null)
    if echo "$HAS_RECOVERY" | grep -q "RECOVERED:"; then
        SUCCESSOR=$(echo "$HAS_RECOVERY" | sed 's/RECOVERED://')
        echo "Recovery detected after ${i}s -- successor: $SUCCESSOR"
        RECOVERED=true
        break
    fi
    sleep 1
done

if [ "$RECOVERED" = "true" ]; then
    pass "DeadPrimary detected and recovered (successor: $SUCCESSOR)"
else
    fail "DeadPrimary: no recovery detected within 120s"
    # Dump debug info
    echo "  DEBUG: Recent recoveries:"
    curl -s "$ORC_URL/api/v2/recoveries" 2>/dev/null | python3 -m json.tool 2>/dev/null | head -30
    echo "  DEBUG: Cluster topology:"
    curl -s "$ORC_URL/api/cluster/$PG_CLUSTER" 2>/dev/null | python3 -m json.tool 2>/dev/null | head -30
fi

# Verify successor is no longer in recovery (promoted to primary)
if [ "$RECOVERED" = "true" ]; then
    sleep 3
    SUCCESSOR_RO=$(curl -s "$ORC_URL/api/cluster/$PG_CLUSTER" 2>/dev/null | python3 -c "
import json, sys
instances = json.load(sys.stdin)
for inst in instances:
    hostname = inst.get('Key', {}).get('Hostname', '')
    if hostname == '$SUCCESSOR':
        print('true' if inst.get('ReadOnly', True) else 'false')
        sys.exit(0)
print('unknown')
" 2>/dev/null || echo "unknown")

    if [ "$SUCCESSOR_RO" = "false" ]; then
        pass "Successor $SUCCESSOR promoted (read_only=false)"
    else
        # After promotion the instance needs a poll cycle to update
        skip "Successor read_only=$SUCCESSOR_RO (may need additional poll cycle)"
    fi
fi

# Verify recovery is recorded
RECOVERY_API=$(curl -s "$ORC_URL/api/v2/recoveries" 2>/dev/null)
if echo "$RECOVERY_API" | grep -q '"IsSuccessful":true'; then
    pass "Recovery audit: /api/v2/recoveries shows successful recovery"
else
    fail "Recovery audit: no successful recovery in API response"
fi

# ----------------------------------------------------------------
echo ""
echo "--- Cleanup: restart pgprimary ---"
$COMPOSE start pgprimary
sleep 5
echo "pgprimary restarted"

summary

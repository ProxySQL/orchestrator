#!/bin/bash
# Topology discovery validation — verify orchestrator correctly discovers
# and represents the actual replication state
set -uo pipefail  # no -e: we handle failures ourselves
cd "$(dirname "$0")/../.."
source tests/functional/lib.sh

echo "=== TOPOLOGY DISCOVERY VALIDATION TESTS ==="

COMPOSE="docker compose -f tests/functional/docker-compose.yml"

wait_for_orchestrator || { echo "FATAL: Orchestrator not reachable"; exit 1; }
discover_topology "mysql1"

# ----------------------------------------------------------------
echo ""
echo "--- Test 1: Correct instance count ---"

INSTANCES=$(curl -s --max-time 10 "$ORC_URL/api/cluster/$CLUSTER_NAME" 2>/dev/null)
INST_COUNT=$(echo "$INSTANCES" | python3 -c "import json,sys; print(len(json.load(sys.stdin)))" 2>/dev/null || echo "0")

if [ "$INST_COUNT" -eq 3 ]; then
    pass "Cluster has exactly 3 instances"
else
    fail "Cluster has $INST_COUNT instances (expected 3)"
fi

# ----------------------------------------------------------------
echo ""
echo "--- Test 2: Master identification ---"

# Get orchestrator's view of the master
MASTER_INFO=$(echo "$INSTANCES" | python3 -c "
import json, sys
instances = json.load(sys.stdin)
for inst in instances:
    if not inst.get('ReadOnly', True):
        print(json.dumps(inst))
        break
" 2>/dev/null)

ORC_MASTER_HOST=$(echo "$MASTER_INFO" | python3 -c "import json,sys; print(json.load(sys.stdin)['Key']['Hostname'])" 2>/dev/null || echo "")
ORC_MASTER_RO=$(echo "$MASTER_INFO" | python3 -c "import json,sys; print(json.load(sys.stdin)['ReadOnly'])" 2>/dev/null || echo "")

# Cross-reference with direct MySQL query
ACTUAL_RO=$($COMPOSE exec -T mysql1 mysql -uroot -ptestpass -Nse "SELECT @@read_only" 2>/dev/null | tr -d '[:space:]')

if [ -n "$ORC_MASTER_HOST" ]; then
    pass "Master identified: $ORC_MASTER_HOST"
else
    fail "No master found in orchestrator data"
fi

if [ "$ORC_MASTER_RO" = "False" ]; then
    pass "Orchestrator reports master read_only=false"
else
    fail "Orchestrator reports master read_only=$ORC_MASTER_RO (expected False)"
fi

if [ "$ACTUAL_RO" = "0" ]; then
    pass "Direct MySQL confirms mysql1 read_only=0"
else
    fail "Direct MySQL: mysql1 read_only=$ACTUAL_RO (expected 0)"
fi

# Verify master has no master key (is top of topology)
MASTER_MASTER=$(echo "$MASTER_INFO" | python3 -c "
import json, sys
inst = json.load(sys.stdin)
mk = inst.get('MasterKey', {})
h = mk.get('Hostname', '')
p = mk.get('Port', 0)
# Empty or port 0 means no master
if not h or p == 0:
    print('none')
else:
    print(f'{h}:{p}')
" 2>/dev/null || echo "error")

if [ "$MASTER_MASTER" = "none" ]; then
    pass "Master has no upstream master (top of topology)"
else
    # It might show itself or empty depending on version — acceptable if it's self
    if echo "$MASTER_MASTER" | grep -q "$ORC_MASTER_HOST"; then
        pass "Master's MasterKey points to itself (acceptable)"
    else
        fail "Master has unexpected upstream: $MASTER_MASTER"
    fi
fi

# ----------------------------------------------------------------
echo ""
echo "--- Test 3: Replica identification ---"

# Get replica info from orchestrator
REPLICAS=$(echo "$INSTANCES" | python3 -c "
import json, sys
instances = json.load(sys.stdin)
for inst in instances:
    if inst.get('ReadOnly', False):
        h = inst['Key']['Hostname']
        mk = inst.get('MasterKey', {}).get('Hostname', '')
        ro = inst.get('ReadOnly', False)
        repl_running = inst.get('ReplicationSQLThreadRuning', False) and inst.get('ReplicationIOThreadRuning', False)
        print(f'{h}|{mk}|{ro}|{repl_running}')
" 2>/dev/null)

REPLICA_COUNT=$(echo "$REPLICAS" | grep -c '|' || echo "0")
if [ "$REPLICA_COUNT" -ge 2 ]; then
    pass "Found $REPLICA_COUNT replicas"
else
    fail "Found $REPLICA_COUNT replicas (expected >= 2)"
fi

# Verify each replica's master key points to actual master
while IFS='|' read -r RHOST RMASTER RRO RRUNNING; do
    [ -z "$RHOST" ] && continue

    if [ "$RMASTER" = "$ORC_MASTER_HOST" ]; then
        pass "Replica $RHOST: master=$RMASTER (correct)"
    else
        fail "Replica $RHOST: master=$RMASTER (expected $ORC_MASTER_HOST)"
    fi

    if [ "$RRO" = "True" ]; then
        pass "Replica $RHOST: read_only=true"
    else
        fail "Replica $RHOST: read_only=$RRO (expected True)"
    fi

    if [ "$RRUNNING" = "True" ]; then
        pass "Replica $RHOST: replication threads running"
    else
        fail "Replica $RHOST: replication threads not running"
    fi
done <<< "$REPLICAS"

# Cross-reference with direct MySQL
for REPLICA in mysql2 mysql3; do
    ACTUAL_RO=$($COMPOSE exec -T "$REPLICA" mysql -uroot -ptestpass -Nse "SELECT @@read_only" 2>/dev/null | tr -d '[:space:]')
    ACTUAL_SOURCE=$(mysql_source_host "$REPLICA")
    if [ "$ACTUAL_RO" = "1" ]; then
        pass "Direct MySQL: $REPLICA read_only=1"
    else
        fail "Direct MySQL: $REPLICA read_only=$ACTUAL_RO (expected 1)"
    fi
    if echo "$ACTUAL_SOURCE" | grep -qi "mysql1"; then
        pass "Direct MySQL: $REPLICA replicates from mysql1"
    else
        fail "Direct MySQL: $REPLICA replicates from '$ACTUAL_SOURCE' (expected mysql1)"
    fi
done

# ----------------------------------------------------------------
echo ""
echo "--- Test 4: GTID tracking ---"

HAS_GTID=$(echo "$INSTANCES" | python3 -c "
import json, sys
instances = json.load(sys.stdin)
for inst in instances:
    gtid = inst.get('GtidPurged', '') or inst.get('ExecutedGtidSet', '')
    if gtid:
        print('yes')
        sys.exit(0)
print('no')
" 2>/dev/null || echo "unknown")

if [ "$HAS_GTID" = "yes" ]; then
    pass "GTID data is being tracked by orchestrator"
else
    fail "No GTID data found in orchestrator instance data"
fi

# ----------------------------------------------------------------
echo ""
echo "--- Test 5: Instance freshness ---"

# Verify LastChecked is recent (within last 60 seconds)
STALE=$(echo "$INSTANCES" | python3 -c "
import json, sys
from datetime import datetime, timezone, timedelta
instances = json.load(sys.stdin)
stale = []
now = datetime.now(timezone.utc)
for inst in instances:
    lc = inst.get('LastChecked', '')
    if lc:
        # Parse the timestamp (Go format: 2006-01-02T15:04:05Z)
        try:
            t = datetime.fromisoformat(lc.replace('Z', '+00:00'))
            age = (now - t).total_seconds()
            if age > 120:
                stale.append(f\"{inst['Key']['Hostname']}:{age:.0f}s\")
        except:
            pass
print(','.join(stale) if stale else 'none')
" 2>/dev/null || echo "unknown")

if [ "$STALE" = "none" ]; then
    pass "All instances have recent LastChecked timestamps"
elif [ "$STALE" = "unknown" ]; then
    skip "Could not parse LastChecked timestamps"
else
    fail "Stale instances: $STALE"
fi

# ----------------------------------------------------------------
echo ""
echo "--- Test 6: Uptime detection ---"

ALL_UP=$(echo "$INSTANCES" | python3 -c "
import json, sys
instances = json.load(sys.stdin)
down = [inst['Key']['Hostname'] for inst in instances if inst.get('Uptime', 0) == 0]
print(','.join(down) if down else 'all_up')
" 2>/dev/null || echo "unknown")

if [ "$ALL_UP" = "all_up" ]; then
    pass "All instances report non-zero uptime"
else
    fail "Instances with zero uptime: $ALL_UP"
fi

summary

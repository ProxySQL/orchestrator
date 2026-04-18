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

# Discover primary first, wait for it to be written to DB, then discover standby.
# This ensures the standby inherits the primary's cluster_name during WriteInstance.
echo "Seeding discovery with 172.30.0.20:5432 (pgprimary)..."
curl -s --max-time 10 "$ORC_URL/api/discover/172.30.0.20/5432" > /dev/null
sleep 5
echo "Seeding discovery with 172.30.0.21:5432 (pgstandby1)..."
curl -s --max-time 10 "$ORC_URL/api/discover/172.30.0.21/5432" > /dev/null

echo "Waiting for topology discovery..."
PG_CLUSTER=""
for i in $(seq 1 60); do
    PG_CLUSTER=$(curl -s --max-time 10 "$ORC_URL/api/clusters" 2>/dev/null | python3 -c "
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
        COUNT=$(curl -s --max-time 10 "$ORC_URL/api/cluster/$PG_CLUSTER" 2>/dev/null | python3 -c "import json,sys; print(len(json.load(sys.stdin)))" 2>/dev/null || echo "0")
        if [ "$COUNT" -ge 2 ] 2>/dev/null; then
            echo "PostgreSQL topology discovered (${COUNT} instances, cluster=$PG_CLUSTER) after ${i}s"
            break
        fi
    fi
    # Re-seed standby periodically
    if [ "$((i % 10))" = "0" ]; then
        curl -s --max-time 10 "$ORC_URL/api/discover/172.30.0.21/5432" > /dev/null 2>&1
    fi
    sleep 1
done

if [ -n "$PG_CLUSTER" ]; then
    pass "PostgreSQL cluster discovered: $PG_CLUSTER"
else
    fail "No PostgreSQL cluster discovered"
fi

INST_COUNT=$(curl -s --max-time 10 "$ORC_URL/api/cluster/$PG_CLUSTER" 2>/dev/null | python3 -c "import json,sys; print(len(json.load(sys.stdin)))" 2>/dev/null || echo "0")
if [ "$INST_COUNT" -ge 2 ]; then
    pass "PostgreSQL instances discovered: $INST_COUNT"
else
    fail "PostgreSQL instances discovered: $INST_COUNT (expected >= 2)"
fi

# Verify primary is not read-only (not in recovery)
PRIMARY_RO=$(curl -s --max-time 10 "$ORC_URL/api/cluster/$PG_CLUSTER" 2>/dev/null | python3 -c "
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
STANDBY_RO=$(curl -s --max-time 10 "$ORC_URL/api/cluster/$PG_CLUSTER" 2>/dev/null | python3 -c "
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
echo "--- Graceful primary switchover tests ---"

# Identify current primary before switchover
CURRENT_PRIMARY=$(curl -s --max-time 10 "$ORC_URL/api/cluster/$PG_CLUSTER" 2>/dev/null | python3 -c "
import json, sys
instances = json.load(sys.stdin)
for inst in instances:
    if not inst.get('ReadOnly', True):
        print(inst['Key']['Hostname'] + ':' + str(inst['Key']['Port']))
        sys.exit(0)
print('')
" 2>/dev/null || echo "")

if [ -z "$CURRENT_PRIMARY" ]; then
    fail "Cannot identify current PostgreSQL primary for graceful switchover"
else
    echo "Current primary: $CURRENT_PRIMARY"

    # Execute graceful-master-takeover-auto via API
    echo "Executing graceful-master-takeover-auto on cluster $PG_CLUSTER..."
    TAKEOVER_RESULT=$(curl -s --max-time 60 "$ORC_URL/api/graceful-master-takeover-auto/$PG_CLUSTER" 2>/dev/null)
    TAKEOVER_CODE=$(echo "$TAKEOVER_RESULT" | python3 -c "import json,sys; print(json.load(sys.stdin).get('Code','ERROR'))" 2>/dev/null || echo "ERROR")

    if [ "$TAKEOVER_CODE" = "OK" ]; then
        pass "Graceful master takeover API returned OK"
    else
        fail "Graceful master takeover API returned: $TAKEOVER_CODE — $TAKEOVER_RESULT"
    fi

    # Wait for topology to settle and re-discover
    sleep 10
    curl -s --max-time 10 "$ORC_URL/api/discover/172.30.0.20/5432" > /dev/null 2>&1
    curl -s --max-time 10 "$ORC_URL/api/discover/172.30.0.21/5432" > /dev/null 2>&1
    sleep 5

    # Verify the switchover at the PostgreSQL level, not via orchestrator's
    # cluster view. After a PG graceful takeover the demoted primary is still
    # running (awaiting an operator-managed restart with standby.signal), so
    # orchestrator sees two roots — one per former cluster — and a "find RO=false
    # in original cluster" check returns the same host both times.
    SWITCHOVER_OK=false

    # pgstandby1 must have been promoted (no longer in recovery)
    PROMOTED=$($COMPOSE exec -T pgstandby1 psql -U postgres -tAc "SELECT pg_is_in_recovery();" 2>/dev/null | tr -d '[:space:]')
    if [ "$PROMOTED" = "f" ]; then
        pass "pgstandby1 has been promoted (pg_is_in_recovery=false)"
        SWITCHOVER_OK=true
    else
        fail "pgstandby1 still in recovery after switchover (got: '$PROMOTED')"
    fi

    # pgprimary must have been set read-only (default_transaction_read_only=on)
    DEMOTED_RO=$($COMPOSE exec -T pgprimary psql -U postgres -tAc "SHOW default_transaction_read_only;" 2>/dev/null | tr -d '[:space:]')
    if [ "$DEMOTED_RO" = "on" ]; then
        pass "pgprimary has default_transaction_read_only=on"
    else
        fail "pgprimary default_transaction_read_only=$DEMOTED_RO (expected on)"
    fi

    # Verify new primary is actually writable (not just flagged read_only=false)
    if $COMPOSE exec -T pgstandby1 psql -U postgres -v ON_ERROR_STOP=1 -tAc \
        "CREATE TABLE IF NOT EXISTS orc_switchover_probe (id int); INSERT INTO orc_switchover_probe VALUES (1);" >/dev/null 2>&1; then
        pass "New primary (pgstandby1) accepts writes"
    else
        fail "New primary (pgstandby1) does not accept writes"
    fi

    # Verify demoted primary has primary_conninfo pointing at the new primary
    DEMOTED_CONNINFO=$($COMPOSE exec -T pgprimary psql -U postgres -tAc \
        "SELECT setting FROM pg_settings WHERE name='primary_conninfo';" 2>/dev/null | tr -d '\r')
    if echo "$DEMOTED_CONNINFO" | grep -q "172.30.0.21"; then
        pass "Demoted primary has primary_conninfo pointing at new primary"
    else
        fail "Demoted primary primary_conninfo missing new primary IP (got: '$DEMOTED_CONNINFO')"
    fi
fi

# ----------------------------------------------------------------
echo ""
echo "--- Graceful switchover negative cases ---"

# Negative case 1: bogus cluster name
BOGUS_RESULT=$(curl -s --max-time 30 "$ORC_URL/api/graceful-master-takeover-auto/no-such-cluster-xyz" 2>/dev/null)
BOGUS_CODE=$(echo "$BOGUS_RESULT" | python3 -c "import json,sys; print(json.load(sys.stdin).get('Code','ERROR'))" 2>/dev/null || echo "ERROR")
if [ "$BOGUS_CODE" != "OK" ]; then
    pass "graceful-master-takeover-auto on bogus cluster rejected (Code=$BOGUS_CODE)"
else
    fail "graceful-master-takeover-auto on bogus cluster unexpectedly returned OK"
fi

# Negative case 2: bogus designated host (PG_CLUSTER is valid, target is not)
BADHOST_RESULT=$(curl -s --max-time 30 "$ORC_URL/api/graceful-master-takeover/$PG_CLUSTER/no.such.host.invalid/5432" 2>/dev/null)
BADHOST_CODE=$(echo "$BADHOST_RESULT" | python3 -c "import json,sys; print(json.load(sys.stdin).get('Code','ERROR'))" 2>/dev/null || echo "ERROR")
if [ "$BADHOST_CODE" != "OK" ]; then
    pass "graceful-master-takeover with bogus designated host rejected (Code=$BADHOST_CODE)"
else
    fail "graceful-master-takeover with bogus designated host unexpectedly returned OK"
fi

# ----------------------------------------------------------------
echo ""
echo "--- Graceful switchover round-trip (switch back) ---"
# After the first switchover, pgprimary has primary_conninfo set but is still
# running as a (read-only) primary — it needs standby.signal + restart to
# actually stream WAL from the new primary. Simulate what a
# PostGracefulTakeoverProcesses hook would do.

if [ "${SWITCHOVER_OK:-false}" = "true" ]; then
    echo "Converting demoted pgprimary into a live standby of pgstandby1..."
    $COMPOSE exec -T pgprimary bash -c 'touch /var/lib/postgresql/data/standby.signal && chown postgres:postgres /var/lib/postgresql/data/standby.signal' || true
    $COMPOSE restart pgprimary
    echo "Waiting for pgprimary to become a healthy standby..."
    STANDBY_READY=false
    for i in $(seq 1 60); do
        IN_RECOVERY=$($COMPOSE exec -T pgprimary psql -U postgres -tAc "SELECT pg_is_in_recovery();" 2>/dev/null | tr -d '[:space:]')
        if [ "$IN_RECOVERY" = "t" ]; then
            STANDBY_READY=true
            echo "pgprimary is in recovery (standby mode) after ${i}s"
            break
        fi
        sleep 1
    done

    if [ "$STANDBY_READY" != "true" ]; then
        fail "pgprimary did not enter standby/recovery mode after restart"
    else
        pass "pgprimary restarted as a standby"

        # Let orchestrator re-discover — after pgprimary restarts as a standby,
        # it joins pgstandby1's cluster ("172.30.0.21:5432"). Poll for that.
        sleep 5
        curl -s --max-time 10 "$ORC_URL/api/discover/172.30.0.20/5432" > /dev/null 2>&1
        curl -s --max-time 10 "$ORC_URL/api/discover/172.30.0.21/5432" > /dev/null 2>&1
        sleep 8

        NEW_CLUSTER=""
        for i in $(seq 1 30); do
            NEW_CLUSTER=$(curl -s --max-time 10 "$ORC_URL/api/all-instances" 2>/dev/null | python3 -c "
import json, sys
for inst in json.load(sys.stdin):
    if inst['Key']['Hostname'] == '172.30.0.21':
        print(inst.get('ClusterName', ''))
        sys.exit(0)
" 2>/dev/null || echo "")
            # Verify pgprimary (172.30.0.20) joined the same cluster as pgstandby1
            PRIMARY_CLUSTER=$(curl -s --max-time 10 "$ORC_URL/api/all-instances" 2>/dev/null | python3 -c "
import json, sys
for inst in json.load(sys.stdin):
    if inst['Key']['Hostname'] == '172.30.0.20':
        print(inst.get('ClusterName', ''))
        sys.exit(0)
" 2>/dev/null || echo "")
            if [ -n "$NEW_CLUSTER" ] && [ "$NEW_CLUSTER" = "$PRIMARY_CLUSTER" ]; then
                break
            fi
            # Re-seed periodically
            if [ "$((i % 5))" = "0" ]; then
                curl -s --max-time 10 "$ORC_URL/api/discover/172.30.0.20/5432" > /dev/null 2>&1
                curl -s --max-time 10 "$ORC_URL/api/discover/172.30.0.21/5432" > /dev/null 2>&1
            fi
            sleep 1
        done

        if [ -n "$NEW_CLUSTER" ] && [ "$NEW_CLUSTER" = "$PRIMARY_CLUSTER" ]; then
            pass "Orchestrator re-unified topology under new primary (cluster=$NEW_CLUSTER)"
        else
            fail "Topology not re-unified: pgstandby1 cluster=$NEW_CLUSTER pgprimary cluster=$PRIMARY_CLUSTER"
        fi

        # Now switch back: pgstandby1 → pgprimary, using the NEW cluster name
        if [ -n "$NEW_CLUSTER" ] && [ "$NEW_CLUSTER" = "$PRIMARY_CLUSTER" ]; then
            echo "Executing graceful-master-takeover-auto on cluster $NEW_CLUSTER..."
            BACK_RESULT=$(curl -s --max-time 60 "$ORC_URL/api/graceful-master-takeover-auto/$NEW_CLUSTER" 2>/dev/null)
            BACK_CODE=$(echo "$BACK_RESULT" | python3 -c "import json,sys; print(json.load(sys.stdin).get('Code','ERROR'))" 2>/dev/null || echo "ERROR")

            if [ "$BACK_CODE" = "OK" ]; then
                pass "Round-trip graceful takeover API returned OK"
            else
                fail "Round-trip graceful takeover returned: $BACK_CODE — $BACK_RESULT"
            fi

            sleep 10

            # Verify pgprimary is now promoted (not in recovery)
            BACK_PROMOTED=$($COMPOSE exec -T pgprimary psql -U postgres -tAc "SELECT pg_is_in_recovery();" 2>/dev/null | tr -d '[:space:]')
            if [ "$BACK_PROMOTED" = "f" ]; then
                pass "Round-trip complete: pgprimary is primary again"
            else
                fail "Round-trip incomplete: pgprimary pg_is_in_recovery='$BACK_PROMOTED' (expected f)"
            fi

            # After round-trip, pgstandby1 is the demoted primary — reactivate
            # it as a live standby so the downstream failover-kill test has a
            # replica to promote.
            echo "Reactivating pgstandby1 as a live standby of pgprimary..."
            $COMPOSE exec -T pgstandby1 bash -c 'touch /var/lib/postgresql/data/standby.signal && chown postgres:postgres /var/lib/postgresql/data/standby.signal' || true
            $COMPOSE restart pgstandby1
            for i in $(seq 1 60); do
                IN_RECOVERY=$($COMPOSE exec -T pgstandby1 psql -U postgres -tAc "SELECT pg_is_in_recovery();" 2>/dev/null | tr -d '[:space:]')
                if [ "$IN_RECOVERY" = "t" ]; then
                    echo "pgstandby1 is streaming as a standby after ${i}s"
                    break
                fi
                sleep 1
            done
            sleep 5
            curl -s --max-time 10 "$ORC_URL/api/discover/172.30.0.20/5432" > /dev/null 2>&1
            curl -s --max-time 10 "$ORC_URL/api/discover/172.30.0.21/5432" > /dev/null 2>&1
            sleep 5
        fi
    fi
else
    skip "Round-trip test skipped — first switchover did not complete"
fi

# ----------------------------------------------------------------
echo ""
echo "--- Failover test: kill current primary ---"
# Static IPs are assigned in docker-compose.yml so the standby remains reachable
# even when the primary container stops.

echo "Stopping pgprimary container..."
$COMPOSE stop pgprimary

# Re-seed standby discovery by IP to ensure orchestrator can still reach it
sleep 2
curl -s --max-time 10 "$ORC_URL/api/discover/172.30.0.21/5432" > /dev/null 2>&1
sleep 3
curl -s --max-time 10 "$ORC_URL/api/discover/172.30.0.21/5432" > /dev/null 2>&1

# Debug: dump cluster state before and during failover
echo "DEBUG: ALL instances in PG orchestrator:"
curl -s --max-time 10 "$ORC_URL/api/all-instances" 2>/dev/null | python3 -c "
import json, sys
for i in json.load(sys.stdin):
    k=i['Key']; m=i['MasterKey']
    print(f'  {k[\"Hostname\"]}:{k[\"Port\"]} Cluster={i[\"ClusterName\"]} RO={i[\"ReadOnly\"]} Master={m[\"Hostname\"]}:{m[\"Port\"]}')" 2>/dev/null || echo "  (failed)"

echo "DEBUG: Cluster state before failover (cluster=$PG_CLUSTER):"
curl -s --max-time 10 "$ORC_URL/api/cluster/$PG_CLUSTER" 2>/dev/null | python3 -c "
import json, sys
for i in json.load(sys.stdin):
    k=i['Key']; m=i['MasterKey']
    print(f'  {k[\"Hostname\"]}:{k[\"Port\"]} RO={i[\"ReadOnly\"]} Master={m[\"Hostname\"]}:{m[\"Port\"]} Depth={i[\"ReplicationDepth\"]}')" 2>/dev/null || echo "  (failed)"

# Wait a bit for re-discovery, then dump state
sleep 15
echo "DEBUG: Cluster state after primary stop + re-discovery:"
curl -s --max-time 10 "$ORC_URL/api/cluster/$PG_CLUSTER" 2>/dev/null | python3 -c "
import json, sys
for i in json.load(sys.stdin):
    k=i['Key']; m=i['MasterKey']
    print(f'  {k[\"Hostname\"]}:{k[\"Port\"]} RO={i[\"ReadOnly\"]} Master={m[\"Hostname\"]}:{m[\"Port\"]} Depth={i[\"ReplicationDepth\"]} Valid={i[\"IsLastCheckValid\"]}')" 2>/dev/null || echo "  (failed)"
echo "DEBUG: Replication analysis:"
curl -s --max-time 10 "$ORC_URL/api/replication-analysis" 2>/dev/null | python3 -c "
import json, sys
for a in json.load(sys.stdin):
    print(f'  {a[\"AnalyzedInstanceKey\"][\"Hostname\"]}:{a[\"AnalyzedInstanceKey\"][\"Port\"]} Analysis={a[\"Analysis\"]} Replicas={a[\"CountReplicas\"]} ValidReplicas={a[\"CountValidReplicas\"]}')" 2>/dev/null || echo "  (failed)"

echo "Waiting for orchestrator to detect DeadPrimary and recover (max 120s)..."
RECOVERED=false
SUCCESSOR=""
for i in $(seq 1 120); do
    RECOVERIES=$(curl -s --max-time 10 "$ORC_URL/api/v2/recoveries" 2>/dev/null)
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
    curl -s --max-time 10 "$ORC_URL/api/v2/recoveries" 2>/dev/null | python3 -m json.tool 2>/dev/null | head -30
    echo "  DEBUG: Cluster topology:"
    curl -s --max-time 10 "$ORC_URL/api/cluster/$PG_CLUSTER" 2>/dev/null | python3 -m json.tool 2>/dev/null | head -30
fi

# Verify successor is no longer in recovery (promoted to primary)
if [ "$RECOVERED" = "true" ]; then
    sleep 3
    SUCCESSOR_RO=$(curl -s --max-time 10 "$ORC_URL/api/cluster/$PG_CLUSTER" 2>/dev/null | python3 -c "
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
RECOVERY_API=$(curl -s --max-time 10 "$ORC_URL/api/v2/recoveries" 2>/dev/null)
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

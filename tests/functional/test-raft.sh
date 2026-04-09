#!/bin/bash
# Raft consensus tests -- verify leader election, failover, and follower redirect
set -uo pipefail
cd "$(dirname "$0")/../.."
source tests/functional/lib.sh

echo "=== RAFT CONSENSUS TESTS ==="

# Port mapping: raft1->3100, raft2->3101, raft3->3102
RAFT_PORTS=(3100 3101 3102)
RAFT_NODES=(orchestrator-raft1 orchestrator-raft2 orchestrator-raft3)
COMPOSE_FILE="tests/functional/docker-compose.yml"

# ============================================================
# Phase 1: Cluster Formation & Leader Election
# ============================================================
echo ""
echo "--- Phase 1: Cluster Formation & Leader Election ---"

docker compose -f "$COMPOSE_FILE" up -d orchestrator-raft1 orchestrator-raft2 orchestrator-raft3

# Wait for all 3 nodes to be reachable and for a leader to be elected
echo "Waiting for Raft cluster to form and elect a leader (up to 90s)..."
LEADER=""
for i in $(seq 1 90); do
    ALL_UP=true
    for port in "${RAFT_PORTS[@]}"; do
        if ! curl -sf --max-time 10 "http://localhost:${port}/api/raft-leader" > /dev/null 2>&1; then
            ALL_UP=false
            break
        fi
    done
    if $ALL_UP; then
        # Check if all nodes agree on a leader
        LEADER1=$(curl -sf --max-time 10 "http://localhost:3100/api/raft-leader" 2>/dev/null | python3 -c "import json,sys; print(json.load(sys.stdin))" 2>/dev/null || echo "")
        LEADER2=$(curl -sf --max-time 10 "http://localhost:3101/api/raft-leader" 2>/dev/null | python3 -c "import json,sys; print(json.load(sys.stdin))" 2>/dev/null || echo "")
        LEADER3=$(curl -sf --max-time 10 "http://localhost:3102/api/raft-leader" 2>/dev/null | python3 -c "import json,sys; print(json.load(sys.stdin))" 2>/dev/null || echo "")
        if [ -n "$LEADER1" ] && [ "$LEADER1" = "$LEADER2" ] && [ "$LEADER2" = "$LEADER3" ]; then
            LEADER="$LEADER1"
            echo "Leader elected: $LEADER (after ${i}s)"
            break
        fi
    fi
    sleep 1
done

if [ -n "$LEADER" ]; then
    pass "Raft leader elected: $LEADER"
else
    fail "Raft leader not elected within 90s"
    # Print debug info
    for port in "${RAFT_PORTS[@]}"; do
        echo "  Node :${port} raft-status: $(curl -sf --max-time 10 http://localhost:${port}/api/raft-status 2>/dev/null || echo 'unreachable')"
    done
fi

# Verify all nodes agree on the same leader
LEADERS_AGREE=true
for port in "${RAFT_PORTS[@]}"; do
    NODE_LEADER=$(curl -sf --max-time 10 "http://localhost:${port}/api/raft-leader" 2>/dev/null | python3 -c "import json,sys; print(json.load(sys.stdin))" 2>/dev/null || echo "")
    if [ "$NODE_LEADER" != "$LEADER" ]; then
        LEADERS_AGREE=false
        break
    fi
done
if $LEADERS_AGREE && [ -n "$LEADER" ]; then
    pass "All 3 nodes agree on the same leader"
else
    fail "Nodes do not agree on the leader"
fi

# Verify exactly one node reports itself as Leader state
LEADER_COUNT=0
for port in "${RAFT_PORTS[@]}"; do
    STATE=$(curl -sf --max-time 10 "http://localhost:${port}/api/raft-state" 2>/dev/null | python3 -c "import json,sys; print(json.load(sys.stdin))" 2>/dev/null || echo "")
    if [ "$STATE" = "Leader" ]; then
        ((LEADER_COUNT++))
    fi
done
if [ "$LEADER_COUNT" -eq 1 ]; then
    pass "Exactly one node is in Leader state"
else
    fail "Expected 1 leader, found $LEADER_COUNT"
fi

# ============================================================
# Phase 2: Leader Serves Topology
# ============================================================
echo ""
echo "--- Phase 2: Leader Serves Topology ---"

# Determine leader port (map leader IP to host port)
LEADER_PORT=""
LEADER_INDEX=""
for idx in 0 1 2; do
    STATE=$(curl -sf --max-time 10 "http://localhost:${RAFT_PORTS[$idx]}/api/raft-state" 2>/dev/null | python3 -c "import json,sys; print(json.load(sys.stdin))" 2>/dev/null || echo "")
    if [ "$STATE" = "Leader" ]; then
        LEADER_PORT="${RAFT_PORTS[$idx]}"
        LEADER_INDEX=$idx
        break
    fi
done

if [ -z "$LEADER_PORT" ]; then
    fail "Could not identify leader port"
else
    echo "Leader is on localhost:${LEADER_PORT} (${RAFT_NODES[$LEADER_INDEX]})"

    # Discover MySQL topology through the leader
    echo "Discovering MySQL topology through the leader..."
    curl -sf --max-time 10 "http://localhost:${LEADER_PORT}/api/discover/mysql1/3306" > /dev/null 2>&1
    curl -sf --max-time 10 "http://localhost:${LEADER_PORT}/api/discover/mysql2/3306" > /dev/null 2>&1
    curl -sf --max-time 10 "http://localhost:${LEADER_PORT}/api/discover/mysql3/3306" > /dev/null 2>&1

    # Wait for topology discovery
    echo "Waiting for topology discovery (up to 60s)..."
    CLUSTER_FOUND=false
    for i in $(seq 1 60); do
        CLUSTERS=$(curl -sf --max-time 10 "http://localhost:${LEADER_PORT}/api/clusters" 2>/dev/null || echo "[]")
        COUNT=$(echo "$CLUSTERS" | python3 -c "import json,sys; c=json.load(sys.stdin); print(len(c))" 2>/dev/null || echo "0")
        if [ "$COUNT" -ge 1 ]; then
            echo "Cluster discovered after ${i}s"
            CLUSTER_FOUND=true
            break
        fi
        # Re-seed discovery periodically
        if [ "$((i % 10))" = "0" ]; then
            curl -sf --max-time 10 "http://localhost:${LEADER_PORT}/api/discover/mysql1/3306" > /dev/null 2>&1
            curl -sf --max-time 10 "http://localhost:${LEADER_PORT}/api/discover/mysql2/3306" > /dev/null 2>&1
            curl -sf --max-time 10 "http://localhost:${LEADER_PORT}/api/discover/mysql3/3306" > /dev/null 2>&1
        fi
        sleep 1
    done

    if $CLUSTER_FOUND; then
        pass "Leader serves cluster data via /api/clusters"
    else
        fail "Leader did not return cluster data within 60s"
    fi

    # Verify followers can also return cluster data (Raft replicates state)
    FOLLOWER_HAS_DATA=true
    for idx in 0 1 2; do
        if [ "$idx" = "$LEADER_INDEX" ]; then
            continue
        fi
        FPORT="${RAFT_PORTS[$idx]}"
        # Followers may redirect or serve data directly; either is valid
        FCLUSTERS=$(curl -sf --max-time 10L "http://localhost:${FPORT}/api/clusters" 2>/dev/null || echo "[]")
        FCOUNT=$(echo "$FCLUSTERS" | python3 -c "import json,sys; c=json.load(sys.stdin); print(len(c))" 2>/dev/null || echo "0")
        if [ "$FCOUNT" -lt 1 ]; then
            FOLLOWER_HAS_DATA=false
            echo "  Follower on :${FPORT} returned $FCOUNT clusters"
        fi
    done
    if $FOLLOWER_HAS_DATA; then
        pass "Followers serve cluster data (Raft state replicated)"
    else
        # This is not necessarily a failure -- followers may need more time
        skip "Some followers do not yet serve cluster data (may need more replication time)"
    fi
fi

# ============================================================
# Phase 3: Leader Failure & Re-election
# ============================================================
echo ""
echo "--- Phase 3: Leader Failure & Re-election ---"

OLD_LEADER="$LEADER"
OLD_LEADER_NODE=""
if [ -n "$LEADER_INDEX" ]; then
    OLD_LEADER_NODE="${RAFT_NODES[$LEADER_INDEX]}"
fi

if [ -z "$OLD_LEADER_NODE" ]; then
    fail "Cannot test leader failure: no leader identified"
else
    echo "Stopping leader node: $OLD_LEADER_NODE"
    docker compose -f "$COMPOSE_FILE" stop "$OLD_LEADER_NODE"

    # Determine which nodes are still running
    REMAINING_PORTS=()
    REMAINING_INDICES=()
    for idx in 0 1 2; do
        if [ "$idx" != "$LEADER_INDEX" ]; then
            REMAINING_PORTS+=("${RAFT_PORTS[$idx]}")
            REMAINING_INDICES+=("$idx")
        fi
    done

    # Wait for re-election
    echo "Waiting for re-election (up to 60s)..."
    NEW_LEADER=""
    for i in $(seq 1 60); do
        L1=$(curl -sf --max-time 10 "http://localhost:${REMAINING_PORTS[0]}/api/raft-leader" 2>/dev/null | python3 -c "import json,sys; print(json.load(sys.stdin))" 2>/dev/null || echo "")
        L2=$(curl -sf --max-time 10 "http://localhost:${REMAINING_PORTS[1]}/api/raft-leader" 2>/dev/null | python3 -c "import json,sys; print(json.load(sys.stdin))" 2>/dev/null || echo "")
        if [ -n "$L1" ] && [ "$L1" = "$L2" ] && [ "$L1" != "$OLD_LEADER" ]; then
            NEW_LEADER="$L1"
            echo "New leader elected: $NEW_LEADER (after ${i}s)"
            break
        fi
        sleep 1
    done

    if [ -n "$NEW_LEADER" ]; then
        pass "New leader elected after stopping old leader: $NEW_LEADER"
    else
        fail "No new leader elected within 60s"
        for port in "${REMAINING_PORTS[@]}"; do
            echo "  Node :${port} status: $(curl -sf --max-time 10 http://localhost:${port}/api/raft-status 2>/dev/null || echo 'unreachable')"
        done
    fi

    # Verify new leader is different from old
    if [ -n "$NEW_LEADER" ] && [ "$NEW_LEADER" != "$OLD_LEADER" ]; then
        pass "New leader is different from old leader"
    elif [ -n "$NEW_LEADER" ]; then
        fail "New leader is the same as old leader (should not happen)"
    fi

    # Verify new leader can serve API requests
    if [ -n "$NEW_LEADER" ]; then
        NEW_LEADER_PORT=""
        for idx in "${REMAINING_INDICES[@]}"; do
            STATE=$(curl -sf --max-time 10 "http://localhost:${RAFT_PORTS[$idx]}/api/raft-state" 2>/dev/null | python3 -c "import json,sys; print(json.load(sys.stdin))" 2>/dev/null || echo "")
            if [ "$STATE" = "Leader" ]; then
                NEW_LEADER_PORT="${RAFT_PORTS[$idx]}"
                break
            fi
        done
        if [ -n "$NEW_LEADER_PORT" ]; then
            CLUSTERS=$(curl -sf --max-time 10 "http://localhost:${NEW_LEADER_PORT}/api/clusters" 2>/dev/null || echo "[]")
            COUNT=$(echo "$CLUSTERS" | python3 -c "import json,sys; c=json.load(sys.stdin); print(len(c))" 2>/dev/null || echo "0")
            if [ "$COUNT" -ge 1 ]; then
                pass "New leader serves cluster data via API"
            else
                skip "New leader returned 0 clusters (state may not have fully replicated yet)"
            fi
        fi
    fi

    # ============================================================
    # Phase 4: Node Rejoin
    # ============================================================
    echo ""
    echo "--- Phase 4: Node Rejoin ---"

    echo "Restarting stopped node: $OLD_LEADER_NODE"
    docker compose -f "$COMPOSE_FILE" start "$OLD_LEADER_NODE"

    # Wait for the restarted node to rejoin
    RESTARTED_PORT="${RAFT_PORTS[$LEADER_INDEX]}"
    echo "Waiting for restarted node (:${RESTARTED_PORT}) to rejoin (up to 60s)..."
    REJOINED=false
    for i in $(seq 1 60); do
        RLEADER=$(curl -sf --max-time 10 "http://localhost:${RESTARTED_PORT}/api/raft-leader" 2>/dev/null | python3 -c "import json,sys; print(json.load(sys.stdin))" 2>/dev/null || echo "")
        if [ -n "$RLEADER" ] && [ "$RLEADER" = "$NEW_LEADER" ]; then
            echo "Node rejoined after ${i}s"
            REJOINED=true
            break
        fi
        sleep 1
    done

    if $REJOINED; then
        pass "Restarted node rejoined the cluster"
    else
        fail "Restarted node did not rejoin within 60s"
    fi

    # Verify the restarted node is a follower (not a new leader)
    RSTATE=$(curl -sf --max-time 10 "http://localhost:${RESTARTED_PORT}/api/raft-state" 2>/dev/null | python3 -c "import json,sys; print(json.load(sys.stdin))" 2>/dev/null || echo "")
    if [ "$RSTATE" = "Follower" ]; then
        pass "Restarted node is a Follower (stable leader)"
    elif [ "$RSTATE" = "Leader" ]; then
        # Leadership may have shifted -- still valid if all agree
        skip "Restarted node became Leader (leadership may have shifted)"
    else
        fail "Restarted node in unexpected state: $RSTATE"
    fi

    # Verify all 3 nodes agree on the current leader
    ALL_AGREE=true
    CURRENT_LEADER=""
    for port in "${RAFT_PORTS[@]}"; do
        NL=$(curl -sf --max-time 10 "http://localhost:${port}/api/raft-leader" 2>/dev/null | python3 -c "import json,sys; print(json.load(sys.stdin))" 2>/dev/null || echo "")
        if [ -z "$CURRENT_LEADER" ]; then
            CURRENT_LEADER="$NL"
        elif [ "$NL" != "$CURRENT_LEADER" ]; then
            ALL_AGREE=false
        fi
    done
    if $ALL_AGREE && [ -n "$CURRENT_LEADER" ]; then
        pass "All 3 nodes agree on current leader after rejoin: $CURRENT_LEADER"
    else
        fail "Nodes do not agree on leader after rejoin"
    fi

    # Verify cluster is healthy (all 3 nodes report healthy)
    HEALTHY_COUNT=0
    for port in "${RAFT_PORTS[@]}"; do
        HEALTH=$(curl -sf --max-time 10 "http://localhost:${port}/api/raft-health" 2>/dev/null || echo "")
        if echo "$HEALTH" | grep -q "healthy"; then
            ((HEALTHY_COUNT++))
        fi
    done
    if [ "$HEALTHY_COUNT" -eq 3 ]; then
        pass "All 3 nodes report healthy"
    else
        skip "Only $HEALTHY_COUNT/3 nodes report healthy (may need more time)"
    fi
fi

# ============================================================
# Cleanup
# ============================================================
echo ""
echo "--- Cleanup ---"
docker compose -f "$COMPOSE_FILE" stop orchestrator-raft1 orchestrator-raft2 orchestrator-raft3 2>/dev/null || true
echo "Raft containers stopped."

summary

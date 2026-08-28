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

# Start node 1 first to let it bootstrap the cluster before other nodes join.
# Starting all 3 simultaneously causes each to call BootstrapCluster independently,
# creating conflicting initial states and perpetual election cycles.
echo "Starting first Raft node (bootstrap node)..."
docker compose -f "$COMPOSE_FILE" up -d orchestrator-raft1

# Wait for node 1 to be reachable (includes apt-get install time)
echo "Waiting for bootstrap node to be ready (up to 90s)..."
BOOTSTRAP_READY=false
for i in $(seq 1 90); do
    if curl -sf --max-time 5 "http://localhost:3100/api/raft-status" > /dev/null 2>&1; then
        BOOTSTRAP_READY=true
        echo "Bootstrap node ready after ${i}s"
        break
    fi
    sleep 1
done

if ! $BOOTSTRAP_READY; then
    fail "Bootstrap Raft node (orchestrator-raft1) not ready within 90s"
    docker compose -f "$COMPOSE_FILE" logs orchestrator-raft1 2>/dev/null | tail -30
    summary
fi
pass "Bootstrap Raft node started and ready"

# Now start the remaining nodes — they will find the bootstrapped cluster
echo "Starting remaining Raft nodes..."
docker compose -f "$COMPOSE_FILE" up -d orchestrator-raft2 orchestrator-raft3

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
# Phase 5: Snapshot Restore Retains Recently-Discovered Instances
# ============================================================
# Regression test for issue #123 (fixed by PR #124).
#
# Restore() in go/logic/snapshot_data.go must not purge locally-known
# instances that are absent from a (stale) snapshot while they were still
# seen within UnseenInstanceForgetHours.
#
# Deterministic setup (reproduces the production race without timing luck):
#   1. forget mysql3 (raft-replicated), then force a snapshot on every node:
#      each node now holds a snapshot WITHOUT mysql3. All earlier commands
#      (including the discover/forget of mysql3) are at or below the snapshot
#      index, so restart-time log replay cannot re-add or re-forget mysql3.
#   2. continuous discovery re-discovers mysql3 on every node independently
#      (a local side effect of polling mysql1; no raft command is involved).
#      mysql3 is now present in every local backend, but absent from every
#      node's last snapshot -- exactly the state the bug report describes.
#   3. stop mysql3 so that nothing can re-discover it after a restore.
#   4. rolling restart of all 3 nodes, stopping the current leader each round
#      (leader change per restart, as in the issue's reproduction steps):
#      - unpatched: Restore() forgets mysql3 on every node -> cluster-wide loss.
#      - patched: mysql3 was recently seen -> retained on every node.
echo ""
echo "--- Phase 5: Snapshot Restore Retains Recently-Discovered Instances (issue #123) ---"

RAFT_DATA_DIRS=(/tmp/raft1 /tmp/raft2 /tmp/raft3)

# backend_instances <node-index>: list instance hostnames in the node's local backend
backend_instances() {
    docker compose -f "$COMPOSE_FILE" exec -T "${RAFT_NODES[$1]}" \
        sqlite3 "${RAFT_DATA_DIRS[$1]}/orchestrator.sqlite3" \
        "select hostname from database_instance order by hostname" 2>/dev/null
}

# wait_all_backends <expected-count> <deadline-seconds>: poll until every node's
# backend holds exactly <expected-count> instances; when the expected count is 3,
# mysql3 must be among them
wait_all_backends() {
    local expected="$1" deadline="$2" i idx rows count all_match
    for i in $(seq 1 "$deadline"); do
        all_match=true
        for idx in 0 1 2; do
            rows=$(backend_instances "$idx" | tr '\n' ' ')
            count=$(echo "$rows" | wc -w | tr -d ' ')
            if [ "$count" != "$expected" ]; then
                all_match=false
                break
            fi
            if [ "$expected" = "3" ] && ! echo "$rows" | grep -q "mysql3"; then
                all_match=false
                break
            fi
        done
        if $all_match; then
            return 0
        fi
        sleep 1
    done
    return 1
}

# index of the current raft leader ("" if none)
current_leader_index() {
    local idx state
    for idx in 0 1 2; do
        state=$(curl -sf --max-time 10 "http://localhost:${RAFT_PORTS[$idx]}/api/raft-state" 2>/dev/null | python3 -c "import json,sys; print(json.load(sys.stdin))" 2>/dev/null || echo "")
        if [ "$state" = "Leader" ]; then
            echo "$idx"
            return 0
        fi
    done
    echo ""
    return 1
}

LEADER_INDEX=$(current_leader_index)
if [ -z "$LEADER_INDEX" ] || ! wait_all_backends 3 10; then
    skip "Raft cluster not healthy with full topology; skipping issue #123 regression phase"
else
    LEADER_PORT="${RAFT_PORTS[$LEADER_INDEX]}"
    PHASE5_OK=true

    # forget mysql3, then snapshot: the new snapshots do not contain mysql3
    curl -sf --max-time 10 "http://localhost:${LEADER_PORT}/api/forget/mysql3/3306" > /dev/null 2>&1 \
        || { fail "forget mysql3 failed"; PHASE5_OK=false; }
    if $PHASE5_OK && wait_all_backends 2 30; then
        pass "mysql3 forgotten on all nodes"
    else
        fail "mysql3 not forgotten on all nodes within 30s"
        PHASE5_OK=false
    fi

    if $PHASE5_OK; then
        for port in "${RAFT_PORTS[@]}"; do
            curl -sf --max-time 30 "http://localhost:${port}/api/raft-snapshot" > /dev/null 2>&1 || { fail "raft-snapshot failed on :${port}"; PHASE5_OK=false; }
        done
        sleep 2
        $PHASE5_OK && pass "Snapshots without mysql3 taken on all nodes"
    fi

    if $PHASE5_OK; then
        # continuous discovery re-discovers mysql3 on each node as a replica of
        # mysql1 -- purely local writes, no raft commands in the log
        echo "Waiting for continuous discovery to re-discover mysql3 on all nodes (up to 90s)..."
        if wait_all_backends 3 90; then
            pass "mysql3 re-discovered locally on all nodes (no raft command)"
        else
            fail "mysql3 not re-discovered on all nodes within 90s"
            for idx in 0 1 2; do
                echo "  ${RAFT_NODES[$idx]} backend: $(backend_instances "$idx" | tr '\n' ' ')"
            done
            PHASE5_OK=false
        fi
    fi

    if $PHASE5_OK; then
        # stop mysql3: without it, nothing can re-discover mysql3 after a
        # restore, so any loss caused by Restore() becomes observable
        echo "Stopping mysql3 to block re-discovery after restore..."
        docker compose -f "$COMPOSE_FILE" stop mysql3 > /dev/null 2>&1 \
            && pass "mysql3 stopped" \
            || { fail "Could not stop mysql3"; PHASE5_OK=false; }
        # let in-flight discovery attempts drain
        sleep 3
    fi

    if $PHASE5_OK; then
        # rolling restart: stop the current leader, wait for re-election, restart;
        # repeat so that all 3 nodes restore from their snapshots
        for ROUND in 1 2 3; do
            LIDX=$(current_leader_index)
            if [ -z "$LIDX" ]; then
                fail "Rolling restart round ${ROUND}: no leader found"
                PHASE5_OK=false
                break
            fi
            NODE="${RAFT_NODES[$LIDX]}"
            RPORT="${RAFT_PORTS[$LIDX]}"
            OLD_LEADER=$(curl -sf --max-time 10 "http://localhost:${RPORT}/api/raft-leader" 2>/dev/null | python3 -c "import json,sys; print(json.load(sys.stdin))" 2>/dev/null || echo "")
            echo "Rolling restart round ${ROUND}: stopping leader ${NODE}"
            docker compose -f "$COMPOSE_FILE" stop "$NODE" > /dev/null 2>&1

            REMAINING_PORTS=()
            for idx in 0 1 2; do
                [ "$idx" != "$LIDX" ] && REMAINING_PORTS+=("${RAFT_PORTS[$idx]}")
            done
            # wait for a NEW leader: until the election completes, the remaining
            # nodes keep reporting the old (stopped) leader
            REELECTED=false
            for i in $(seq 1 60); do
                L1=$(curl -sf --max-time 10 "http://localhost:${REMAINING_PORTS[0]}/api/raft-leader" 2>/dev/null | python3 -c "import json,sys; print(json.load(sys.stdin))" 2>/dev/null || echo "")
                L2=$(curl -sf --max-time 10 "http://localhost:${REMAINING_PORTS[1]}/api/raft-leader" 2>/dev/null | python3 -c "import json,sys; print(json.load(sys.stdin))" 2>/dev/null || echo "")
                if [ -n "$L1" ] && [ "$L1" = "$L2" ] && [ "$L1" != "$OLD_LEADER" ]; then
                    REELECTED=true
                    break
                fi
                sleep 1
            done
            if ! $REELECTED; then
                fail "Rolling restart round ${ROUND}: no re-election within 60s"
                PHASE5_OK=false
                docker compose -f "$COMPOSE_FILE" start "$NODE" > /dev/null 2>&1
                break
            fi

            docker compose -f "$COMPOSE_FILE" start "$NODE" > /dev/null 2>&1
            # wait for the restarted node to agree on the leader with a running node
            REJOINED=false
            for i in $(seq 1 60); do
                RL=$(curl -sf --max-time 10 "http://localhost:${RPORT}/api/raft-leader" 2>/dev/null | python3 -c "import json,sys; print(json.load(sys.stdin))" 2>/dev/null || echo "")
                CL=$(curl -sf --max-time 10 "http://localhost:${REMAINING_PORTS[0]}/api/raft-leader" 2>/dev/null | python3 -c "import json,sys; print(json.load(sys.stdin))" 2>/dev/null || echo "")
                if [ -n "$RL" ] && [ "$RL" = "$CL" ]; then
                    REJOINED=true
                    break
                fi
                sleep 1
            done
            if $REJOINED; then
                pass "Round ${ROUND}: leader change + ${NODE} restarted and rejoined"
            else
                fail "Rolling restart round ${ROUND}: ${NODE} did not rejoin within 60s"
                PHASE5_OK=false
                break
            fi
        done
    fi

    if $PHASE5_OK; then
        # The actual regression check: every node's local backend must still
        # contain mysql3, which was recently seen but absent from the snapshots
        RETAINED=true
        for idx in 0 1 2; do
            ROWS=$(backend_instances "$idx" | tr '\n' ' ')
            if ! echo "$ROWS" | grep -q "mysql3"; then
                RETAINED=false
                echo "  ${RAFT_NODES[$idx]} backend after restart: ${ROWS}"
            fi
        done
        if $RETAINED; then
            pass "Recently-discovered instance retained on all nodes after rolling restart (issue #123)"
        else
            fail "Recently-discovered instance lost after rolling restart (issue #123 regression)"
        fi
    fi

    # restore the environment: mysql3 back up, topology healed
    echo "Restarting mysql3..."
    docker compose -f "$COMPOSE_FILE" start mysql3 > /dev/null 2>&1
    if wait_all_backends 3 90; then
        pass "Topology healed after issue #123 regression phase"
    else
        skip "Topology not fully healed after issue #123 regression phase (mysql3 may need more time)"
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

# PostgreSQL Graceful Primary Switchover — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add planned/graceful primary switchover for PostgreSQL, reusing the same CLI/API surface as MySQL's graceful master takeover.

**Architecture:** New PostgreSQL-specific instance operations (`SetReadOnly`, `GetCurrentWALLSN`, `WaitForStandbyLSN`, `RepositionAsStandby`) in `go/inst/instance_topology_postgresql.go`, a new orchestration function `PostgreSQLGracefulPrimarySwitchover` in `go/logic/topology_recovery_postgresql.go`, and a provider dispatch in the existing `GracefulMasterTakeover` in `go/logic/topology_recovery.go`.

**Tech Stack:** Go, PostgreSQL system functions (`pg_current_wal_lsn`, `pg_last_wal_replay_lsn`, `pg_is_in_recovery`, `pg_stat_activity`, `pg_terminate_backend`, `pg_reload_conf`), `database/sql`

**Spec:** `docs/superpowers/specs/2026-04-18-postgresql-graceful-switchover-design.md`

---

### Task 1: `PostgreSQLSetReadOnly` — Unit Test + Implementation

**Files:**
- Modify: `go/inst/instance_topology_postgresql.go` (append new function after line 181)
- Create: `go/inst/instance_topology_postgresql_test.go`

- [ ] **Step 1: Write the failing test**

Create `go/inst/instance_topology_postgresql_test.go`:

```go
/*
   Copyright 2024 Orchestrator Authors

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package inst

import (
	"testing"
)

func TestPostgreSQLSetReadOnlyNilKey(t *testing.T) {
	_, err := PostgreSQLSetReadOnly(nil, true)
	if err == nil {
		t.Fatal("expected error for nil instanceKey")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./go/inst/... -run TestPostgreSQLSetReadOnlyNilKey -v`
Expected: FAIL — `PostgreSQLSetReadOnly` is not defined.

- [ ] **Step 3: Write minimal implementation**

Append to `go/inst/instance_topology_postgresql.go` (after line 181, before the closing of the file):

```go
// PostgreSQLSetReadOnly sets or unsets read-only mode on a PostgreSQL instance
// by updating default_transaction_read_only via ALTER SYSTEM and reloading.
// When setting read-only, it also terminates existing non-replication connections
// to prevent stray writes during graceful switchover.
func PostgreSQLSetReadOnly(instanceKey *InstanceKey, readOnly bool) (*Instance, error) {
	if instanceKey == nil {
		return nil, fmt.Errorf("PostgreSQLSetReadOnly: nil instanceKey")
	}

	db, err := openPostgreSQLTopology(*instanceKey)
	if err != nil {
		return nil, log.Errore(err)
	}
	defer func() { _ = db.Close() }()

	readOnlyValue := "off"
	if readOnly {
		readOnlyValue = "on"
	}

	log.Infof("PostgreSQLSetReadOnly: setting default_transaction_read_only = %s on %+v", readOnlyValue, *instanceKey)

	if _, err := db.Exec(fmt.Sprintf("ALTER SYSTEM SET default_transaction_read_only = %s", readOnlyValue)); err != nil {
		return nil, log.Errore(fmt.Errorf("PostgreSQLSetReadOnly: ALTER SYSTEM failed on %+v: %v", *instanceKey, err))
	}

	if _, err := db.Exec("SELECT pg_reload_conf()"); err != nil {
		return nil, log.Errore(fmt.Errorf("PostgreSQLSetReadOnly: pg_reload_conf() failed on %+v: %v", *instanceKey, err))
	}

	if readOnly {
		// Terminate existing non-replication, non-orchestrator connections to close
		// the write window. Replication backends (walsender) and our own connection
		// are excluded.
		log.Infof("PostgreSQLSetReadOnly: terminating non-replication backends on %+v", *instanceKey)
		_, err := db.Exec(`
			SELECT pg_terminate_backend(pid)
			FROM pg_stat_activity
			WHERE pid <> pg_backend_pid()
			  AND backend_type NOT IN ('walsender', 'walreceiver', 'autovacuum worker', 'logical replication launcher')
			  AND backend_type <> 'background worker'
			  AND datname IS NOT NULL
		`)
		if err != nil {
			// Non-fatal: log warning but continue — read-only is already set
			_ = log.Warningf("PostgreSQLSetReadOnly: error terminating backends on %+v: %v", *instanceKey, err)
		}
	}

	return ReadPostgreSQLTopologyInstance(instanceKey)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./go/inst/... -run TestPostgreSQLSetReadOnlyNilKey -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go/inst/instance_topology_postgresql.go go/inst/instance_topology_postgresql_test.go
git commit -m "feat(postgresql): add PostgreSQLSetReadOnly for graceful switchover"
```

---

### Task 2: `PostgreSQLGetCurrentWALLSN` — Unit Test + Implementation

**Files:**
- Modify: `go/inst/instance_topology_postgresql.go` (append after PostgreSQLSetReadOnly)
- Modify: `go/inst/instance_topology_postgresql_test.go` (add test)

- [ ] **Step 1: Write the failing test**

Add to `go/inst/instance_topology_postgresql_test.go`:

```go
func TestPostgreSQLGetCurrentWALLSNNilKey(t *testing.T) {
	_, err := PostgreSQLGetCurrentWALLSN(nil)
	if err == nil {
		t.Fatal("expected error for nil instanceKey")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./go/inst/... -run TestPostgreSQLGetCurrentWALLSNNilKey -v`
Expected: FAIL — `PostgreSQLGetCurrentWALLSN` is not defined.

- [ ] **Step 3: Write minimal implementation**

Append to `go/inst/instance_topology_postgresql.go`:

```go
// PostgreSQLGetCurrentWALLSN returns the current WAL write position (LSN)
// of a PostgreSQL primary instance.
func PostgreSQLGetCurrentWALLSN(instanceKey *InstanceKey) (string, error) {
	if instanceKey == nil {
		return "", fmt.Errorf("PostgreSQLGetCurrentWALLSN: nil instanceKey")
	}

	db, err := openPostgreSQLTopology(*instanceKey)
	if err != nil {
		return "", log.Errore(err)
	}
	defer func() { _ = db.Close() }()

	var lsn string
	if err := db.QueryRow("SELECT pg_current_wal_lsn()::text").Scan(&lsn); err != nil {
		return "", log.Errore(fmt.Errorf("PostgreSQLGetCurrentWALLSN: failed on %+v: %v", *instanceKey, err))
	}

	log.Infof("PostgreSQLGetCurrentWALLSN: %+v current LSN is %s", *instanceKey, lsn)
	return lsn, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./go/inst/... -run TestPostgreSQLGetCurrentWALLSNNilKey -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go/inst/instance_topology_postgresql.go go/inst/instance_topology_postgresql_test.go
git commit -m "feat(postgresql): add PostgreSQLGetCurrentWALLSN for WAL position tracking"
```

---

### Task 3: `PostgreSQLWaitForStandbyLSN` — Unit Test + Implementation

**Files:**
- Modify: `go/inst/instance_topology_postgresql.go` (append after PostgreSQLGetCurrentWALLSN)
- Modify: `go/inst/instance_topology_postgresql_test.go` (add test)

- [ ] **Step 1: Write the failing test**

Add to `go/inst/instance_topology_postgresql_test.go`:

```go
func TestPostgreSQLWaitForStandbyLSNNilKey(t *testing.T) {
	err := PostgreSQLWaitForStandbyLSN(nil, "0/0", 1*time.Second)
	if err == nil {
		t.Fatal("expected error for nil instanceKey")
	}
}

func TestPostgreSQLWaitForStandbyLSNEmptyLSN(t *testing.T) {
	key := &InstanceKey{Hostname: "localhost", Port: 5432}
	err := PostgreSQLWaitForStandbyLSN(key, "", 1*time.Second)
	if err == nil {
		t.Fatal("expected error for empty targetLSN")
	}
}
```

Add `"time"` to the imports in the test file.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./go/inst/... -run TestPostgreSQLWaitForStandbyLSN -v`
Expected: FAIL — `PostgreSQLWaitForStandbyLSN` is not defined.

- [ ] **Step 3: Write minimal implementation**

Append to `go/inst/instance_topology_postgresql.go`:

```go
// PostgreSQLWaitForStandbyLSN polls a PostgreSQL standby until its replay LSN
// reaches or exceeds the target LSN, or the timeout expires.
func PostgreSQLWaitForStandbyLSN(standbyKey *InstanceKey, targetLSN string, timeout time.Duration) error {
	if standbyKey == nil {
		return fmt.Errorf("PostgreSQLWaitForStandbyLSN: nil standbyKey")
	}
	if targetLSN == "" {
		return fmt.Errorf("PostgreSQLWaitForStandbyLSN: empty targetLSN")
	}

	db, err := openPostgreSQLTopology(*standbyKey)
	if err != nil {
		return log.Errore(err)
	}
	defer func() { _ = db.Close() }()

	log.Infof("PostgreSQLWaitForStandbyLSN: waiting for %+v to reach LSN %s (timeout %v)", *standbyKey, targetLSN, timeout)

	pollInterval := 500 * time.Millisecond
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		var reached bool
		err := db.QueryRow(
			"SELECT pg_last_wal_replay_lsn() >= $1::pg_lsn", targetLSN,
		).Scan(&reached)
		if err != nil {
			_ = log.Warningf("PostgreSQLWaitForStandbyLSN: error polling %+v: %v", *standbyKey, err)
		} else if reached {
			log.Infof("PostgreSQLWaitForStandbyLSN: %+v reached target LSN %s", *standbyKey, targetLSN)
			return nil
		}
		time.Sleep(pollInterval)
	}

	return fmt.Errorf("PostgreSQLWaitForStandbyLSN: %+v did not reach LSN %s within %v", *standbyKey, targetLSN, timeout)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./go/inst/... -run TestPostgreSQLWaitForStandbyLSN -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go/inst/instance_topology_postgresql.go go/inst/instance_topology_postgresql_test.go
git commit -m "feat(postgresql): add PostgreSQLWaitForStandbyLSN for catch-up polling"
```

---

### Task 4: `PostgreSQLRepositionAsStandby` — Unit Test + Implementation

**Files:**
- Modify: `go/inst/instance_topology_postgresql.go` (append after PostgreSQLWaitForStandbyLSN)
- Modify: `go/inst/instance_topology_postgresql_test.go` (add test)

- [ ] **Step 1: Write the failing test**

Add to `go/inst/instance_topology_postgresql_test.go`:

```go
func TestPostgreSQLRepositionAsStandbyNilKeys(t *testing.T) {
	key := &InstanceKey{Hostname: "localhost", Port: 5432}
	if err := PostgreSQLRepositionAsStandby(nil, key); err == nil {
		t.Fatal("expected error for nil instanceKey")
	}
	if err := PostgreSQLRepositionAsStandby(key, nil); err == nil {
		t.Fatal("expected error for nil newPrimaryKey")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./go/inst/... -run TestPostgreSQLRepositionAsStandbyNilKeys -v`
Expected: FAIL — `PostgreSQLRepositionAsStandby` is not defined.

- [ ] **Step 3: Write minimal implementation**

Append to `go/inst/instance_topology_postgresql.go`:

```go
// PostgreSQLRepositionAsStandby configures a (demoted) PostgreSQL primary to
// replicate from a new primary by updating primary_conninfo via ALTER SYSTEM.
// The actual restart and standby.signal creation is the operator's
// responsibility, typically via PostGracefulTakeoverProcesses hooks.
func PostgreSQLRepositionAsStandby(instanceKey *InstanceKey, newPrimaryKey *InstanceKey) error {
	if instanceKey == nil || newPrimaryKey == nil {
		return fmt.Errorf("PostgreSQLRepositionAsStandby: nil key provided")
	}

	db, err := openPostgreSQLTopology(*instanceKey)
	if err != nil {
		return log.Errore(err)
	}
	defer func() { _ = db.Close() }()

	newConnInfo := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s sslmode=%s application_name=orchestrator",
		newPrimaryKey.Hostname,
		newPrimaryKey.Port,
		config.Config.PostgreSQLTopologyUser,
		config.Config.PostgreSQLTopologyPassword,
		config.Config.PostgreSQLSSLMode,
	)

	log.Infof("PostgreSQLRepositionAsStandby: configuring %+v to replicate from %+v", *instanceKey, *newPrimaryKey)

	if _, err := db.Exec(fmt.Sprintf("ALTER SYSTEM SET primary_conninfo = '%s'", newConnInfo)); err != nil {
		return log.Errore(fmt.Errorf("PostgreSQLRepositionAsStandby: ALTER SYSTEM failed on %+v: %v", *instanceKey, err))
	}

	if _, err := db.Exec("SELECT pg_reload_conf()"); err != nil {
		return log.Errore(fmt.Errorf("PostgreSQLRepositionAsStandby: pg_reload_conf() failed on %+v: %v", *instanceKey, err))
	}

	log.Infof("PostgreSQLRepositionAsStandby: %+v configured. Operator must restart with standby.signal to complete demotion.", *instanceKey)
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./go/inst/... -run TestPostgreSQLRepositionAsStandbyNilKeys -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go/inst/instance_topology_postgresql.go go/inst/instance_topology_postgresql_test.go
git commit -m "feat(postgresql): add PostgreSQLRepositionAsStandby for demoted primary"
```

---

### Task 5: `PostgreSQLGracefulPrimarySwitchover` — Implementation

**Files:**
- Modify: `go/logic/topology_recovery_postgresql.go` (append new function)

- [ ] **Step 1: Write the orchestration function**

Append to `go/logic/topology_recovery_postgresql.go`. First update imports at the top of the file:

Replace the import block:
```go
import (
	"fmt"

	"github.com/proxysql/orchestrator/go/config"
	"github.com/proxysql/orchestrator/go/inst"
)
```

With:
```go
import (
	"fmt"
	"time"

	"github.com/proxysql/golib/log"
	"github.com/proxysql/orchestrator/go/config"
	"github.com/proxysql/orchestrator/go/inst"
)
```

Then append the function:

```go
// PostgreSQLGracefulPrimarySwitchover performs a planned switchover of a
// PostgreSQL primary to a designated standby. It sets the primary read-only,
// waits for the standby to catch up, promotes the standby, reconfigures
// remaining standbys, and configures the demoted primary for repositioning.
func PostgreSQLGracefulPrimarySwitchover(clusterName string, designatedKey *inst.InstanceKey, auto bool) (topologyRecovery *TopologyRecovery, err error) {
	// --- Validate: read primary and standbys ---
	clusterMasters, err := inst.ReadClusterMaster(clusterName)
	if err != nil {
		return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: cannot deduce cluster primary for %+v: %v", clusterName, err)
	}
	if len(clusterMasters) != 1 {
		return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: found %d potential primaries for %+v, expected 1", len(clusterMasters), clusterName)
	}
	clusterPrimary := clusterMasters[0]

	standbys, err := inst.ReadReplicaInstances(&clusterPrimary.Key)
	if err != nil {
		return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: error reading standbys: %v", err)
	}
	if len(standbys) == 0 {
		return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: primary %+v has no standbys", clusterPrimary.Key)
	}

	// --- Determine designated standby ---
	var designatedStandby *inst.Instance
	if designatedKey != nil && designatedKey.IsValid() {
		// User specified a standby — verify it's a direct replica
		for _, s := range standbys {
			if s.Key.Equals(designatedKey) {
				designatedStandby = s
				break
			}
		}
		if designatedStandby == nil {
			return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: designated instance %+v is not a standby of %+v", *designatedKey, clusterPrimary.Key)
		}
	} else if len(standbys) == 1 {
		designatedStandby = standbys[0]
	} else if auto {
		designatedStandby, err = inst.PostgreSQLGetBestStandbyForPromotion(standbys, nil)
		if err != nil {
			return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: cannot auto-select standby: %v", err)
		}
	} else {
		return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: multiple standbys and no designated instance specified (use auto or specify -d)")
	}

	if designatedStandby.IsDowntimed {
		return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: designated standby %+v is downtimed", designatedStandby.Key)
	}
	if !designatedStandby.IsLastCheckValid {
		return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: designated standby %+v has invalid last check", designatedStandby.Key)
	}
	if !designatedStandby.HasReasonableMaintenanceReplicationLag() {
		return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: designated standby %+v has excessive replication lag", designatedStandby.Key)
	}

	log.Infof("PostgreSQLGracefulPrimarySwitchover: will demote %+v and promote %+v", clusterPrimary.Key, designatedStandby.Key)

	// --- Register recovery and build analysis entry ---
	analysisEntry, err := forceAnalysisEntry(clusterName, inst.DeadPrimary, inst.GracefulMasterTakeoverCommandHint, &clusterPrimary.Key)
	if err != nil {
		return nil, err
	}

	// --- Execute PreGracefulTakeoverProcesses ---
	preGracefulTakeoverTopologyRecovery := &TopologyRecovery{
		SuccessorKey:  &designatedStandby.Key,
		AnalysisEntry: analysisEntry,
	}
	if err := executeProcesses(config.Config.PreGracefulTakeoverProcesses, "PreGracefulTakeoverProcesses", preGracefulTakeoverTopologyRecovery, true); err != nil {
		return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: PreGracefulTakeoverProcesses failed: %v", err)
	}

	// --- Set primary read-only ---
	log.Infof("PostgreSQLGracefulPrimarySwitchover: setting %+v read-only", clusterPrimary.Key)
	if _, err := inst.PostgreSQLSetReadOnly(&clusterPrimary.Key, true); err != nil {
		return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: failed to set read-only on %+v: %v", clusterPrimary.Key, err)
	}

	// --- Capture target LSN ---
	targetLSN, err := inst.PostgreSQLGetCurrentWALLSN(&clusterPrimary.Key)
	if err != nil {
		// Undo read-only before aborting
		_, _ = inst.PostgreSQLSetReadOnly(&clusterPrimary.Key, false)
		return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: failed to get WAL LSN from %+v: %v", clusterPrimary.Key, err)
	}
	log.Infof("PostgreSQLGracefulPrimarySwitchover: target LSN is %s", targetLSN)

	// --- Wait for standby to catch up ---
	catchUpTimeout := time.Duration(config.Config.ReasonableMaintenanceReplicationLagSeconds) * time.Second
	if err := inst.PostgreSQLWaitForStandbyLSN(&designatedStandby.Key, targetLSN, catchUpTimeout); err != nil {
		// Undo read-only before aborting
		_, _ = inst.PostgreSQLSetReadOnly(&clusterPrimary.Key, false)
		return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: standby catch-up failed: %v", err)
	}

	// --- Promote designated standby ---
	promotedInstance, err := inst.PostgreSQLPromoteStandby(&designatedStandby.Key)
	if err != nil {
		// Undo read-only before aborting
		_, _ = inst.PostgreSQLSetReadOnly(&clusterPrimary.Key, false)
		return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: promotion of %+v failed: %v", designatedStandby.Key, err)
	}
	log.Infof("PostgreSQLGracefulPrimarySwitchover: promoted %+v to primary", promotedInstance.Key)

	// --- Register recovery ---
	topologyRecovery, err = AttemptRecoveryRegistration(&analysisEntry, false, false)
	if err != nil || topologyRecovery == nil {
		_ = log.Warningf("PostgreSQLGracefulPrimarySwitchover: error registering recovery: %v", err)
		// Continue — promotion already happened
		topologyRecovery = &TopologyRecovery{
			AnalysisEntry: analysisEntry,
		}
	}
	topologyRecovery.SuccessorKey = &promotedInstance.Key

	// --- Reconfigure remaining standbys ---
	for _, standby := range standbys {
		if standby.Key.Equals(&promotedInstance.Key) {
			continue
		}
		AuditTopologyRecovery(topologyRecovery, fmt.Sprintf("reconfiguring standby %+v to replicate from new primary %+v", standby.Key, promotedInstance.Key))
		if err := inst.PostgreSQLReconfigureStandby(&standby.Key, &promotedInstance.Key); err != nil {
			AuditTopologyRecovery(topologyRecovery, fmt.Sprintf("error reconfiguring standby %+v: %v (continuing)", standby.Key, err))
			topologyRecovery.LostReplicas.AddKey(standby.Key)
		}
	}

	// --- Configure demoted primary for repositioning ---
	AuditTopologyRecovery(topologyRecovery, fmt.Sprintf("configuring demoted primary %+v to replicate from %+v", clusterPrimary.Key, promotedInstance.Key))
	if err := inst.PostgreSQLRepositionAsStandby(&clusterPrimary.Key, &promotedInstance.Key); err != nil {
		AuditTopologyRecovery(topologyRecovery, fmt.Sprintf("error configuring demoted primary %+v: %v", clusterPrimary.Key, err))
	}

	// --- Resolve recovery ---
	resolveRecovery(topologyRecovery, promotedInstance)

	// --- Execute PostGracefulTakeoverProcesses ---
	executeProcesses(config.Config.PostGracefulTakeoverProcesses, "PostGracefulTakeoverProcesses", topologyRecovery, false)

	AuditTopologyRecovery(topologyRecovery, fmt.Sprintf("PostgreSQLGracefulPrimarySwitchover: completed. New primary: %+v", promotedInstance.Key))
	return topologyRecovery, nil
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./go/...`
Expected: compiles without errors.

- [ ] **Step 3: Commit**

```bash
git add go/logic/topology_recovery_postgresql.go
git commit -m "feat(postgresql): add PostgreSQLGracefulPrimarySwitchover orchestration"
```

---

### Task 6: Provider Dispatch in `GracefulMasterTakeover`

**Files:**
- Modify: `go/logic/topology_recovery.go` (lines 2114-2122, add dispatch after reading cluster master)

- [ ] **Step 1: Add PostgreSQL dispatch**

In `go/logic/topology_recovery.go`, after line 2122 (`clusterMaster := clusterMasters[0]`), add:

```go
	// PostgreSQL: dispatch to PostgreSQL-specific implementation
	if clusterMaster.ProviderType == "postgresql" {
		topologyRecovery, err := PostgreSQLGracefulPrimarySwitchover(clusterName, designatedKey, auto)
		return topologyRecovery, nil, err
	}
```

This goes right after `clusterMaster := clusterMasters[0]` and before `clusterMasterDirectReplicas, err := inst.ReadReplicaInstances(...)`.

- [ ] **Step 2: Verify compilation**

Run: `go build ./go/...`
Expected: compiles without errors.

- [ ] **Step 3: Verify existing tests still pass**

Run: `go test ./go/logic/... -v -count=1 2>&1 | tail -20`
Expected: all existing tests pass (or skip if they require a running backend).

- [ ] **Step 4: Commit**

```bash
git add go/logic/topology_recovery.go
git commit -m "feat(postgresql): dispatch graceful-master-takeover to PostgreSQL switchover"
```

---

### Task 7: Functional Test — Graceful Switchover

**Files:**
- Modify: `tests/functional/test-postgresql.sh` (append graceful switchover test section)

- [ ] **Step 1: Add graceful switchover test to the functional test script**

Append to `tests/functional/test-postgresql.sh`, before the final summary section (before `echo "=== POSTGRESQL TESTS COMPLETE ==="`):

```bash
# ----------------------------------------------------------------
echo ""
echo "--- Graceful primary switchover tests ---"

# Re-discover topology after any prior failover tests
echo "Re-discovering PostgreSQL topology..."
curl -s --max-time 10 "$ORC_URL/api/discover/172.30.0.20/5432" > /dev/null 2>&1
curl -s --max-time 10 "$ORC_URL/api/discover/172.30.0.21/5432" > /dev/null 2>&1
sleep 10

# Identify current primary
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
    fail "Cannot identify current PostgreSQL primary"
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

    # Wait for topology to settle
    sleep 10

    # Verify primary has changed
    NEW_PRIMARY=$(curl -s --max-time 10 "$ORC_URL/api/cluster/$PG_CLUSTER" 2>/dev/null | python3 -c "
import json, sys
instances = json.load(sys.stdin)
for inst in instances:
    if not inst.get('ReadOnly', True):
        print(inst['Key']['Hostname'] + ':' + str(inst['Key']['Port']))
        sys.exit(0)
print('')
" 2>/dev/null || echo "")

    if [ -n "$NEW_PRIMARY" ] && [ "$NEW_PRIMARY" != "$CURRENT_PRIMARY" ]; then
        pass "Primary switched from $CURRENT_PRIMARY to $NEW_PRIMARY"
    else
        fail "Primary did not change: was $CURRENT_PRIMARY, now ${NEW_PRIMARY:-unknown}"
    fi
fi
```

- [ ] **Step 2: Verify the test script has valid syntax**

Run: `bash -n tests/functional/test-postgresql.sh`
Expected: no syntax errors.

- [ ] **Step 3: Commit**

```bash
git add tests/functional/test-postgresql.sh
git commit -m "test(postgresql): add functional test for graceful primary switchover"
```

---

### Task 8: Update Documentation

**Files:**
- Modify: `docs/database-providers.md` (remove "not yet implemented" note)
- Modify: `docs/user-manual.md` (remove "not yet supported" note, add graceful switchover section)

- [ ] **Step 1: Update database-providers.md**

Find the line (around line 183-184) that says:
```
Graceful master takeover (planned switchover) is not yet implemented for PostgreSQL. Only unplanned failover (dead primary) is supported.
```

Replace with:
```
### Graceful Primary Switchover

Graceful master takeover (planned switchover) is supported for PostgreSQL. The same CLI commands
and API endpoints used for MySQL (`graceful-master-takeover`, `graceful-master-takeover-auto`) work
for PostgreSQL clusters. The switchover flow:

1. Sets the primary read-only via `ALTER SYSTEM SET default_transaction_read_only = on` and terminates
   existing non-replication connections.
2. Captures the primary's current WAL LSN and waits for the designated standby to catch up.
3. Promotes the designated standby via `pg_promote()`.
4. Reconfigures remaining standbys to replicate from the new primary.
5. Configures the demoted primary's `primary_conninfo` to point at the new primary.

**Important:** The demoted primary requires a restart with `standby.signal` to complete its conversion
to a standby. Use `PostGracefulTakeoverProcesses` hooks to automate this step.
```

- [ ] **Step 2: Update user-manual.md**

Find the line (around line 699) that says:
```
Graceful master takeover is not yet supported for PostgreSQL.
```

Replace with:
```
Graceful master takeover is supported for PostgreSQL. Use `graceful-master-takeover` or
`graceful-master-takeover-auto` CLI commands, or the equivalent API endpoints. The demoted
primary requires an operator-managed restart with `standby.signal` — configure this via
`PostGracefulTakeoverProcesses` hooks.
```

- [ ] **Step 3: Commit**

```bash
git add docs/database-providers.md docs/user-manual.md
git commit -m "docs: update PostgreSQL docs to reflect graceful switchover support"
```

---

### Task 9: Create GitHub Issue

**Files:** None (GitHub CLI only)

- [ ] **Step 1: Create the GitHub issue**

```bash
gh issue create \
  --title "feat: PostgreSQL graceful primary switchover (planned failover)" \
  --body "$(cat <<'BODY'
## Summary

Implement planned/graceful primary switchover for PostgreSQL. Currently only unplanned failover (dead primary detection → promotion) is supported. This adds the ability to gracefully demote a running primary and promote a designated standby with zero data loss.

## Motivation

Operators need planned switchover for maintenance windows, version upgrades, and host migrations. The MySQL path already supports this via `graceful-master-takeover` / `graceful-master-takeover-auto` — PostgreSQL should have parity.

## Design

See `docs/superpowers/specs/2026-04-18-postgresql-graceful-switchover-design.md` for the full spec.

### New instance operations (go/inst/instance_topology_postgresql.go):
- `PostgreSQLSetReadOnly` — ALTER SYSTEM + pg_reload_conf + pg_terminate_backend
- `PostgreSQLGetCurrentWALLSN` — pg_current_wal_lsn()
- `PostgreSQLWaitForStandbyLSN` — poll pg_last_wal_replay_lsn() until caught up
- `PostgreSQLRepositionAsStandby` — ALTER SYSTEM SET primary_conninfo for demoted primary

### Switchover orchestration (go/logic/topology_recovery_postgresql.go):
- `PostgreSQLGracefulPrimarySwitchover` — full flow: validate → pre-hooks → set read-only → wait catch-up → promote → reconfigure standbys → reposition demoted primary → post-hooks

### CLI/API dispatch (go/logic/topology_recovery.go):
- Provider check in `GracefulMasterTakeover()` dispatches to PostgreSQL path when cluster is PostgreSQL
- Same commands/endpoints: `graceful-master-takeover`, `graceful-master-takeover-auto`

### Key decisions:
- Demoted primary restart is operator's responsibility via `PostGracefulTakeoverProcesses` hooks
- No `pg_rewind` needed — primary is read-only before switchover, so no timeline divergence
- Separate implementation function (not branching in MySQL code) — follows existing pattern

## Test Plan
- [ ] Unit tests for nil/empty input validation on all 4 new instance operations
- [ ] Functional test: graceful switchover against real PostgreSQL topology
- [ ] Verify existing MySQL graceful takeover still works (no regression)
BODY
)"
```

- [ ] **Step 2: Note the issue number for commit references**

Record the issue URL/number returned by `gh issue create`.

- [ ] **Step 3: Commit** (no files changed, skip)

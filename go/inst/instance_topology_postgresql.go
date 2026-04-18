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
	"fmt"
	"strings"
	"time"

	"github.com/proxysql/golib/log"
	"github.com/proxysql/orchestrator/go/config"
)

// PostgreSQLPromoteStandby promotes a PostgreSQL standby to primary by calling
// pg_promote(). It waits for the instance to exit recovery mode and returns
// the promoted instance.
func PostgreSQLPromoteStandby(instanceKey *InstanceKey) (*Instance, error) {
	if instanceKey == nil {
		return nil, fmt.Errorf("PostgreSQLPromoteStandby: nil instanceKey")
	}

	db, err := openPostgreSQLTopology(*instanceKey)
	if err != nil {
		return nil, log.Errore(err)
	}
	defer func() { _ = db.Close() }()

	// Verify the instance is actually in recovery (a standby)
	var inRecovery bool
	if err := db.QueryRow("SELECT pg_is_in_recovery()").Scan(&inRecovery); err != nil {
		return nil, log.Errore(err)
	}
	if !inRecovery {
		return nil, fmt.Errorf("PostgreSQLPromoteStandby: %+v is not in recovery mode (not a standby)", *instanceKey)
	}

	log.Infof("PostgreSQLPromoteStandby: promoting %+v", *instanceKey)

	// pg_promote() is available since PostgreSQL 12
	if _, err := db.Exec("SELECT pg_promote(true, 60)"); err != nil {
		return nil, log.Errore(fmt.Errorf("PostgreSQLPromoteStandby: pg_promote() failed on %+v: %v", *instanceKey, err))
	}

	// Wait for the instance to exit recovery mode
	waitTimeout := 30 * time.Second
	pollInterval := 500 * time.Millisecond
	deadline := time.Now().Add(waitTimeout)

	for time.Now().Before(deadline) {
		if err := db.QueryRow("SELECT pg_is_in_recovery()").Scan(&inRecovery); err != nil {
			_ = log.Warningf("PostgreSQLPromoteStandby: error checking recovery state on %+v: %v", *instanceKey, err)
		} else if !inRecovery {
			log.Infof("PostgreSQLPromoteStandby: %+v has exited recovery mode (promoted to primary)", *instanceKey)
			// Re-read the instance to get updated state
			return ReadPostgreSQLTopologyInstance(instanceKey)
		}
		time.Sleep(pollInterval)
	}

	return nil, fmt.Errorf("PostgreSQLPromoteStandby: %+v did not exit recovery mode within %v", *instanceKey, waitTimeout)
}

// PostgreSQLReconfigureStandby reconfigures a PostgreSQL standby to replicate
// from a new primary by updating primary_conninfo via ALTER SYSTEM and
// reloading the configuration.
func PostgreSQLReconfigureStandby(standbyKey *InstanceKey, newPrimaryKey *InstanceKey) error {
	if standbyKey == nil || newPrimaryKey == nil {
		return fmt.Errorf("PostgreSQLReconfigureStandby: nil key provided")
	}

	db, err := openPostgreSQLTopology(*standbyKey)
	if err != nil {
		return log.Errore(err)
	}
	defer func() { _ = db.Close() }()

	// Verify the instance is a standby
	var inRecovery bool
	if err := db.QueryRow("SELECT pg_is_in_recovery()").Scan(&inRecovery); err != nil {
		return log.Errore(err)
	}
	if !inRecovery {
		return fmt.Errorf("PostgreSQLReconfigureStandby: %+v is not in recovery mode (not a standby)", *standbyKey)
	}

	// Build new primary_conninfo
	newConnInfo := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s sslmode=%s application_name=orchestrator",
		newPrimaryKey.Hostname,
		newPrimaryKey.Port,
		config.Config.PostgreSQLTopologyUser,
		config.Config.PostgreSQLTopologyPassword,
		config.Config.PostgreSQLSSLMode,
	)

	log.Infof("PostgreSQLReconfigureStandby: reconfiguring %+v to replicate from %+v", *standbyKey, *newPrimaryKey)

	// Update primary_conninfo using ALTER SYSTEM
	// Escape single quotes to prevent SQL breakage from passwords containing quotes
	escapedConnInfo := strings.ReplaceAll(newConnInfo, "'", "''")
	_, err = db.Exec(fmt.Sprintf("ALTER SYSTEM SET primary_conninfo = '%s'", escapedConnInfo))
	if err != nil {
		return log.Errore(fmt.Errorf("PostgreSQLReconfigureStandby: ALTER SYSTEM failed on %+v: %v", *standbyKey, err))
	}

	// Reload configuration
	if _, err := db.Exec("SELECT pg_reload_conf()"); err != nil {
		return log.Errore(fmt.Errorf("PostgreSQLReconfigureStandby: pg_reload_conf() failed on %+v: %v", *standbyKey, err))
	}

	// Restart WAL receiver by pausing and resuming replay
	// This forces the standby to reconnect to the new primary
	_, _ = db.Exec("SELECT pg_wal_replay_pause()")
	time.Sleep(100 * time.Millisecond)
	_, _ = db.Exec("SELECT pg_wal_replay_resume()")

	log.Infof("PostgreSQLReconfigureStandby: %+v reconfigured to replicate from %+v", *standbyKey, *newPrimaryKey)
	return nil
}

// PostgreSQLGetBestStandbyForPromotion picks the best standby to promote
// from the list of replicas. It prefers the standby with:
// 1. Valid last check
// 2. Replication running
// 3. Lowest replication lag
// 4. Not downtimed
func PostgreSQLGetBestStandbyForPromotion(replicas [](*Instance), candidateKey *InstanceKey) (*Instance, error) {
	if len(replicas) == 0 {
		return nil, fmt.Errorf("PostgreSQLGetBestStandbyForPromotion: no replicas provided")
	}

	// If a candidate is specified and it's valid, prefer it
	if candidateKey != nil {
		for _, replica := range replicas {
			if replica.Key.Equals(candidateKey) && replica.IsLastCheckValid && !replica.IsDowntimed {
				return replica, nil
			}
		}
	}

	// Otherwise, pick the best standby
	var bestStandby *Instance
	for _, replica := range replicas {
		if !replica.IsLastCheckValid {
			continue
		}
		if replica.IsDowntimed {
			continue
		}
		if bestStandby == nil {
			bestStandby = replica
			continue
		}
		// Prefer replica with lower lag
		if replica.ReplicationLagSeconds.Valid && bestStandby.ReplicationLagSeconds.Valid {
			if replica.ReplicationLagSeconds.Int64 < bestStandby.ReplicationLagSeconds.Int64 {
				bestStandby = replica
			}
		}
		// Prefer replica with higher LSN (more up-to-date)
		if replica.ExecBinlogCoordinates.LogPos > bestStandby.ExecBinlogCoordinates.LogPos {
			bestStandby = replica
		}
	}

	if bestStandby == nil {
		return nil, fmt.Errorf("PostgreSQLGetBestStandbyForPromotion: no valid standby found for promotion")
	}
	return bestStandby, nil
}

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

	// Escape single quotes to prevent SQL breakage from passwords containing quotes
	escapedConnInfo := strings.ReplaceAll(newConnInfo, "'", "''")
	if _, err := db.Exec(fmt.Sprintf("ALTER SYSTEM SET primary_conninfo = '%s'", escapedConnInfo)); err != nil {
		return log.Errore(fmt.Errorf("PostgreSQLRepositionAsStandby: ALTER SYSTEM failed on %+v: %v", *instanceKey, err))
	}

	if _, err := db.Exec("SELECT pg_reload_conf()"); err != nil {
		return log.Errore(fmt.Errorf("PostgreSQLRepositionAsStandby: pg_reload_conf() failed on %+v: %v", *instanceKey, err))
	}

	log.Infof("PostgreSQLRepositionAsStandby: %+v configured. Operator must restart with standby.signal to complete demotion.", *instanceKey)
	return nil
}

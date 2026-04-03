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
	defer db.Close()

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
			log.Warningf("PostgreSQLPromoteStandby: error checking recovery state on %+v: %v", *instanceKey, err)
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
	defer db.Close()

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
	_, err = db.Exec(fmt.Sprintf("ALTER SYSTEM SET primary_conninfo = '%s'", newConnInfo))
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

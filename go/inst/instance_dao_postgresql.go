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
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/proxysql/golib/log"
	"github.com/proxysql/orchestrator/go/config"
)

// connInfoHostPortRegexp extracts host and port from primary_conninfo
var connInfoHostPortRegexp = regexp.MustCompile(`host=(\S+).*port=(\d+)`)

// ReadPostgreSQLTopologyInstance connects to a PostgreSQL instance, discovers
// its role (primary or standby), and populates an Instance struct that maps
// PostgreSQL concepts to orchestrator's MySQL-oriented data model.
func ReadPostgreSQLTopologyInstance(instanceKey *InstanceKey) (*Instance, error) {
	if instanceKey == nil {
		return nil, fmt.Errorf("ReadPostgreSQLTopologyInstance: nil instanceKey")
	}

	db, err := openPostgreSQLTopology(*instanceKey)
	if err != nil {
		return nil, log.Errore(err)
	}
	defer func() { _ = db.Close() }()

	instance := NewInstance()
	instance.Key = *instanceKey

	readingStartTime := time.Now()

	// Determine PostgreSQL version
	var version string
	if err := db.QueryRow("SELECT version()").Scan(&version); err != nil {
		return nil, log.Errore(err)
	}
	instance.Version = parsePostgreSQLVersion(version)
	instance.FlavorName = "PostgreSQL"
	instance.ProviderType = "postgresql"

	// Check whether this instance is in recovery (standby) or is a primary
	var inRecovery bool
	if err := db.QueryRow("SELECT pg_is_in_recovery()").Scan(&inRecovery); err != nil {
		return nil, log.Errore(err)
	}

	// PostgreSQL always uses WAL-based replication (analogous to GTID)
	instance.GTIDMode = "ON"
	instance.SupportsOracleGTID = false
	instance.UsingOracleGTID = false
	instance.UsingMariaDBGTID = false
	instance.LogBinEnabled = true
	instance.LogReplicationUpdatesEnabled = true

	if inRecovery {
		// This is a standby (replica)
		if err := readPostgreSQLStandbyInfo(db, instance); err != nil {
			return nil, err
		}
	} else {
		// This is a primary (master)
		if err := readPostgreSQLPrimaryInfo(db, instance); err != nil {
			return nil, err
		}
	}

	instance.ReadOnly = inRecovery

	// Set cluster name: for primaries it's their own key, for standbys it's the master's key
	if inRecovery && instance.MasterKey.Hostname != "" {
		instance.ClusterName = instance.MasterKey.StringCode()
	} else {
		instance.ClusterName = instance.Key.StringCode()
	}
	instance.SuggestedClusterAlias = instance.ClusterName

	// Set discovery latency
	instance.LastDiscoveryLatency = time.Since(readingStartTime)

	// Mark as valid
	instance.IsLastCheckValid = true
	instance.IsUpToDate = true
	instance.IsRecentlyChecked = true
	instance.LastSeenTimestamp = time.Now().Format("2006-01-02 15:04:05")

	// Write instance to backend DB
	_ = WriteInstance(instance, true, nil)

	return instance, nil
}

// readPostgreSQLStandbyInfo populates Instance fields for a PostgreSQL standby.
func readPostgreSQLStandbyInfo(db *sql.DB, instance *Instance) error {
	// Get WAL receiver status and source connection info
	var status, conninfo sql.NullString
	var senderHost sql.NullString
	var senderPort sql.NullInt64
	err := db.QueryRow(`
		SELECT status, conninfo,
			sender_host, sender_port
		FROM pg_stat_wal_receiver
		LIMIT 1
	`).Scan(&status, &conninfo, &senderHost, &senderPort)

	walReceiverActive := false
	if err == sql.ErrNoRows {
		// No WAL receiver row: replication is not running
		walReceiverActive = false
	} else if err != nil {
		return log.Errore(err)
	} else {
		walReceiverActive = status.Valid && status.String == "streaming"
	}

	instance.ReplicationIOThreadRuning = walReceiverActive
	instance.Slave_IO_Running = walReceiverActive
	instance.ReplicationIOThreadState = ReplicationThreadStateFromStatus(boolToString(walReceiverActive, "Yes", "No"))

	// WAL replay is considered running if we're in recovery
	// (the standby is always replaying unless paused)
	var walReplayPaused bool
	if err := db.QueryRow("SELECT pg_is_wal_replay_paused()").Scan(&walReplayPaused); err != nil {
		return log.Errore(err)
	}
	sqlThreadRunning := !walReplayPaused
	instance.ReplicationSQLThreadRuning = sqlThreadRunning
	instance.Slave_SQL_Running = sqlThreadRunning
	instance.ReplicationSQLThreadState = ReplicationThreadStateFromStatus(boolToString(sqlThreadRunning, "Yes", "No"))

	// Get current replay LSN as position
	var replayLSN sql.NullString
	if err := db.QueryRow("SELECT pg_last_wal_replay_lsn()::text").Scan(&replayLSN); err != nil {
		return log.Errore(err)
	}
	if replayLSN.Valid {
		lsnPos := lsnToInt64(replayLSN.String)
		instance.ExecBinlogCoordinates = BinlogCoordinates{
			LogFile: "wal",
			LogPos:  lsnPos,
		}
		instance.ReadBinlogCoordinates = instance.ExecBinlogCoordinates
		instance.SelfBinlogCoordinates = BinlogCoordinates{
			LogFile: "wal",
			LogPos:  lsnPos,
		}
	}

	// Compute replication lag
	var lagSeconds sql.NullFloat64
	if err := db.QueryRow(`
		SELECT EXTRACT(EPOCH FROM now() - pg_last_xact_replay_timestamp())
	`).Scan(&lagSeconds); err != nil {
		return log.Errore(err)
	}
	if lagSeconds.Valid {
		lag := int64(lagSeconds.Float64)
		instance.SecondsBehindMaster = sql.NullInt64{Int64: lag, Valid: true}
		instance.ReplicationLagSeconds = sql.NullInt64{Int64: lag, Valid: true}
		instance.SlaveLagSeconds = instance.ReplicationLagSeconds
	} else {
		instance.SecondsBehindMaster = sql.NullInt64{Int64: 0, Valid: false}
		instance.ReplicationLagSeconds = sql.NullInt64{Int64: 0, Valid: false}
		instance.SlaveLagSeconds = instance.ReplicationLagSeconds
	}

	// Extract primary host:port — try multiple sources
	masterFound := false
	// Source 1: sender_host/sender_port from pg_stat_wal_receiver
	if senderHost.Valid && senderHost.String != "" {
		port := int(senderPort.Int64)
		if port == 0 {
			port = 5432
		}
		instance.MasterKey = InstanceKey{Hostname: senderHost.String, Port: port}
		masterFound = true
	}
	// Source 2: conninfo from pg_stat_wal_receiver
	if !masterFound && conninfo.Valid && conninfo.String != "" {
		host, port := parseConnInfo(conninfo.String)
		if host != "" {
			if port == 0 {
				port = 5432
			}
			instance.MasterKey = InstanceKey{Hostname: host, Port: port}
			masterFound = true
		}
	}
	// Source 3: parse primary_conninfo from postgresql.auto.conf via SHOW
	if !masterFound {
		var primaryConninfo sql.NullString
		if err := db.QueryRow("SELECT setting FROM pg_settings WHERE name = 'primary_conninfo'").Scan(&primaryConninfo); err == nil && primaryConninfo.Valid {
			host, port := parseConnInfo(primaryConninfo.String)
			if host != "" {
				if port == 0 {
					port = 5432
				}
				instance.MasterKey = InstanceKey{Hostname: host, Port: port}
				masterFound = true
			}
		}
	}
	if !masterFound {
		_ = log.Warningf("PostgreSQL standby %s:%d: could not determine master host", instance.Key.Hostname, instance.Key.Port)
	}

	instance.HasReplicationCredentials = true
	instance.ReplicationCredentialsAvailable = true
	instance.ReplicationDepth = 1
	instance.Uptime = 1 // Indicate instance is up

	return nil
}

// readPostgreSQLPrimaryInfo populates Instance fields for a PostgreSQL primary.
func readPostgreSQLPrimaryInfo(db *sql.DB, instance *Instance) error {
	// Get current WAL LSN
	var currentLSN string
	if err := db.QueryRow("SELECT pg_current_wal_lsn()::text").Scan(&currentLSN); err != nil {
		return log.Errore(err)
	}
	lsnPos := lsnToInt64(currentLSN)
	instance.SelfBinlogCoordinates = BinlogCoordinates{
		LogFile: "wal",
		LogPos:  lsnPos,
	}

	// A primary is not replicating
	instance.ReplicationIOThreadRuning = false
	instance.ReplicationSQLThreadRuning = false
	instance.ReplicationIOThreadState = ReplicationThreadStateNoThread
	instance.ReplicationSQLThreadState = ReplicationThreadStateNoThread
	instance.SecondsBehindMaster = sql.NullInt64{Int64: 0, Valid: false}
	instance.ReplicationLagSeconds = sql.NullInt64{Int64: 0, Valid: false}
	instance.ReplicationDepth = 0
	instance.Uptime = 1

	// Discover standbys from pg_stat_replication
	instance.Replicas = make(InstanceKeyMap)
	rows, err := db.Query(`
		SELECT client_hostname, client_addr, client_port
		FROM pg_stat_replication
	`)
	if err != nil {
		return log.Errore(err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var clientHostname, clientAddr sql.NullString
		var clientPort sql.NullInt64
		if err := rows.Scan(&clientHostname, &clientAddr, &clientPort); err != nil {
			return log.Errore(err)
		}
		hostname := ""
		if clientHostname.Valid && clientHostname.String != "" {
			hostname = clientHostname.String
		} else if clientAddr.Valid && clientAddr.String != "" {
			hostname = clientAddr.String
		}
		if hostname != "" {
			port := config.Config.DefaultInstancePort
			// client_port in pg_stat_replication is the ephemeral port, not the PG listen port.
			// We use DefaultInstancePort instead.
			replicaKey := InstanceKey{Hostname: hostname, Port: port}
			instance.AddReplicaKey(&replicaKey)
		}
	}
	instance.SlaveHosts = instance.Replicas

	return nil
}

// parsePostgreSQLVersion extracts a clean version string from the full
// pg version() output, e.g. "PostgreSQL 16.2" from
// "PostgreSQL 16.2 (Debian 16.2-1.pgdg120+2) on x86_64-..."
func parsePostgreSQLVersion(fullVersion string) string {
	parts := strings.Fields(fullVersion)
	if len(parts) >= 2 && strings.EqualFold(parts[0], "PostgreSQL") {
		return parts[1]
	}
	return fullVersion
}

// parseConnInfo extracts host and port from a PostgreSQL primary_conninfo string.
func parseConnInfo(conninfo string) (string, int) {
	matches := connInfoHostPortRegexp.FindStringSubmatch(conninfo)
	if len(matches) >= 3 {
		host := matches[1]
		port, _ := strconv.Atoi(matches[2])
		return host, port
	}
	// Try just host
	hostMatch := regexp.MustCompile(`host=(\S+)`).FindStringSubmatch(conninfo)
	if len(hostMatch) >= 2 {
		return hostMatch[1], 0
	}
	return "", 0
}

// lsnToInt64 converts a PostgreSQL LSN string (e.g. "0/16B3748") to an int64
// for use as a BinlogCoordinates LogPos.
func lsnToInt64(lsn string) int64 {
	parts := strings.Split(lsn, "/")
	if len(parts) != 2 {
		return 0
	}
	high, err := strconv.ParseInt(parts[0], 16, 64)
	if err != nil {
		return 0
	}
	low, err := strconv.ParseInt(parts[1], 16, 64)
	if err != nil {
		return 0
	}
	return (high << 32) | low
}

// boolToString is a helper that returns trueStr if v is true, falseStr otherwise.
func boolToString(v bool, trueStr, falseStr string) string {
	if v {
		return trueStr
	}
	return falseStr
}

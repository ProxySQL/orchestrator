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
	"net/url"

	_ "github.com/lib/pq"
	"github.com/proxysql/golib/log"
	"github.com/proxysql/orchestrator/go/config"
)

// PostgreSQLProvider implements DatabaseProvider for PostgreSQL streaming
// replication topologies.
type PostgreSQLProvider struct{}

// NewPostgreSQLProvider creates a new PostgreSQL database provider.
func NewPostgreSQLProvider() *PostgreSQLProvider {
	return &PostgreSQLProvider{}
}

// ProviderName returns "postgresql".
func (p *PostgreSQLProvider) ProviderName() string {
	return "postgresql"
}

// openPostgreSQLTopology opens a connection to a PostgreSQL instance using
// credentials from the orchestrator configuration.
func openPostgreSQLTopology(key InstanceKey) (*sql.DB, error) {
	u := &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(config.Config.PostgreSQLTopologyUser, config.Config.PostgreSQLTopologyPassword),
		Host:     fmt.Sprintf("%s:%d", key.Hostname, key.Port),
		Path:     "postgres",
		RawQuery: fmt.Sprintf("sslmode=%s&connect_timeout=5", config.Config.PostgreSQLSSLMode),
	}
	db, err := sql.Open("postgres", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(3)
	db.SetMaxIdleConns(1)
	return db, nil
}

// GetReplicationStatus retrieves the replication state for a PostgreSQL instance.
// On a standby it queries pg_stat_wal_receiver; on a primary it queries
// pg_current_wal_lsn().
func (p *PostgreSQLProvider) GetReplicationStatus(key InstanceKey) (*ReplicationStatus, error) {
	db, err := openPostgreSQLTopology(key)
	if err != nil {
		return nil, log.Errore(err)
	}
	defer db.Close()

	// Check whether this instance is in recovery (i.e. is a standby).
	var inRecovery bool
	if err := db.QueryRow("SELECT pg_is_in_recovery()").Scan(&inRecovery); err != nil {
		return nil, log.Errore(err)
	}

	if inRecovery {
		return p.getStandbyReplicationStatus(db)
	}
	return p.getPrimaryReplicationStatus(db)
}

// getStandbyReplicationStatus reads replication state from a PostgreSQL standby
// via pg_stat_wal_receiver and pg_last_wal_replay_lsn().
func (p *PostgreSQLProvider) getStandbyReplicationStatus(db *sql.DB) (*ReplicationStatus, error) {
	var status, lsn sql.NullString
	var lagSeconds sql.NullFloat64

	err := db.QueryRow(`
		SELECT
			COALESCE(r.status, ''),
			pg_last_wal_replay_lsn()::text,
			COALESCE(EXTRACT(EPOCH FROM now() - pg_last_xact_replay_timestamp()), -1)
		FROM (SELECT 'streaming' as status FROM pg_stat_wal_receiver LIMIT 1) r
	`).Scan(&status, &lsn, &lagSeconds)

	if err == sql.ErrNoRows {
		// No WAL receiver row means replication is not running.
		return &ReplicationStatus{
			ReplicaRunning:   false,
			IOThreadRunning:  false,
			SQLThreadRunning: false,
			Position:         "",
			Lag:              -1,
		}, nil
	}
	if err != nil {
		return nil, log.Errore(err)
	}

	ioRunning := status.Valid && status.String == "streaming"
	lag := int64(-1)
	if lagSeconds.Valid {
		lag = int64(lagSeconds.Float64)
	}

	position := ""
	if lsn.Valid {
		position = lsn.String
	}

	return &ReplicationStatus{
		ReplicaRunning:   ioRunning,
		IOThreadRunning:  ioRunning,
		SQLThreadRunning: ioRunning, // PG does not separate IO/SQL threads; mirror IO state
		Position:         position,
		Lag:              lag,
	}, nil
}

// getPrimaryReplicationStatus returns a ReplicationStatus for a primary server.
// A primary is not itself a replica, so ReplicaRunning is false, and we report
// the current WAL insert position.
func (p *PostgreSQLProvider) getPrimaryReplicationStatus(db *sql.DB) (*ReplicationStatus, error) {
	var lsn string
	if err := db.QueryRow("SELECT pg_current_wal_lsn()::text").Scan(&lsn); err != nil {
		return nil, log.Errore(err)
	}
	return &ReplicationStatus{
		ReplicaRunning:   false,
		IOThreadRunning:  false,
		SQLThreadRunning: false,
		Position:         lsn,
		Lag:              0,
	}, nil
}

// IsReplicaRunning checks whether the WAL receiver is active on a PostgreSQL
// standby instance.
func (p *PostgreSQLProvider) IsReplicaRunning(key InstanceKey) (bool, error) {
	db, err := openPostgreSQLTopology(key)
	if err != nil {
		return false, log.Errore(err)
	}
	defer db.Close()

	var status sql.NullString
	err = db.QueryRow("SELECT status FROM pg_stat_wal_receiver LIMIT 1").Scan(&status)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, log.Errore(err)
	}
	return status.Valid && status.String == "streaming", nil
}

// SetReadOnly sets or clears the default_transaction_read_only parameter on
// a PostgreSQL instance and reloads the configuration.
func (p *PostgreSQLProvider) SetReadOnly(key InstanceKey, readOnly bool) error {
	db, err := openPostgreSQLTopology(key)
	if err != nil {
		return log.Errore(err)
	}
	defer db.Close()

	value := "off"
	if readOnly {
		value = "on"
	}
	if _, err := db.Exec(fmt.Sprintf("ALTER SYSTEM SET default_transaction_read_only = %s", value)); err != nil {
		return log.Errore(err)
	}
	if _, err := db.Exec("SELECT pg_reload_conf()"); err != nil {
		return log.Errore(err)
	}
	return nil
}

// IsReadOnly checks whether default_transaction_read_only is enabled on a
// PostgreSQL instance.
func (p *PostgreSQLProvider) IsReadOnly(key InstanceKey) (bool, error) {
	db, err := openPostgreSQLTopology(key)
	if err != nil {
		return false, log.Errore(err)
	}
	defer db.Close()

	var value string
	if err := db.QueryRow("SHOW default_transaction_read_only").Scan(&value); err != nil {
		return false, log.Errore(err)
	}
	return value == "on", nil
}

// StartReplication is a no-op for PostgreSQL streaming replication. Streaming
// replication starts automatically when a standby connects to its primary.
// WAL replay is resumed if it was previously paused.
func (p *PostgreSQLProvider) StartReplication(key InstanceKey) error {
	log.Infof("PostgreSQL streaming replication on %s:%d starts automatically; resuming WAL replay if paused", key.Hostname, key.Port)

	db, err := openPostgreSQLTopology(key)
	if err != nil {
		return log.Errore(err)
	}
	defer db.Close()

	var inRecovery bool
	if err := db.QueryRow("SELECT pg_is_in_recovery()").Scan(&inRecovery); err != nil {
		return log.Errore(err)
	}
	if !inRecovery {
		log.Infof("StartReplication: %s:%d is a primary, WAL replay resume not applicable", key.Hostname, key.Port)
		return nil
	}

	if _, err := db.Exec("SELECT pg_wal_replay_resume()"); err != nil {
		return log.Errore(err)
	}
	return nil
}

// StopReplication pauses WAL replay on a PostgreSQL standby. This is the
// closest equivalent to stopping replication in MySQL. Note that the WAL
// receiver (IO thread equivalent) remains connected; only replay is paused.
func (p *PostgreSQLProvider) StopReplication(key InstanceKey) error {
	db, err := openPostgreSQLTopology(key)
	if err != nil {
		return log.Errore(err)
	}
	defer db.Close()

	var inRecovery bool
	if err := db.QueryRow("SELECT pg_is_in_recovery()").Scan(&inRecovery); err != nil {
		return log.Errore(err)
	}
	if !inRecovery {
		return fmt.Errorf("StopReplication: %s:%d is a primary, WAL replay pause not applicable", key.Hostname, key.Port)
	}

	if _, err := db.Exec("SELECT pg_wal_replay_pause()"); err != nil {
		return log.Errore(err)
	}
	return nil
}

// Compile-time check that PostgreSQLProvider implements DatabaseProvider.
var _ DatabaseProvider = (*PostgreSQLProvider)(nil)

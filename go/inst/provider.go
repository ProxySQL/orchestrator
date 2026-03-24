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

// ReplicationStatus represents database-agnostic replication state.
// Each database provider maps its engine-specific replication details
// into this common structure.
type ReplicationStatus struct {
	// ReplicaRunning indicates whether replication is fully operational
	// (both IO and SQL threads running for MySQL, WAL receiver active for PostgreSQL, etc.)
	ReplicaRunning bool

	// SQLThreadRunning indicates whether the SQL/apply thread is running.
	// For engines without separate threads, this mirrors ReplicaRunning.
	SQLThreadRunning bool

	// IOThreadRunning indicates whether the IO/receiver thread is running.
	// For engines without separate threads, this mirrors ReplicaRunning.
	IOThreadRunning bool

	// Position is an opaque replication position string.
	// For MySQL this is a GTID set or binlog file:pos; for PostgreSQL it would be an LSN.
	Position string

	// Lag is the replication lag in seconds. -1 indicates lag is unknown.
	Lag int64
}

// DatabaseProvider abstracts database-specific operations.
// Implementations exist per database engine (MySQL, future PostgreSQL, etc.).
// This interface covers the minimal set of operations needed for topology
// management in a database-agnostic way.
type DatabaseProvider interface {
	// Discovery

	// GetReplicationStatus retrieves the current replication state
	// for the given instance, returning a database-agnostic status.
	GetReplicationStatus(key InstanceKey) (*ReplicationStatus, error)

	// IsReplicaRunning checks whether replication is actively running
	// on the given instance.
	IsReplicaRunning(key InstanceKey) (bool, error)

	// Read-only control

	// SetReadOnly sets or clears the read-only state on the given instance.
	SetReadOnly(key InstanceKey, readOnly bool) error

	// IsReadOnly checks whether the given instance is in read-only mode.
	IsReadOnly(key InstanceKey) (bool, error)

	// Replication control

	// StartReplication starts the replication process on the given instance.
	StartReplication(key InstanceKey) error

	// StopReplication stops the replication process on the given instance.
	StopReplication(key InstanceKey) error

	// Provider metadata

	// ProviderName returns a short identifier for this provider (e.g. "mysql").
	ProviderName() string
}

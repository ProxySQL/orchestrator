# Database Provider Abstraction

## Overview

Orchestrator supports a database provider abstraction layer that decouples core
orchestration logic from database-specific operations. This allows orchestrator
to manage different database engines through a common interface.

MySQL is the default provider. PostgreSQL is fully supported for streaming
replication topologies, including discovery, failure detection, and automated
failover. The abstraction layer is designed to support additional providers in
the future.

## Architecture

The provider system consists of three components:

1. **`DatabaseProvider` interface** (`go/inst/provider.go`) -- defines the
   contract that all database providers must implement.
2. **Provider implementations** (e.g., `go/inst/provider_mysql.go`) -- concrete
   implementations for specific database engines.
3. **Provider registry** (`go/inst/provider_registry.go`) -- a global registry
   that holds the active provider instance.

## The DatabaseProvider Interface

```go
type DatabaseProvider interface {
    // Discovery
    GetReplicationStatus(key InstanceKey) (*ReplicationStatus, error)
    IsReplicaRunning(key InstanceKey) (bool, error)

    // Read-only control
    SetReadOnly(key InstanceKey, readOnly bool) error
    IsReadOnly(key InstanceKey) (bool, error)

    // Replication control
    StartReplication(key InstanceKey) error
    StopReplication(key InstanceKey) error

    // Provider metadata
    ProviderName() string
}
```

### ReplicationStatus

The `ReplicationStatus` struct provides a database-agnostic view of replication
state:

| Field            | Description                                                  |
|------------------|--------------------------------------------------------------|
| ReplicaRunning   | Whether replication is fully operational                     |
| SQLThreadRunning | Whether the SQL/apply thread is running                      |
| IOThreadRunning  | Whether the IO/receiver thread is running                    |
| Position         | Opaque replication position (MySQL GTID, PG LSN, etc.)      |
| Lag              | Replication lag in seconds; -1 if unknown                    |

## Using the Provider

```go
import "github.com/proxysql/orchestrator/go/inst"

// Get the current provider
provider := inst.GetProvider()

// Check replication status
status, err := provider.GetReplicationStatus(instanceKey)

// Control read-only mode
err = provider.SetReadOnly(instanceKey, true)

// Control replication
err = provider.StopReplication(instanceKey)
err = provider.StartReplication(instanceKey)
```

## MySQL Provider

The MySQL provider (`MySQLProvider`) is the default provider. It delegates to
orchestrator's existing MySQL DAO functions, so all current behavior is
preserved.

The MySQL provider is automatically registered at init time. No configuration
is needed to use it.

## PostgreSQL Provider

The PostgreSQL provider (`PostgreSQLProvider`) supports PostgreSQL streaming
replication topologies. It uses the `lib/pq` driver to connect to PostgreSQL
instances and provides full support for discovery, failure detection, and
automated failover.

### Switching to PostgreSQL Mode

Set `ProviderType` to `"postgresql"` in your orchestrator configuration:

```json
{
  "ProviderType": "postgresql",
  "PostgreSQLTopologyUser": "orchestrator",
  "PostgreSQLTopologyPassword": "secret",
  "PostgreSQLSSLMode": "require",
  "DefaultInstancePort": 5432
}
```

When `ProviderType` is set to `"postgresql"`, orchestrator automatically uses
the PostgreSQL provider for all topology operations: discovery, failure
analysis, and recovery.

### Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `ProviderType` | string | `"mysql"` | Set to `"postgresql"` to enable PostgreSQL mode |
| `PostgreSQLTopologyUser` | string | `""` | Username for connecting to PostgreSQL instances |
| `PostgreSQLTopologyPassword` | string | `""` | Password for connecting to PostgreSQL instances |
| `PostgreSQLSSLMode` | string | `"require"` | SSL mode: `disable`, `require`, `verify-ca`, `verify-full` |

The orchestrator user on PostgreSQL needs the `pg_monitor` role:

```sql
CREATE USER orchestrator WITH PASSWORD 'secret';
GRANT pg_monitor TO orchestrator;
```

### Supported Operations

| Operation | PostgreSQL Implementation |
|-----------|--------------------------|
| **Discovery (primary)** | Queries `pg_current_wal_lsn()` for WAL position and `pg_stat_replication` to discover connected standbys. |
| **Discovery (standby)** | Queries `pg_stat_wal_receiver` for WAL receiver status and `pg_last_wal_replay_lsn()` for replay position. Extracts primary host/port from `conninfo`. |
| **Replication lag** | Computes `EXTRACT(EPOCH FROM now() - pg_last_xact_replay_timestamp())` on standbys. |
| **Failure detection** | Analyzes reachability of primary and replication status of standbys. Produces `DeadPrimary`, `DeadPrimaryAndSomeStandbys`, `StandbyNotReplicating`, `AllStandbyNotReplicating`, and `UnreachablePrimary` analysis codes. |
| **Promotion** | Calls `pg_promote(true, 60)` on the selected standby and waits up to 30 seconds for it to exit recovery mode. |
| **Standby reconfiguration** | Updates `primary_conninfo` via `ALTER SYSTEM`, calls `pg_reload_conf()`, and pauses/resumes WAL replay to force reconnection. |
| **Best standby selection** | Prefers standbys with: valid last check, replication running, lowest lag, highest WAL LSN, not downtimed. A candidate key is preferred if specified and valid. |
| GetReplicationStatus | Queries `pg_stat_wal_receiver` (standby) or `pg_current_wal_lsn()` (primary). Reports WAL LSN as position and `replay_lag` as lag. |
| IsReplicaRunning | Checks `pg_stat_wal_receiver` for an active WAL receiver with `status = 'streaming'`. |
| SetReadOnly | Runs `ALTER SYSTEM SET default_transaction_read_only = on/off` followed by `SELECT pg_reload_conf()`. |
| IsReadOnly | Queries `SHOW default_transaction_read_only`. |
| StartReplication | Calls `SELECT pg_wal_replay_resume()`. Streaming replication itself starts automatically when the standby connects. |
| StopReplication | Calls `SELECT pg_wal_replay_pause()` to pause WAL replay. The WAL receiver remains connected. |

### Differences from MySQL Mode

- **No separate IO/SQL threads.** PostgreSQL does not have the concept of
  separate IO and SQL threads. The WAL receiver handles both receiving and
  applying. Orchestrator maps the WAL receiver status to both the IO and SQL
  thread fields.
- **No intermediate masters.** PostgreSQL streaming replication uses a flat
  primary-standby topology. There is no equivalent of MySQL intermediate masters.
  Orchestrator treats all standbys as direct replicas of the primary.
- **WAL-based positioning.** PostgreSQL uses WAL LSN (Log Sequence Number)
  instead of binlog file:position or GTIDs. Orchestrator converts LSN to an
  int64 for internal use.
- **Streaming replication is automatic.** `StartReplication` resumes WAL replay
  but cannot start the WAL receiver itself -- that is controlled by PostgreSQL's
  `primary_conninfo` configuration.
- **StopReplication pauses replay only.** The WAL receiver continues to receive
  WAL segments; only application (replay) is paused.
- **Promotion uses `pg_promote()`.** Available since PostgreSQL 12. Orchestrator
  calls `pg_promote(true, 60)` which waits for promotion to complete.
- **Reconfiguration uses `ALTER SYSTEM`.** To repoint a standby, orchestrator
  updates `primary_conninfo` via `ALTER SYSTEM SET` and reloads the config.
- **No topology refactoring.** Moving replicas between masters (drag-and-drop in
  the UI) is not supported in PostgreSQL mode. Only failover and standby
  reconfiguration are available.
- **No ProxySQL integration.** ProxySQL hooks are MySQL-specific. Use PgBouncer
  or another PostgreSQL-aware connection pooler.

### Limitations and Known Issues

- **Cascading replication** (standby replicating from another standby) is not
  currently detected or managed.
- **Logical replication** is not supported -- only physical streaming replication.
- **Synchronous replication** settings are not managed by orchestrator. If you
  use `synchronous_standby_names`, you must manage it separately.
- **`client_port` from `pg_stat_replication`** is an ephemeral port, not the
  PostgreSQL listen port. Orchestrator uses `DefaultInstancePort` for standby
  discovery, so all instances must listen on the same port.
- **Graceful master takeover** (planned switchover) is supported for PostgreSQL.
  The same CLI commands and API endpoints (`graceful-master-takeover`,
  `graceful-master-takeover-auto`) work for PostgreSQL clusters. The switchover
  sets the primary read-only, waits for the designated standby to catch up via
  WAL LSN, promotes the standby, reconfigures remaining standbys, and configures
  the demoted primary's `primary_conninfo`. The demoted primary requires an
  operator-managed restart with `standby.signal` — use
  `PostGracefulTakeoverProcesses` hooks to automate this step.

## Implementing a New Provider

To add support for a new database engine:

1. Create a new file `go/inst/provider_<engine>.go`.
2. Define a struct that implements all methods of `DatabaseProvider`.
3. Add a compile-time interface check:
   ```go
   var _ DatabaseProvider = (*MyNewProvider)(nil)
   ```
4. Register the provider during initialization or based on configuration:
   ```go
   inst.SetProvider(NewMyProvider())
   ```

### Guidelines

- **Return errors, don't panic.** All provider methods return errors.
- **Map engine-specific state to ReplicationStatus.** The `ReplicationStatus`
  struct is intentionally generic. Map your engine's replication details
  into the common fields.
- **Position is opaque.** The `Position` field in `ReplicationStatus` is a
  string that means different things for different engines. Consumers should
  not parse it directly.
- **Lag of -1 means unknown.** If your engine cannot determine replication lag,
  return -1.

## Current Limitations

This is the initial extraction. The provider interface currently covers:

- Replication status discovery
- Read-only control
- Basic replication start/stop

Future work will expand the interface to cover:

- Topology changes (reparenting, detach/reattach)
- GTID operations
- Semi-sync configuration
- Instance discovery and metadata

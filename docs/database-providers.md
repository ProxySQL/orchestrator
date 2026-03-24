# Database Provider Abstraction

## Overview

Orchestrator supports a database provider abstraction layer that decouples core
orchestration logic from database-specific operations. This allows orchestrator
to manage different database engines through a common interface.

MySQL is the default (and currently only) provider. The abstraction layer is
designed to support future providers such as PostgreSQL.

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

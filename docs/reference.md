# Orchestrator Reference Manual

Complete reference for orchestrator configuration, CLI commands, and HTTP API endpoints.

**Source of truth:** This document is generated from the orchestrator source code. Configuration fields come from the `Configuration` struct in `go/config/config.go`. CLI commands come from `go/app/cli.go`. API endpoints come from `go/http/api.go` and `go/http/apiv2.go`.

---

## Table of Contents

- [1. Configuration Reference](#1-configuration-reference)
  - [1.1 General / Debug](#11-general--debug)
  - [1.2 HTTP / Network](#12-http--network)
  - [1.3 MySQL Topology Credentials](#13-mysql-topology-credentials)
  - [1.4 MySQL Topology TLS](#14-mysql-topology-tls)
  - [1.5 PostgreSQL Topology](#15-postgresql-topology)
  - [1.6 Orchestrator Backend Database](#16-orchestrator-backend-database)
  - [1.7 Orchestrator Backend TLS](#17-orchestrator-backend-tls)
  - [1.8 MySQL Connection Timeouts](#18-mysql-connection-timeouts)
  - [1.9 Raft Consensus](#19-raft-consensus)
  - [1.10 Discovery](#110-discovery)
  - [1.11 Instance Polling and Buffering](#111-instance-polling-and-buffering)
  - [1.12 Hostname Resolution](#112-hostname-resolution)
  - [1.13 Replication Lag and Checks](#113-replication-lag-and-checks)
  - [1.14 Cluster Classification and Detection](#114-cluster-classification-and-detection)
  - [1.15 Pseudo-GTID](#115-pseudo-gtid)
  - [1.16 Binlog Analysis](#116-binlog-analysis)
  - [1.17 Failure Detection and Recovery](#117-failure-detection-and-recovery)
  - [1.18 Recovery Hook Processes](#118-recovery-hook-processes)
  - [1.19 Master Failover Behavior](#119-master-failover-behavior)
  - [1.20 Semi-Sync](#120-semi-sync)
  - [1.21 Authentication and Security](#121-authentication-and-security)
  - [1.22 SSL / TLS (Web)](#122-ssl--tls-web)
  - [1.23 Agents](#123-agents)
  - [1.24 Agent TLS](#124-agent-tls)
  - [1.25 Audit](#125-audit)
  - [1.26 Pools](#126-pools)
  - [1.27 Promotion and Filters](#127-promotion-and-filters)
  - [1.28 Consul / ZooKeeper / KV Stores](#128-consul--zookeeper--kv-stores)
  - [1.29 Graphite](#129-graphite)
  - [1.30 ProxySQL](#130-proxysql)
  - [1.31 Prometheus](#131-prometheus)
  - [1.32 Web UI / Miscellaneous](#132-web-ui--miscellaneous)
- [2. CLI Command Reference](#2-cli-command-reference)
  - [2.1 Smart Relocation](#21-smart-relocation)
  - [2.2 Classic file:pos Relocation](#22-classic-filepos-relocation)
  - [2.3 Binlog Server Relocation](#23-binlog-server-relocation)
  - [2.4 GTID Relocation](#24-gtid-relocation)
  - [2.5 Pseudo-GTID Relocation](#25-pseudo-gtid-relocation)
  - [2.6 Replication, General](#26-replication-general)
  - [2.7 Replication Information](#27-replication-information)
  - [2.8 Instance](#28-instance)
  - [2.9 Binary Logs](#29-binary-logs)
  - [2.10 Pools](#210-pools)
  - [2.11 Information](#211-information)
  - [2.12 Key-Value Stores](#212-key-value-stores)
  - [2.13 Tags](#213-tags)
  - [2.14 Instance Management](#214-instance-management)
  - [2.15 Recovery](#215-recovery)
  - [2.16 Instance Meta](#216-instance-meta)
  - [2.17 Meta / Orchestrator Operations](#217-meta--orchestrator-operations)
  - [2.18 Global Recoveries](#218-global-recoveries)
  - [2.19 Bulk Operations](#219-bulk-operations)
  - [2.20 ProxySQL](#220-proxysql)
  - [2.21 Agent](#221-agent)
- [3. API v1 Reference](#3-api-v1-reference)
  - [3.1 Smart Relocation](#31-smart-relocation)
  - [3.2 Classic file:pos Relocation](#32-classic-filepos-relocation)
  - [3.3 Binlog Server](#33-binlog-server)
  - [3.4 GTID Relocation](#34-gtid-relocation)
  - [3.5 Pseudo-GTID Relocation](#35-pseudo-gtid-relocation)
  - [3.6 Topology / Promotion](#36-topology--promotion)
  - [3.7 Replication Control](#37-replication-control)
  - [3.8 Replication Information](#38-replication-information)
  - [3.9 Instance Control](#39-instance-control)
  - [3.10 Binary Logs](#310-binary-logs)
  - [3.11 Pools](#311-pools)
  - [3.12 Search and Discovery](#312-search-and-discovery)
  - [3.13 Cluster Information](#313-cluster-information)
  - [3.14 Tags](#314-tags)
  - [3.15 Instance Management](#315-instance-management)
  - [3.16 Recovery and Analysis](#316-recovery-and-analysis)
  - [3.17 Problems and Audit](#317-problems-and-audit)
  - [3.18 Health and Raft](#318-health-and-raft)
  - [3.19 Hostname and Configuration](#319-hostname-and-configuration)
  - [3.20 Bulk Operations](#320-bulk-operations)
  - [3.21 Discovery Metrics](#321-discovery-metrics)
  - [3.22 Agents](#322-agents)
  - [3.23 ProxySQL](#323-proxysql)
- [4. API v2 Reference](#4-api-v2-reference)
- [5. ProxySQL Configuration](#5-proxysql-configuration)
- [6. Observability](#6-observability)

---

## 1. Configuration Reference

Orchestrator is configured via a JSON file. All fields belong to the `Configuration` struct in `go/config/config.go`. Passwords support environment variable substitution in the form `${ENV_VAR_NAME}`.

### 1.1 General / Debug

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Debug` | bool | `false` | Set debug mode (similar to `--debug` option) |
| `EnableSyslog` | bool | `false` | Should logs be directed (in addition) to syslog daemon? |
| `ProviderType` | string | `"mysql"` | Database provider type: `"mysql"` (default) or `"postgresql"`. When set to `"postgresql"`, orchestrator uses PostgreSQL-specific discovery, failure detection, and recovery logic. See [Database Providers](database-providers.md) for details. |

### 1.2 HTTP / Network

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `ListenAddress` | string | `":3000"` | Where orchestrator HTTP should listen for TCP |
| `ListenSocket` | string | `""` | Where orchestrator HTTP should listen for unix socket (when given, TCP is disabled) |
| `HTTPAdvertise` | string | `""` | For raft setups, HTTP address this node advertises to peers. Must include scheme, host, and port (e.g. `http://11.22.33.44:3030`). Must not include a path. |
| `AgentsServerPort` | string | `":3001"` | Port orchestrator agents talk back to |
| `URLPrefix` | string | `""` | URL prefix to run orchestrator on non-root web path, e.g. `/orchestrator` for running behind nginx |
| `StatusEndpoint` | string | `"/api/status"` | Override the status endpoint |
| `StatusOUVerify` | bool | `false` | If true, try to verify OUs when Mutual TLS is on |

### 1.3 MySQL Topology Credentials

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `MySQLTopologyUser` | string | `""` | Username for connecting to topology MySQL instances |
| `MySQLTopologyPassword` | string | `""` | Password for connecting to topology MySQL instances. Supports `${ENV_VAR}` syntax. |
| `MySQLTopologyCredentialsConfigFile` | string | `""` | my.cnf-style config file for topology credentials. Reads `user` and `password` from the `[client]` section. |
| `MySQLTopologyMaxAllowedPacket` | int32 | `-1` | `max_allowed_packet` value when connecting to topology instances |

### 1.4 MySQL Topology TLS

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `MySQLTopologySSLPrivateKeyFile` | string | `""` | Private key file for TLS authentication with topology instances |
| `MySQLTopologySSLCertFile` | string | `""` | Certificate PEM file for TLS authentication with topology instances |
| `MySQLTopologySSLCAFile` | string | `""` | Certificate Authority PEM file for topology instance TLS |
| `MySQLTopologySSLSkipVerify` | bool | `false` | If true, do not strictly validate mutual TLS certs for topology instances |
| `MySQLTopologyUseMutualTLS` | bool | `false` | Turn on TLS authentication with the topology MySQL instances |
| `MySQLTopologyUseMixedTLS` | bool | `true` | Mixed TLS and non-TLS authentication with topology instances |
| `TLSCacheTTLFactor` | uint | `100` | Factor of `InstancePollSeconds` used as TLS info cache expiry |

### 1.5 PostgreSQL Topology

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `PostgreSQLTopologyUser` | string | `""` | Username for connecting to PostgreSQL topology instances |
| `PostgreSQLTopologyPassword` | string | `""` | Password for connecting to PostgreSQL topology instances |
| `PostgreSQLSSLMode` | string | `"require"` | SSL mode for PostgreSQL connections: `disable`, `require`, `verify-ca`, `verify-full` |

### 1.6 Orchestrator Backend Database

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `BackendDB` | string | `"mysql"` | EXPERIMENTAL: type of backend db; either `"mysql"` or `"sqlite3"` |
| `SQLite3DataFile` | string | `""` | Full path to sqlite3 data file (required when `BackendDB` is `"sqlite3"`) |
| `SkipOrchestratorDatabaseUpdate` | bool | `false` | When true, do not check backend schema nor attempt to update it. Useful when running multiple orchestrator versions. |
| `PanicIfDifferentDatabaseDeploy` | bool | `false` | When true, panic if backend DB was provisioned by a different version |
| `MySQLOrchestratorHost` | string | `""` | Hostname of the orchestrator backend MySQL instance |
| `MySQLOrchestratorPort` | uint | `3306` | Port of the orchestrator backend MySQL instance |
| `MySQLOrchestratorDatabase` | string | `""` | Database name for orchestrator backend |
| `MySQLOrchestratorUser` | string | `""` | Username for orchestrator backend MySQL |
| `MySQLOrchestratorPassword` | string | `""` | Password for orchestrator backend MySQL. Supports `${ENV_VAR}` syntax. |
| `MySQLOrchestratorCredentialsConfigFile` | string | `""` | my.cnf-style config file for backend credentials. Reads `user` and `password` from `[client]`. |
| `MySQLOrchestratorMaxPoolConnections` | int | `128` | Maximum size of the connection pool to the backend DB |
| `MySQLOrchestratorReadTimeoutSeconds` | int | `30` | Seconds before backend MySQL read operation is aborted (driver-side) |
| `MySQLOrchestratorRejectReadOnly` | bool | `false` | Reject read-only connections to backend |
| `MySQLOrchestratorMaxAllowedPacket` | int32 | `-1` | `max_allowed_packet` for backend MySQL connections |

### 1.7 Orchestrator Backend TLS

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `MySQLOrchestratorSSLPrivateKeyFile` | string | `""` | Private key file for TLS with backend MySQL |
| `MySQLOrchestratorSSLCertFile` | string | `""` | Certificate PEM file for TLS with backend MySQL |
| `MySQLOrchestratorSSLCAFile` | string | `""` | Certificate Authority PEM file for backend MySQL TLS |
| `MySQLOrchestratorSSLSkipVerify` | bool | `false` | Skip strict validation of mutual TLS certs for backend |
| `MySQLOrchestratorUseMutualTLS` | bool | `false` | Turn on TLS authentication with the backend MySQL instance |

### 1.8 MySQL Connection Timeouts

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `MySQLConnectTimeoutSeconds` | int | `2` | Seconds before connection is aborted (driver-side) |
| `MySQLDiscoveryReadTimeoutSeconds` | int | `10` | Seconds before topology read for discovery is aborted |
| `MySQLTopologyReadTimeoutSeconds` | int | `600` | Seconds before topology read (non-discovery) is aborted |
| `MySQLConnectionLifetimeSeconds` | int | `0` | Seconds the driver keeps a connection alive before recycling. 0 means unlimited. |
| `DefaultInstancePort` | int | `3306` | Default port when not specified on command line |

### 1.9 Raft Consensus

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `RaftEnabled` | bool | `false` | When true, set up orchestrator in a raft consensus layout. When false, all `Raft*` variables are ignored. |
| `RaftBind` | string | `"127.0.0.1:10008"` | Address to bind for raft communication |
| `RaftAdvertise` | string | `""` | Address to advertise for raft. Defaults to `RaftBind` if empty. |
| `RaftDataDir` | string | `""` | Directory for raft data storage (required when `RaftEnabled` is true) |
| `DefaultRaftPort` | int | `10008` | Default port for `RaftNodes` entries that don't specify a port |
| `RaftNodes` | []string | `[]` | Raft nodes to make initial connection with |
| `ExpectFailureAnalysisConcensus` | bool | `true` | Expect failure analysis consensus before recovery |

### 1.10 Discovery

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `DiscoverByShowSlaveHosts` | bool | `false` | Attempt `SHOW SLAVE HOSTS` before `PROCESSLIST` |
| `DiscoveryMaxConcurrency` | uint | `300` | Number of goroutines doing host discovery |
| `DiscoveryQueueCapacity` | uint | `100000` | Buffer size of the discovery queue. Should be greater than the number of DB instances. |
| `DiscoveryQueueMaxStatisticsSize` | int | `120` | Maximum number of individual per-second statistics kept for the discovery queue |
| `DiscoveryCollectionRetentionSeconds` | uint | `120` | Seconds to retain discovery collection information |
| `DiscoverySeeds` | []string | `[]` | Hard-coded array of `hostname:port` ensuring orchestrator discovers these on startup |
| `DiscoveryIgnoreReplicaHostnameFilters` | []string | `[]` | Regexp filters to prevent auto-discovering new replicas |
| `DiscoveryIgnoreMasterHostnameFilters` | []string | `[]` | Regexp filters to prevent auto-discovering a master |
| `DiscoveryIgnoreHostnameFilters` | []string | `[]` | Regexp filters to prevent discovering instances of any kind |
| `DiscoveryIgnoreReplicationUsernameFilters` | []string | `[]` | Regexp filters to prevent discovering instances with a matching replication username |
| `EnableDiscoveryFiltersLogs` | bool | `true` | Log filtered instances during discovery |
| `UnseenInstanceForgetHours` | uint | `240` | Hours after which an unseen instance is forgotten |
| `SnapshotTopologiesIntervalHours` | uint | `0` | Interval in hours between snapshot-topologies invocations. 0 disables. |

### 1.11 Instance Polling and Buffering

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `InstancePollSeconds` | uint | `5` | Seconds between instance reads |
| `DeadInstancePollSecondsMultiplyFactor` | float32 | `1` | Multiply factor for dead instance poll interval. Must be >= 1. |
| `DeadInstancePollSecondsMax` | uint | `300` | Maximum delay between dead instance read attempts |
| `DeadInstanceDiscoveryMaxConcurrency` | uint | `0` | Number of goroutines doing dead host discovery. 0 means unlimited. |
| `DeadInstanceDiscoveryLogsEnabled` | bool | `false` | Enable logs for dead instance discoveries |
| `ReasonableInstanceCheckSeconds` | uint | `1` | Seconds an instance read is allowed to take before `LastCheckValid` becomes false |
| `InstanceWriteBufferSize` | int | `100` | Max number of instances to flush in one `INSERT ... ON DUPLICATE KEY UPDATE` |
| `BufferInstanceWrites` | bool | `false` | Set to true for write-optimization on backend table (writes can be stale and overwrite non-stale data) |
| `InstanceFlushIntervalMilliseconds` | int | `100` | Max interval between instance write buffer flushes |
| `InstanceBulkOperationsWaitTimeoutSeconds` | uint | `10` | Time to wait on a single instance during bulk operations |
| `SkipMaxScaleCheck` | bool | `true` | Skip MaxScale BinlogServer checks. Set to true if you never have MaxScale in your topology. |
| `LowerReplicaVersionAllowed` | bool | `false` | Allow lower version replica to replicate from higher version master (produces a warning) |
| `UseSuperReadOnly` | bool | `false` | Should orchestrator use `super_read_only` any time it sets `read_only` |
| `MaxConcurrentReplicaOperations` | int | `5` | Maximum number of concurrent operations on replicas |

### 1.12 Hostname Resolution

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `HostnameResolveMethod` | string | `"default"` | Method to normalize hostname: `"none"`, `"default"`, `"cname"` |
| `MySQLHostnameResolveMethod` | string | `"@@hostname"` | Method to normalize hostname via MySQL: `"none"`, `"@@hostname"`, `"@@report_host"` |
| `SkipBinlogServerUnresolveCheck` | bool | `true` | Skip the double-check that an unresolved hostname resolves back for binlog servers |
| `ExpiryHostnameResolvesMinutes` | int | `60` | Minutes after which hostname resolves expire |
| `RejectHostnameResolvePattern` | string | `""` | Regexp pattern for resolved hostnames that will be rejected (not cached, not written to db) |

### 1.13 Replication Lag and Checks

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `SlaveLagQuery` | string | `""` | Synonym for `ReplicationLagQuery` (deprecated, use `ReplicationLagQuery`) |
| `ReplicationLagQuery` | string | `""` | Custom query to check replica lag (e.g. heartbeat table). Must return one row, one numeric column. |
| `ReplicationCredentialsQuery` | string | `""` | Custom query returning replication credentials: username, password, SSLCaCert, SSLCert, SSLKey. Optional. |
| `ReasonableReplicationLagSeconds` | int | `10` | Above this value, replication lag is considered a problem |
| `ReasonableMaintenanceReplicationLagSeconds` | int | `20` | Above this value, move-up and move-below are blocked |
| `ProblemIgnoreHostnameFilters` | []string | `[]` | Regexp filters to minimize problem visualization for matching hostnames |
| `VerifyReplicationFilters` | bool | `false` | Include replication filters check before approving topology refactoring |
| `ReduceReplicationAnalysisCount` | bool | `true` | When true, analysis only reports instances where problems are possible (skips most leaf nodes) |

### 1.14 Cluster Classification and Detection

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `ClusterNameToAlias` | map[string]string | `{}` | Map between regex matching cluster name to a human-friendly alias |
| `DetectClusterAliasQuery` | string | `""` | Optional query (on topology instance) that returns the alias of a cluster. Executed on master only. Must return one row, one column. |
| `DetectClusterDomainQuery` | string | `""` | Optional query (on topology instance) that returns the VIP/CNAME/domain for the cluster master. Must return one row, one column. |
| `DetectInstanceAliasQuery` | string | `""` | Optional query (on topology instance) that returns the alias of an instance. Must return one row, one column. |
| `DetectPromotionRuleQuery` | string | `""` | Optional query (on topology instance) that returns the promotion rule of an instance. Must return one row, one column. |
| `DataCenterPattern` | string | `""` | Regexp with one group, extracting datacenter name from hostname |
| `RegionPattern` | string | `""` | Regexp with one group, extracting region name from hostname |
| `PhysicalEnvironmentPattern` | string | `""` | Regexp with one group, extracting physical environment from hostname |
| `DetectDataCenterQuery` | string | `""` | Optional query returning the data center of an instance. Overrides `DataCenterPattern`. |
| `DetectRegionQuery` | string | `""` | Optional query returning the region of an instance. Overrides `RegionPattern`. |
| `DetectPhysicalEnvironmentQuery` | string | `""` | Optional query returning the physical environment of an instance. Overrides `PhysicalEnvironmentPattern`. |
| `DetectSemiSyncEnforcedQuery` | string | `""` | Optional query to determine whether semi-sync is fully enforced. Must return 0 or 1. |
| `RemoveTextFromHostnameDisplay` | string | `""` | Text to strip from hostname on cluster/clusters pages |
| `ReadOnly` | bool | `false` | When true, orchestrator operates in read-only mode |

### 1.15 Pseudo-GTID

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `AutoPseudoGTID` | bool | `false` | Should orchestrator automatically inject Pseudo-GTID entries to masters. When true, overrides `PseudoGTIDPattern` and related settings. |
| `PseudoGTIDPattern` | string | `""` | Pattern to look for in binlogs as a unique entry. When empty, Pseudo-GTID refactoring is disabled. |
| `PseudoGTIDPatternIsFixedSubstring` | bool | `false` | If true, `PseudoGTIDPattern` is a fixed substring (not regex), boosting search time |
| `PseudoGTIDMonotonicHint` | string | `""` | Substring in Pseudo-GTID entry indicating entries are monotonically increasing |
| `DetectPseudoGTIDQuery` | string | `""` | Optional query to authoritatively decide whether Pseudo-GTID is enabled on an instance |

### 1.16 Binlog Analysis

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `BinlogEventsChunkSize` | int | `10000` | Chunk size (X) for `SHOW BINLOG EVENTS LIMIT ?,X`. Smaller means less locking, more work. |
| `SkipBinlogEventsContaining` | []string | `[]` | When scanning binlogs for Pseudo-GTID, skip entries containing these substrings (not regex). |

### 1.17 Failure Detection and Recovery

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `FailureDetectionPeriodBlockMinutes` | int | `60` | Time (minutes) an instance's failure discovery is kept active, preventing concurrent discoveries |
| `RecoveryPeriodBlockMinutes` | int | `60` | (Deprecated: use `RecoveryPeriodBlockSeconds`) Time for which a recovery is kept active |
| `RecoveryPeriodBlockSeconds` | int | `3600` | Overrides `RecoveryPeriodBlockMinutes`. Time for which a recovery is kept active. |
| `RecoveryIgnoreHostnameFilters` | []string | `[]` | Recovery analysis completely ignores hosts matching these patterns |
| `RecoverMasterClusterFilters` | []string | `[]` | Only do master recovery on clusters matching these regexp patterns (`".*"` matches all) |
| `RecoverIntermediateMasterClusterFilters` | []string | `[]` | Only do intermediate-master recovery on clusters matching these patterns |
| `RecoverNonWriteableMaster` | bool | `false` | When true, treat a read-only master as a failure scenario and attempt to make it writeable |

### 1.18 Recovery Hook Processes

All process hook fields accept a list of shell commands. Placeholders available: `{failureType}`, `{instanceType}`, `{isMaster}`, `{isCoMaster}`, `{failureDescription}`, `{command}`, `{failedHost}`, `{failureCluster}`, `{failureClusterAlias}`, `{failureClusterDomain}`, `{failedPort}`, `{successorHost}`, `{successorPort}`, `{successorAlias}`, `{successorBinlogCoordinates}`, `{countReplicas}`, `{replicaHosts}`, `{isDowntimed}`, `{autoMasterRecovery}`, `{autoIntermediateMasterRecovery}`, `{isSuccessful}`, `{lostReplicas}`, `{countLostReplicas}`.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `ProcessesShellCommand` | string | `"bash"` | Shell that executes command scripts |
| `OnFailureDetectionProcesses` | []string | `[]` | Processes to execute when detecting a failover scenario (before deciding whether to failover) |
| `PreGracefulTakeoverProcesses` | []string | `[]` | Processes before a graceful takeover. Non-zero exit aborts the operation. |
| `PreFailoverProcesses` | []string | `[]` | Processes before a failover. Non-zero exit aborts the operation. |
| `PostFailoverProcesses` | []string | `[]` | Processes after a failover |
| `PostUnsuccessfulFailoverProcesses` | []string | `[]` | Processes after a not-completely-successful failover |
| `PostMasterFailoverProcesses` | []string | `[]` | Processes after a master failover |
| `PostIntermediateMasterFailoverProcesses` | []string | `[]` | Processes after an intermediate-master failover |
| `PostGracefulTakeoverProcesses` | []string | `[]` | Processes after a graceful master takeover |
| `PostTakeMasterProcesses` | []string | `[]` | Processes after a successful take-master event |

### 1.19 Master Failover Behavior

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `CoMasterRecoveryMustPromoteOtherCoMaster` | bool | `true` | When true, only the other co-master can be promoted. When false, any instance is eligible. |
| `DetachLostSlavesAfterMasterFailover` | bool | `true` | Synonym for `DetachLostReplicasAfterMasterFailover` |
| `DetachLostReplicasAfterMasterFailover` | bool | `false` | Forcibly detach replicas that were more up-to-date than the promoted replica |
| `ApplyMySQLPromotionAfterMasterFailover` | bool | `true` | Should orchestrator apply MySQL master promotion: `set read_only=0`, detach replication, etc. |
| `PreventCrossDataCenterMasterFailover` | bool | `false` | When true, cross-DC failover is not allowed |
| `PreventCrossRegionMasterFailover` | bool | `false` | When true, cross-region failover is not allowed |
| `MasterFailoverLostInstancesDowntimeMinutes` | uint | `0` | Minutes to downtime servers lost after master failover. 0 disables. |
| `MasterFailoverDetachSlaveMasterHost` | bool | `false` | Synonym for `MasterFailoverDetachReplicaMasterHost` |
| `MasterFailoverDetachReplicaMasterHost` | bool | `false` | Issue detach-replica-master-host on newly promoted master. Meaningless if `ApplyMySQLPromotionAfterMasterFailover` is true. |
| `FailMasterPromotionOnLagMinutes` | uint | `0` | Fail master promotion if candidate replica is lagging >= this many minutes. Requires `ReplicationLagQuery`. |
| `FailMasterPromotionIfSQLThreadNotUpToDate` | bool | `false` | Abort promotion if candidate has not consumed all relay logs. Cannot be true with `DelayMasterPromotionIfSQLThreadNotUpToDate`. |
| `DelayMasterPromotionIfSQLThreadNotUpToDate` | bool | `false` | Delay promotion until SQL thread catches up. Cannot be true with `FailMasterPromotionIfSQLThreadNotUpToDate`. |
| `PostponeSlaveRecoveryOnLagMinutes` | uint | `0` | Synonym for `PostponeReplicaRecoveryOnLagMinutes` |
| `PostponeReplicaRecoveryOnLagMinutes` | uint | `0` | On crash recovery, lagging replicas are resurrected late, after master/IM election. 0 disables. |

### 1.20 Semi-Sync

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `EnforceExactSemiSyncReplicas` | bool | `false` | If true, semi-sync replicas are enabled/disabled to match the wait count in priority order |
| `RecoverLockedSemiSyncMaster` | bool | `false` | If true, recover from `LockedSemiSync` state by enabling semi-sync on replicas |
| `ReasonableLockedSemiSyncMasterSeconds` | uint | `0` | Time to evaluate the LockedSemiSync hypothesis. Falls back to `ReasonableReplicationLagSeconds` if 0. |

### 1.21 Authentication and Security

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `AuthenticationMethod` | string | `""` | Type of authentication: `""` (none), `"basic"`, `"multi"`, `"proxy"`, `"token"` |
| `HTTPAuthUser` | string | `""` | Username for HTTP Basic authentication (blank disables) |
| `HTTPAuthPassword` | string | `""` | Password for HTTP Basic authentication |
| `AuthUserHeader` | string | `"X-Forwarded-User"` | HTTP header indicating auth user when `AuthenticationMethod` is `"proxy"` |
| `PowerAuthUsers` | []string | `["*"]` | On `AuthenticationMethod == "proxy"`, list of users that can make changes. All others are read-only. |
| `PowerAuthGroups` | []string | `[]` | Unix groups the authenticated user must belong to for write access |
| `AccessTokenUseExpirySeconds` | uint | `60` | Time by which an issued token must be used |
| `AccessTokenExpiryMinutes` | uint | `1440` | Time after which HTTP access token expires |
| `OAuthClientId` | string | `""` | OAuth client ID |
| `OAuthClientSecret` | string | `""` | OAuth client secret |
| `OAuthScopes` | []string | `nil` | OAuth scopes |

### 1.22 SSL / TLS (Web)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `UseSSL` | bool | `false` | Use SSL on the server web port |
| `UseMutualTLS` | bool | `false` | Use mutual TLS for web and API connections |
| `SSLSkipVerify` | bool | `false` | Ignore SSL certification errors |
| `SSLPrivateKeyFile` | string | `""` | SSL private key file |
| `SSLCertFile` | string | `""` | SSL certificate file |
| `SSLCAFile` | string | `""` | Certificate Authority file |
| `SSLValidOUs` | []string | `[]` | Valid organizational units for mutual TLS |

### 1.23 Agents

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `ServeAgentsHttp` | bool | `false` | Spawn another HTTP interface dedicated for orchestrator-agent |
| `AgentPollMinutes` | uint | `60` | Minutes between agent polling |
| `UnseenAgentForgetHours` | uint | `6` | Hours after which an unseen agent is forgotten |
| `StaleSeedFailMinutes` | uint | `60` | Minutes after which a stale (no progress) seed is considered failed |
| `SeedAcceptableBytesDiff` | int64 | `8192` | Acceptable byte difference between seed source and target data |
| `SeedWaitSecondsBeforeSend` | int64 | `2` | Seconds to wait before starting send command on agent |

### 1.24 Agent TLS

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `AgentsUseSSL` | bool | `false` | Listen on agents port with SSL and connect to agents via SSL |
| `AgentsUseMutualTLS` | bool | `false` | Use mutual TLS for server-to-agent communication |
| `AgentSSLSkipVerify` | bool | `false` | Ignore SSL certification errors for agents |
| `AgentSSLPrivateKeyFile` | string | `""` | Agent SSL private key file |
| `AgentSSLCertFile` | string | `""` | Agent SSL certificate file |
| `AgentSSLCAFile` | string | `""` | Agent Certificate Authority file |
| `AgentSSLValidOUs` | []string | `[]` | Valid organizational units for mutual TLS with agents |

### 1.25 Audit

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `AuditLogFile` | string | `""` | Name of log file for audit operations. Empty disables file logging. |
| `AuditToSyslog` | bool | `false` | Write audit messages to syslog |
| `AuditToBackendDB` | bool | `false` | Write audit messages to the backend DB's `audit` table |
| `AuditPurgeDays` | uint | `7` | Days after which audit entries are purged from the database |

### 1.26 Pools

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `SupportFuzzyPoolHostnames` | bool | `true` | Allow `submit-pool-instances` to pass fuzzy (non-FQDN) instance names. Implies more backend queries. |
| `InstancePoolExpiryMinutes` | uint | `60` | Minutes after which `database_instance_pool` entries expire |
| `CandidateInstanceExpireMinutes` | uint | `60` | Minutes after which a candidate replica suggestion expires |

### 1.27 Promotion and Filters

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `PromotionIgnoreHostnameFilters` | []string | `[]` | Orchestrator will not promote replicas with hostnames matching these patterns |
| `OSCIgnoreHostnameFilters` | []string | `[]` | OSC replica recommendation will ignore matching hostnames |

### 1.28 Consul / ZooKeeper / KV Stores

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `ConsulAddress` | string | `""` | Address of Consul HTTP API (e.g. `127.0.0.1:8500`) |
| `ConsulScheme` | string | `"http"` | Scheme for Consul: `http` or `https` |
| `ConsulAclToken` | string | `""` | ACL token for writing to Consul KV |
| `ConsulCrossDataCenterDistribution` | bool | `false` | Auto-deduce all Consul DCs and write KVs in all DCs |
| `ConsulKVStoreProvider` | string | `"consul"` | Consul KV store provider: `"consul"` or `"consul-txn"` |
| `ConsulMaxKVsPerTransaction` | int | `5` | Maximum KV operations per single Consul Transaction. Requires `"consul-txn"` provider. Range: 5-64. |
| `ZkAddress` | string | `""` | UNSUPPORTED YET. ZooKeeper server addresses in `srv1[:port1][,srv2[:port2]...]` format. Default port is 2181. |
| `KVClusterMasterPrefix` | string | `"mysql/master"` | Prefix for cluster master entries in KV stores |

### 1.29 Graphite

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `GraphiteAddr` | string | `""` | Address of graphite port. If supplied, metrics will be written here. |
| `GraphitePath` | string | `""` | Prefix for graphite path. May include `{hostname}` placeholder. |
| `GraphiteConvertHostnameDotsToUnderscores` | bool | `true` | Convert hostname dots to underscores in graphite path |
| `GraphitePollSeconds` | int | `60` | Graphite writes interval. 0 disables. |

### 1.30 ProxySQL

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `ProxySQLAdminAddress` | string | `""` | Address of ProxySQL Admin interface (e.g. `127.0.0.1`). Empty disables ProxySQL hooks. |
| `ProxySQLAdminPort` | int | `6032` | Port of ProxySQL Admin interface |
| `ProxySQLAdminUser` | string | `"admin"` | Username for ProxySQL Admin |
| `ProxySQLAdminPassword` | string | `""` | Password for ProxySQL Admin |
| `ProxySQLAdminUseTLS` | bool | `false` | Use TLS for ProxySQL Admin connection |
| `ProxySQLWriterHostgroup` | int | `0` | ProxySQL hostgroup ID for the writer (master). Must be > 0 to enable hooks. |
| `ProxySQLReaderHostgroup` | int | `0` | ProxySQL hostgroup ID for readers (replicas). Optional. |
| `ProxySQLPreFailoverAction` | string | `"offline_soft"` | Pre-failover action on old master: `"offline_soft"`, `"weight_zero"`, or `"none"` |

### 1.31 Prometheus

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `PrometheusEnabled` | bool | `true` | When true, expose Prometheus metrics on `/metrics` endpoint |

### 1.32 Web UI / Miscellaneous

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `WebMessage` | string | `""` | If provided, shown on all web pages below the title bar |
| `PrependMessagesWithOrcIdentity` | string | `""` | Use `"FQDN"`, `"hostname"`, or `"custom"` to prefix error messages. Empty/`"none"` disables. |
| `CustomOrcIdentity` | string | `""` | Custom identity string when `PrependMessagesWithOrcIdentity` is `"custom"` |

---

## 2. CLI Command Reference

Usage: `orchestrator -c <command> [-i <instance.fqdn>[:<port>]] [-d <destination.fqdn>[:<port>]] [options]`

When `RaftEnabled` is true, CLI access is blocked by default. Use `--ignore-raft-setup` to override, or use the `orchestrator-client` script which speaks to the HTTP API.

### Command Synonyms

The following legacy command names are automatically mapped to their current equivalents:

| Legacy Name | Current Name |
|-------------|-------------|
| `stop-slave` | `stop-replica` |
| `start-slave` | `start-replica` |
| `restart-slave` | `restart-replica` |
| `reset-slave` | `reset-replica` |
| `restart-slave-statements` | `restart-replica-statements` |
| `relocate-slaves` | `relocate-replicas` |
| `regroup-slaves` | `regroup-replicas` |
| `move-up-slaves` | `move-up-replicas` |
| `repoint-slaves` | `repoint-replicas` |
| `enslave-siblings` | `take-siblings` |
| `enslave-master` | `take-master` |
| `get-candidate-slave` | `get-candidate-replica` |
| `move-slaves-gtid` | `move-replicas-gtid` |
| `regroup-slaves-gtid` | `regroup-replicas-gtid` |
| `match-slaves` | `match-replicas` |
| `match-up-slaves` | `match-up-replicas` |
| `regroup-slaves-pgtid` | `regroup-replicas-pgtid` |
| `which-cluster-osc-slaves` | `which-cluster-osc-replicas` |
| `which-cluster-gh-ost-slaves` | `which-cluster-gh-ost-replicas` |
| `which-slaves` | `which-replicas` |
| `detach-slave`, `detach-replica`, `detach-slave-master-host` | `detach-replica-master-host` |
| `reattach-slave`, `reattach-replica`, `reattach-slave-master-host` | `reattach-replica-master-host` |

### 2.1 Smart Relocation

| Command | Description | Example |
|---------|-------------|---------|
| `relocate` | Relocate a replica beneath another instance | `orchestrator -c relocate -i replica1:3306 -d newmaster:3306` |
| `relocate-below` | Synonym to `relocate` (will be deprecated) | `orchestrator -c relocate-below -i replica1:3306 -d newmaster:3306` |
| `relocate-replicas` | Relocates all or part of the replicas of a given instance under another instance | `orchestrator -c relocate-replicas -i oldmaster:3306 -d newmaster:3306` |
| `take-siblings` | Turn all siblings of a replica into its sub-replicas | `orchestrator -c take-siblings -i replica1:3306` |
| `regroup-replicas` | Given an instance, pick one of its replicas and make it local master of its siblings | `orchestrator -c regroup-replicas -i master:3306` |

### 2.2 Classic file:pos Relocation

| Command | Description | Example |
|---------|-------------|---------|
| `move-up` | Move a replica one level up the topology | `orchestrator -c move-up -i replica1:3306` |
| `move-up-replicas` | Moves replicas of the given instance one level up the topology | `orchestrator -c move-up-replicas -i intermediate:3306` |
| `move-below` | Moves a replica beneath its sibling. Both must replicate from same master. | `orchestrator -c move-below -i replica1:3306 -d sibling:3306` |
| `move-equivalent` | Moves a replica beneath another server using previously recorded equivalence coordinates | `orchestrator -c move-equivalent -i replica1:3306 -d target:3306` |
| `repoint` | Make the given instance replicate from another instance without changing binlog coordinates. Use with care. | `orchestrator -c repoint -i replica1:3306 -d newmaster:3306` |
| `repoint-replicas` | Repoint all replicas of given instance to replicate back from the instance. Use with care. | `orchestrator -c repoint-replicas -i master:3306` |
| `take-master` | Turn an instance into a master of its own master; essentially switch the two | `orchestrator -c take-master -i replica1:3306` |
| `make-co-master` | Create a master-master replication. Given instance is a replica replicating directly from a master. | `orchestrator -c make-co-master -i replica1:3306` |
| `get-candidate-replica` | Suggest the most up-to-date replica of a given instance that is good for promotion | `orchestrator -c get-candidate-replica -i master:3306` |

### 2.3 Binlog Server Relocation

| Command | Description | Example |
|---------|-------------|---------|
| `regroup-replicas-bls` | Regroup Binlog Server replicas of a given instance | `orchestrator -c regroup-replicas-bls -i master:3306` |

### 2.4 GTID Relocation

| Command | Description | Example |
|---------|-------------|---------|
| `move-gtid` | Move a replica beneath another instance using GTID | `orchestrator -c move-gtid -i replica1:3306 -d newmaster:3306` |
| `move-replicas-gtid` | Moves all replicas of a given instance under another using GTID | `orchestrator -c move-replicas-gtid -i oldmaster:3306 -d newmaster:3306` |
| `regroup-replicas-gtid` | Given an instance, pick one of its replicas and make it local master of siblings, using GTID | `orchestrator -c regroup-replicas-gtid -i master:3306` |

### 2.5 Pseudo-GTID Relocation

| Command | Description | Example |
|---------|-------------|---------|
| `match` | Matches a replica beneath another instance using Pseudo-GTID | `orchestrator -c match -i replica1:3306 -d target:3306` |
| `match-up` | Transport the replica one level up the hierarchy using Pseudo-GTID | `orchestrator -c match-up -i replica1:3306` |
| `rematch` | Reconnect a replica onto its master via Pseudo-GTID | `orchestrator -c rematch -i replica1:3306` |
| `match-replicas` | Matches all replicas of a given instance under another using Pseudo-GTID | `orchestrator -c match-replicas -i oldmaster:3306 -d target:3306` |
| `match-up-replicas` | Matches replicas of the given instance one level up, making them siblings, using Pseudo-GTID | `orchestrator -c match-up-replicas -i intermediate:3306` |
| `regroup-replicas-pgtid` | Given an instance, pick one of its replicas and make it local master of siblings, using Pseudo-GTID | `orchestrator -c regroup-replicas-pgtid -i master:3306` |

### 2.6 Replication, General

| Command | Description | Example |
|---------|-------------|---------|
| `enable-gtid` | If possible, turn on GTID replication | `orchestrator -c enable-gtid -i replica1:3306` |
| `disable-gtid` | Turn off GTID replication, back to file:pos | `orchestrator -c disable-gtid -i replica1:3306` |
| `which-gtid-errant` | Get errant GTID set (empty if no errant GTID) | `orchestrator -c which-gtid-errant -i replica1:3306` |
| `gtid-errant-reset-master` | Reset master on instance, remove GTID errant transactions | `orchestrator -c gtid-errant-reset-master -i replica1:3306` |
| `skip-query` | Skip a single statement on a replica (GTID or non-GTID) | `orchestrator -c skip-query -i replica1:3306` |
| `stop-replica` | Issue a STOP SLAVE on an instance | `orchestrator -c stop-replica -i replica1:3306` |
| `start-replica` | Issue a START SLAVE on an instance | `orchestrator -c start-replica -i replica1:3306` |
| `restart-replica` | STOP and START SLAVE on an instance | `orchestrator -c restart-replica -i replica1:3306` |
| `reset-replica` | Issues a RESET SLAVE command; use with care | `orchestrator -c reset-replica -i replica1:3306` |
| `detach-replica-master-host` | Stops replication and modifies Master_Host to an impossible but reversible value | `orchestrator -c detach-replica-master-host -i replica1:3306` |
| `reattach-replica-master-host` | Undo a detach-replica-master-host operation | `orchestrator -c reattach-replica-master-host -i replica1:3306` |
| `master-pos-wait` | Wait until replica reaches given replication coordinates (`--binlog=file:pos`) | `orchestrator -c master-pos-wait -i replica1:3306 --binlog=mysql-bin.000003:12345` |
| `enable-semi-sync-master` | Enable semi-sync replication (master-side) | `orchestrator -c enable-semi-sync-master -i master:3306` |
| `disable-semi-sync-master` | Disable semi-sync replication (master-side) | `orchestrator -c disable-semi-sync-master -i master:3306` |
| `enable-semi-sync-replica` | Enable semi-sync replication (replica-side) | `orchestrator -c enable-semi-sync-replica -i replica1:3306` |
| `disable-semi-sync-replica` | Disable semi-sync replication (replica-side) | `orchestrator -c disable-semi-sync-replica -i replica1:3306` |
| `restart-replica-statements` | Get a list of statements to stop then restore replica to same execution state. Use `--statement` for injected statement. | `orchestrator -c restart-replica-statements -i replica1:3306` |

### 2.7 Replication Information

| Command | Description | Example |
|---------|-------------|---------|
| `can-replicate-from` | Can instance (`-i`) replicate from another (`-d`) per replication rules? Prints the destination if yes. | `orchestrator -c can-replicate-from -i replica1:3306 -d master:3306` |
| `is-replicating` | Is an instance actively replicating right now? | `orchestrator -c is-replicating -i replica1:3306` |
| `is-replication-stopped` | Is an instance a replica with both replication threads stopped? | `orchestrator -c is-replication-stopped -i replica1:3306` |

### 2.8 Instance

| Command | Description | Example |
|---------|-------------|---------|
| `set-read-only` | Turn an instance read-only via `SET GLOBAL read_only := 1` | `orchestrator -c set-read-only -i master:3306` |
| `set-writeable` | Turn an instance writeable via `SET GLOBAL read_only := 0` | `orchestrator -c set-writeable -i master:3306` |

### 2.9 Binary Logs

| Command | Description | Example |
|---------|-------------|---------|
| `flush-binary-logs` | Flush binary logs on an instance | `orchestrator -c flush-binary-logs -i master:3306` |
| `purge-binary-logs` | Purge binary logs of an instance (requires `--binlog`) | `orchestrator -c purge-binary-logs -i master:3306 --binlog=mysql-bin.000003` |
| `last-pseudo-gtid` | Find latest Pseudo-GTID entry in instance's binary logs | `orchestrator -c last-pseudo-gtid -i master:3306` |
| `locate-gtid-errant` | List binary logs containing errant GTIDs | `orchestrator -c locate-gtid-errant -i replica1:3306` |
| `last-executed-relay-entry` | Find coordinates of last executed relay log entry | `orchestrator -c last-executed-relay-entry -i replica1:3306` |
| `correlate-relaylog-pos` | Given an instance (`-i`) and relaylog coordinates (`--binlog=file:pos`), find correlated coordinates in another instance's relay logs (`-d`) | `orchestrator -c correlate-relaylog-pos -i replica1:3306 -d replica2:3306 --binlog=relay-bin.000003:12345` |
| `find-binlog-entry` | Get binlog file:pos of entry given by `--pattern` (exact match) in a given instance | `orchestrator -c find-binlog-entry -i master:3306 --pattern "DROP VIEW"` |
| `correlate-binlog-pos` | Given an instance (`-i`) and binlog coordinates (`--binlog=file:pos`), find correlated coordinates in another instance (`-d`) | `orchestrator -c correlate-binlog-pos -i master:3306 -d replica1:3306 --binlog=mysql-bin.000003:12345` |

### 2.10 Pools

| Command | Description | Example |
|---------|-------------|---------|
| `submit-pool-instances` | Submit a pool name with a list of instances in that pool | `orchestrator -c submit-pool-instances --pool mypool -i "host1:3306,host2:3306"` |
| `cluster-pool-instances` | List all pools and their associated instances | `orchestrator -c cluster-pool-instances` |
| `which-heuristic-cluster-pool-instances` | List instances of a given cluster in any or a specific pool | `orchestrator -c which-heuristic-cluster-pool-instances --alias mycluster --pool mypool` |

### 2.11 Information

| Command | Description | Example |
|---------|-------------|---------|
| `find` | Find instances whose hostname matches given regex pattern | `orchestrator -c find --pattern "db-prod.*"` |
| `search` | Search instances by name, version, version comment, port | `orchestrator -c search --pattern "5.7"` |
| `clusters` | List all clusters known to orchestrator | `orchestrator -c clusters` |
| `clusters-alias` | List all clusters with their aliases | `orchestrator -c clusters-alias` |
| `all-clusters-masters` | List writeable masters, one per cluster | `orchestrator -c all-clusters-masters` |
| `topology` | Show an ASCII-graph of a replication topology | `orchestrator -c topology -i master:3306` or `orchestrator -c topology --alias mycluster` |
| `topology-tabulated` | Show an ASCII-graph of a replication topology (tabulated format) | `orchestrator -c topology-tabulated --alias mycluster` |
| `topology-tags` | Show an ASCII-graph of a replication topology with instance tags | `orchestrator -c topology-tags --alias mycluster` |
| `all-instances` | The complete list of known instances | `orchestrator -c all-instances` |
| `which-instance` | Output the fully-qualified hostname:port of the given instance | `orchestrator -c which-instance -i host:3306` |
| `which-cluster` | Output the cluster name an instance belongs to | `orchestrator -c which-cluster -i host:3306` |
| `which-cluster-alias` | Output the alias of the cluster an instance belongs to | `orchestrator -c which-cluster-alias -i host:3306` |
| `which-cluster-domain` | Output the domain name of the cluster an instance belongs to | `orchestrator -c which-cluster-domain -i host:3306` |
| `which-heuristic-domain-instance` | Returns the instance associated as writer with a cluster's domain name | `orchestrator -c which-heuristic-domain-instance --alias mycluster` |
| `which-cluster-master` | Output the name of the master in a given cluster | `orchestrator -c which-cluster-master --alias mycluster` |
| `which-cluster-instances` | Output the list of instances in the same cluster | `orchestrator -c which-cluster-instances --alias mycluster` |
| `which-cluster-osc-replicas` | Output replicas in a cluster suitable for pt-online-schema-change | `orchestrator -c which-cluster-osc-replicas --alias mycluster` |
| `which-cluster-gh-ost-replicas` | Output replicas in a cluster suitable as a gh-ost working server | `orchestrator -c which-cluster-gh-ost-replicas --alias mycluster` |
| `which-master` | Output the hostname:port of a given instance's master | `orchestrator -c which-master -i replica1:3306` |
| `which-downtimed-instances` | List instances currently downtimed, optionally filtered by cluster | `orchestrator -c which-downtimed-instances --alias mycluster` |
| `which-replicas` | Output the hostname:port list of replicas of a given instance | `orchestrator -c which-replicas -i master:3306` |
| `which-lost-in-recovery` | List instances marked as downtimed for being lost in a recovery process | `orchestrator -c which-lost-in-recovery` |
| `instance-status` | Output short status on a given instance | `orchestrator -c instance-status -i host:3306` |
| `get-cluster-heuristic-lag` | Output a heuristic representative lag for a given cluster | `orchestrator -c get-cluster-heuristic-lag --alias mycluster` |

### 2.12 Key-Value Stores

| Command | Description | Example |
|---------|-------------|---------|
| `submit-masters-to-kv-stores` | Submit master of a specific cluster, or all masters to key-value stores | `orchestrator -c submit-masters-to-kv-stores --alias mycluster` |

### 2.13 Tags

| Command | Description | Example |
|---------|-------------|---------|
| `tags` | List tags for a given instance | `orchestrator -c tags -i host:3306` |
| `tag-value` | Get tag value for a specific instance (requires `--tag`) | `orchestrator -c tag-value -i host:3306 --tag "role"` |
| `tagged` | List instances tagged by tag-string. Format: `"tagname"` or `"tagname=tagvalue"` or comma-separated for intersection. | `orchestrator -c tagged --tag "role=primary"` |
| `tag` | Add a tag to a given instance (requires `--tag`) | `orchestrator -c tag -i host:3306 --tag "role=primary"` |
| `untag` | Remove a tag from an instance (requires `--tag`) | `orchestrator -c untag -i host:3306 --tag "role"` |
| `untag-all` | Remove a tag from all matching instances (requires `--tag`) | `orchestrator -c untag-all --tag "role"` |

### 2.14 Instance Management

| Command | Description | Example |
|---------|-------------|---------|
| `discover` | Lookup an instance, investigate it | `orchestrator -c discover -i host:3306` |
| `forget` | Forget about an instance's existence | `orchestrator -c forget -i host:3306` |
| `begin-maintenance` | Request a maintenance lock on an instance (requires `--reason`) | `orchestrator -c begin-maintenance -i host:3306 --reason "hardware upgrade" --duration 1h` |
| `end-maintenance` | Remove maintenance lock from an instance | `orchestrator -c end-maintenance -i host:3306` |
| `in-maintenance` | Check whether instance is under maintenance | `orchestrator -c in-maintenance -i host:3306` |
| `begin-downtime` | Mark an instance as downtimed (requires `--reason`) | `orchestrator -c begin-downtime -i host:3306 --reason "planned maintenance" --duration 2h` |
| `end-downtime` | Indicate an instance is no longer downtimed | `orchestrator -c end-downtime -i host:3306` |

### 2.15 Recovery

| Command | Description | Example |
|---------|-------------|---------|
| `recover` | Do auto-recovery given a dead instance | `orchestrator -c recover -i dead-master:3306` |
| `recover-lite` | Do auto-recovery without executing external processes | `orchestrator -c recover-lite -i dead-master:3306` |
| `force-master-failover` | Forcibly discard master and initiate failover, even if no problem detected. Orchestrator chooses the replacement. | `orchestrator -c force-master-failover --alias mycluster` |
| `force-master-takeover` | Forcibly discard master and promote specified instance (`-d`) | `orchestrator -c force-master-takeover --alias mycluster -d newmaster:3306` |
| `graceful-master-takeover` | Gracefully promote a new master. Specify identity via `-d` or have a single direct replica. | `orchestrator -c graceful-master-takeover --alias mycluster -d newmaster:3306` |
| `graceful-master-takeover-auto` | Gracefully promote a new master. Orchestrator attempts to pick the replica automatically. | `orchestrator -c graceful-master-takeover-auto --alias mycluster` |
| `replication-analysis` | Request an analysis of potential crash incidents in all known topologies | `orchestrator -c replication-analysis` |
| `ack-all-recoveries` | Acknowledge all recoveries; unblocks pending future recoveries (requires `--reason`) | `orchestrator -c ack-all-recoveries --reason "all clear"` |
| `ack-cluster-recoveries` | Acknowledge recoveries for a given cluster (requires `--reason`) | `orchestrator -c ack-cluster-recoveries --alias mycluster --reason "resolved"` |
| `ack-instance-recoveries` | Acknowledge recoveries for a given instance (requires `--reason`) | `orchestrator -c ack-instance-recoveries -i host:3306 --reason "resolved"` |

### 2.16 Instance Meta

| Command | Description | Example |
|---------|-------------|---------|
| `register-candidate` | Indicate that an instance is a preferred candidate for master promotion | `orchestrator -c register-candidate -i replica1:3306 --promotion-rule prefer` |
| `register-hostname-unresolve` | Assigns the given instance a virtual ("unresolved") name | `orchestrator -c register-hostname-unresolve -i host:3306 --hostname virtualname` |
| `deregister-hostname-unresolve` | Deregister/disassociate a hostname with an "unresolved" name | `orchestrator -c deregister-hostname-unresolve -i host:3306` |
| `set-heuristic-domain-instance` | Associate domain name of given cluster with the writer master | `orchestrator -c set-heuristic-domain-instance --alias mycluster` |

### 2.17 Meta / Orchestrator Operations

| Command | Description | Example |
|---------|-------------|---------|
| `snapshot-topologies` | Take a snapshot of existing topologies | `orchestrator -c snapshot-topologies` |
| `continuous` | Enter continuous mode: actively poll for instances, diagnose problems, do maintenance | `orchestrator -c continuous` |
| `active-nodes` | List currently active orchestrator nodes | `orchestrator -c active-nodes` |
| `access-token` | Get an HTTP access token | `orchestrator -c access-token` |
| `resolve` | Resolve given hostname | `orchestrator -c resolve -i host:3306` |
| `reset-hostname-resolve-cache` | Clear the hostname resolve cache | `orchestrator -c reset-hostname-resolve-cache` |
| `dump-config` | Print out configuration in JSON format | `orchestrator -c dump-config` |
| `show-resolve-hosts` | Show the content of the hostname_resolve table (debugging) | `orchestrator -c show-resolve-hosts` |
| `show-unresolve-hosts` | Show the content of the hostname_unresolve table (debugging) | `orchestrator -c show-unresolve-hosts` |
| `redeploy-internal-db` | Force internal schema migration to current backend structure | `orchestrator -c redeploy-internal-db` |
| `internal-suggest-promoted-replacement` | Internal only, used to test promotion logic in CI | `orchestrator -c internal-suggest-promoted-replacement -i old:3306 -d candidate:3306` |

### 2.18 Global Recoveries

| Command | Description | Example |
|---------|-------------|---------|
| `disable-global-recoveries` | Disallow orchestrator from performing recoveries globally | `orchestrator -c disable-global-recoveries` |
| `enable-global-recoveries` | Allow orchestrator to perform recoveries globally | `orchestrator -c enable-global-recoveries` |
| `check-global-recoveries` | Show the global recovery configuration | `orchestrator -c check-global-recoveries` |

### 2.19 Bulk Operations

| Command | Description | Example |
|---------|-------------|---------|
| `bulk-instances` | Return a sorted list of instance names known to orchestrator | `orchestrator -c bulk-instances` |
| `bulk-promotion-rules` | Return a list of promotion rules known to orchestrator | `orchestrator -c bulk-promotion-rules` |

### 2.20 ProxySQL

| Command | Description | Example |
|---------|-------------|---------|
| `proxysql-test` | Test connectivity to ProxySQL Admin interface | `orchestrator -c proxysql-test` |
| `proxysql-servers` | Show `mysql_servers` from ProxySQL | `orchestrator -c proxysql-servers` |

### 2.21 Agent

| Command | Description | Example |
|---------|-------------|---------|
| `custom-command` | Execute a custom command on the agent as defined in the agent conf | `orchestrator -c custom-command --hostname agenthost --pattern commandname` |

---

## 3. API v1 Reference

All v1 API endpoints are accessible under `/api/`. All endpoints return JSON. When `URLPrefix` is configured, endpoints are at `/<prefix>/api/`.

Base URL: `http://<orchestrator-host>:3000/api/`

### 3.1 Smart Relocation

| Endpoint | Description |
|----------|-------------|
| `GET /api/relocate/{host}/{port}/{belowHost}/{belowPort}` | Relocate a replica beneath another instance |
| `GET /api/relocate-below/{host}/{port}/{belowHost}/{belowPort}` | Same as relocate |
| `GET /api/relocate-slaves/{host}/{port}/{belowHost}/{belowPort}` | Relocate all replicas of an instance to another |

### 3.2 Classic file:pos Relocation

| Endpoint | Description |
|----------|-------------|
| `GET /api/move-up/{host}/{port}` | Move a replica one level up |
| `GET /api/move-up-slaves/{host}/{port}` | Move replicas of an instance one level up |
| `GET /api/move-below/{host}/{port}/{siblingHost}/{siblingPort}` | Move a replica beneath its sibling |
| `GET /api/move-equivalent/{host}/{port}/{belowHost}/{belowPort}` | Move via equivalence coordinates |
| `GET /api/repoint/{host}/{port}/{belowHost}/{belowPort}` | Repoint without changing binlog coordinates |
| `GET /api/repoint-slaves/{host}/{port}` | Repoint all replicas |
| `GET /api/make-co-master/{host}/{port}` | Create co-master replication |
| `GET /api/enslave-siblings/{host}/{port}` | Turn siblings into sub-replicas |
| `GET /api/enslave-master/{host}/{port}` | Take over the master role |
| `GET /api/master-equivalent/{host}/{port}/{logFile}/{logPos}` | Find equivalent coordinates on master |

### 3.3 Binlog Server

| Endpoint | Description |
|----------|-------------|
| `GET /api/regroup-slaves-bls/{host}/{port}` | Regroup Binlog Server replicas |

### 3.4 GTID Relocation

| Endpoint | Description |
|----------|-------------|
| `GET /api/move-below-gtid/{host}/{port}/{belowHost}/{belowPort}` | Move using GTID |
| `GET /api/move-slaves-gtid/{host}/{port}/{belowHost}/{belowPort}` | Move replicas using GTID |
| `GET /api/regroup-slaves-gtid/{host}/{port}` | Regroup replicas using GTID |

### 3.5 Pseudo-GTID Relocation

| Endpoint | Description |
|----------|-------------|
| `GET /api/match/{host}/{port}/{belowHost}/{belowPort}` | Match using Pseudo-GTID |
| `GET /api/match-below/{host}/{port}/{belowHost}/{belowPort}` | Same as match |
| `GET /api/match-up/{host}/{port}` | Match-up one level using Pseudo-GTID |
| `GET /api/match-slaves/{host}/{port}/{belowHost}/{belowPort}` | Match all replicas using Pseudo-GTID |
| `GET /api/match-up-slaves/{host}/{port}` | Match replicas up one level using Pseudo-GTID |
| `GET /api/regroup-slaves-pgtid/{host}/{port}` | Regroup replicas using Pseudo-GTID |

### 3.6 Topology / Promotion

| Endpoint | Description |
|----------|-------------|
| `GET /api/make-master/{host}/{port}` | Promote an instance to master |
| `GET /api/make-local-master/{host}/{port}` | Promote an instance to local master |
| `GET /api/regroup-slaves/{host}/{port}` | Regroup replicas (smart mode) |

### 3.7 Replication Control

| Endpoint | Description |
|----------|-------------|
| `GET /api/enable-gtid/{host}/{port}` | Enable GTID replication |
| `GET /api/disable-gtid/{host}/{port}` | Disable GTID replication |
| `GET /api/locate-gtid-errant/{host}/{port}` | Locate errant GTID entries |
| `GET /api/gtid-errant-reset-master/{host}/{port}` | Reset master to clear errant GTIDs |
| `GET /api/gtid-errant-inject-empty/{host}/{port}` | Inject empty transactions for errant GTIDs |
| `GET /api/skip-query/{host}/{port}` | Skip a query on a replica |
| `GET /api/start-slave/{host}/{port}` | Start replication |
| `GET /api/restart-slave/{host}/{port}` | Restart replication |
| `GET /api/stop-slave/{host}/{port}` | Stop replication |
| `GET /api/stop-slave-nice/{host}/{port}` | Stop replication nicely |
| `GET /api/reset-slave/{host}/{port}` | Reset replication |
| `GET /api/detach-slave/{host}/{port}` | Detach replica master host |
| `GET /api/reattach-slave/{host}/{port}` | Reattach replica master host |
| `GET /api/detach-slave-master-host/{host}/{port}` | Detach replica master host |
| `GET /api/reattach-slave-master-host/{host}/{port}` | Reattach replica master host |
| `GET /api/flush-binary-logs/{host}/{port}` | Flush binary logs |
| `GET /api/purge-binary-logs/{host}/{port}/{logFile}` | Purge binary logs to given file |
| `GET /api/restart-slave-statements/{host}/{port}` | Get restart replica statements |
| `GET /api/enable-semi-sync-master/{host}/{port}` | Enable semi-sync (master-side) |
| `GET /api/disable-semi-sync-master/{host}/{port}` | Disable semi-sync (master-side) |
| `GET /api/enable-semi-sync-replica/{host}/{port}` | Enable semi-sync (replica-side) |
| `GET /api/disable-semi-sync-replica/{host}/{port}` | Disable semi-sync (replica-side) |
| `GET /api/delay-replication/{host}/{port}/{seconds}` | Set replication delay |

### 3.8 Replication Information

| Endpoint | Description |
|----------|-------------|
| `GET /api/can-replicate-from/{host}/{port}/{belowHost}/{belowPort}` | Check if replication is possible |
| `GET /api/can-replicate-from-gtid/{host}/{port}/{belowHost}/{belowPort}` | Check GTID-based replication possibility |

### 3.9 Instance Control

| Endpoint | Description |
|----------|-------------|
| `GET /api/set-read-only/{host}/{port}` | Set instance read-only |
| `GET /api/set-writeable/{host}/{port}` | Set instance writeable |
| `GET /api/kill-query/{host}/{port}/{process}` | Kill a specific query |

### 3.10 Binary Logs

| Endpoint | Description |
|----------|-------------|
| `GET /api/last-pseudo-gtid/{host}/{port}` | Find last Pseudo-GTID entry |

### 3.11 Pools

| Endpoint | Description |
|----------|-------------|
| `GET /api/submit-pool-instances/{pool}` | Submit pool instances |
| `GET /api/cluster-pool-instances/{clusterName}` | List pool instances for a cluster |
| `GET /api/cluster-pool-instances/{clusterName}/{pool}` | List instances in a specific pool for a cluster |
| `GET /api/heuristic-cluster-pool-instances/{clusterName}` | Heuristic pool instances for a cluster |
| `GET /api/heuristic-cluster-pool-instances/{clusterName}/{pool}` | Heuristic instances for a specific pool |
| `GET /api/heuristic-cluster-pool-lag/{clusterName}` | Heuristic pool lag for a cluster |
| `GET /api/heuristic-cluster-pool-lag/{clusterName}/{pool}` | Heuristic lag for a specific pool |

### 3.12 Search and Discovery

| Endpoint | Description |
|----------|-------------|
| `GET /api/search/{searchString}` | Search instances by various attributes |
| `GET /api/search` | Search instances (empty search returns all) |
| `GET /api/instance/{host}/{port}` | Get instance details |
| `GET /api/discover/{host}/{port}` | Discover an instance |
| `GET /api/async-discover/{host}/{port}` | Asynchronously discover an instance |
| `GET /api/refresh/{host}/{port}` | Refresh instance data |
| `GET /api/forget/{host}/{port}` | Forget an instance |
| `GET /api/forget-cluster/{clusterHint}` | Forget an entire cluster |

### 3.13 Cluster Information

| Endpoint | Description |
|----------|-------------|
| `GET /api/cluster/{clusterHint}` | Get cluster instances |
| `GET /api/cluster/alias/{clusterAlias}` | Get cluster by alias |
| `GET /api/cluster/instance/{host}/{port}` | Get cluster by instance |
| `GET /api/cluster-info/{clusterHint}` | Get cluster info |
| `GET /api/cluster-info/alias/{clusterAlias}` | Get cluster info by alias |
| `GET /api/cluster-osc-slaves/{clusterHint}` | Get OSC replicas |
| `GET /api/set-cluster-alias/{clusterName}` | Set a manual cluster alias override |
| `GET /api/clusters` | List all clusters |
| `GET /api/clusters-info` | List all clusters with info |
| `GET /api/masters` | List all masters |
| `GET /api/master/{clusterHint}` | Get master for a cluster |
| `GET /api/instance-replicas/{host}/{port}` | List replicas of an instance |
| `GET /api/all-instances` | List all instances |
| `GET /api/downtimed` | List all downtimed instances |
| `GET /api/downtimed/{clusterHint}` | List downtimed instances for a cluster |
| `GET /api/topology/{clusterHint}` | ASCII topology for a cluster |
| `GET /api/topology/{host}/{port}` | ASCII topology via instance |
| `GET /api/topology-tabulated/{clusterHint}` | Tabulated ASCII topology |
| `GET /api/topology-tabulated/{host}/{port}` | Tabulated ASCII topology via instance |
| `GET /api/topology-tags/{clusterHint}` | ASCII topology with tags |
| `GET /api/topology-tags/{host}/{port}` | ASCII topology with tags via instance |
| `GET /api/snapshot-topologies` | Snapshot all topologies |

### 3.14 Tags

| Endpoint | Description |
|----------|-------------|
| `GET /api/tagged` | List instances matching tag query |
| `GET /api/tags/{host}/{port}` | List tags for an instance |
| `GET /api/tag-value/{host}/{port}` | Get tag value |
| `GET /api/tag-value/{host}/{port}/{tagName}` | Get specific tag value |
| `GET /api/tag/{host}/{port}` | Set a tag |
| `GET /api/tag/{host}/{port}/{tagName}/{tagValue}` | Set a tag with name and value |
| `GET /api/untag/{host}/{port}` | Remove a tag |
| `GET /api/untag/{host}/{port}/{tagName}` | Remove a specific tag |
| `GET /api/untag-all` | Remove tag from all instances |
| `GET /api/untag-all/{tagName}/{tagValue}` | Remove specific tag from all instances |

### 3.15 Instance Management

| Endpoint | Description |
|----------|-------------|
| `GET /api/begin-maintenance/{host}/{port}/{owner}/{reason}` | Begin maintenance on an instance |
| `GET /api/end-maintenance/{host}/{port}` | End maintenance by instance key |
| `GET /api/in-maintenance/{host}/{port}` | Check if instance is in maintenance |
| `GET /api/end-maintenance/{maintenanceKey}` | End maintenance by maintenance key |
| `GET /api/maintenance` | List all active maintenance entries |
| `GET /api/begin-downtime/{host}/{port}/{owner}/{reason}` | Begin downtime |
| `GET /api/begin-downtime/{host}/{port}/{owner}/{reason}/{duration}` | Begin downtime with duration |
| `GET /api/end-downtime/{host}/{port}` | End downtime |

### 3.16 Recovery and Analysis

| Endpoint | Description |
|----------|-------------|
| `GET /api/replication-analysis` | Get replication analysis for all topologies |
| `GET /api/replication-analysis/{clusterName}` | Analysis for a specific cluster |
| `GET /api/replication-analysis/instance/{host}/{port}` | Analysis for a specific instance |
| `GET /api/recover/{host}/{port}` | Initiate recovery |
| `GET /api/recover/{host}/{port}/{candidateHost}/{candidatePort}` | Recover with candidate |
| `GET /api/recover-lite/{host}/{port}` | Recover without external processes |
| `GET /api/recover-lite/{host}/{port}/{candidateHost}/{candidatePort}` | Recover-lite with candidate |
| `GET /api/graceful-master-takeover/{host}/{port}` | Graceful master takeover |
| `GET /api/graceful-master-takeover/{host}/{port}/{designatedHost}/{designatedPort}` | Graceful takeover with designated |
| `GET /api/graceful-master-takeover/{clusterHint}` | Graceful takeover by cluster |
| `GET /api/graceful-master-takeover/{clusterHint}/{designatedHost}/{designatedPort}` | Graceful takeover by cluster with designated |
| `GET /api/graceful-master-takeover-auto/{host}/{port}` | Auto graceful takeover |
| `GET /api/graceful-master-takeover-auto/{host}/{port}/{designatedHost}/{designatedPort}` | Auto takeover with designated |
| `GET /api/graceful-master-takeover-auto/{clusterHint}` | Auto takeover by cluster |
| `GET /api/graceful-master-takeover-auto/{clusterHint}/{designatedHost}/{designatedPort}` | Auto takeover by cluster with designated |
| `GET /api/force-master-failover/{host}/{port}` | Force master failover |
| `GET /api/force-master-failover/{clusterHint}` | Force failover by cluster |
| `GET /api/force-master-takeover/{clusterHint}/{designatedHost}/{designatedPort}` | Force takeover by cluster |
| `GET /api/force-master-takeover/{host}/{port}/{designatedHost}/{designatedPort}` | Force takeover with specific instance |
| `GET /api/register-candidate/{host}/{port}/{promotionRule}` | Register promotion candidate |
| `GET /api/automated-recovery-filters` | Get recovery filters |
| `GET /api/audit-failure-detection` | Audit failure detections |
| `GET /api/audit-failure-detection/{page}` | Audit failure detections (paginated) |
| `GET /api/audit-failure-detection/id/{id}` | Audit failure detection by ID |
| `GET /api/audit-failure-detection/alias/{clusterAlias}` | Audit failure detection by alias |
| `GET /api/audit-failure-detection/alias/{clusterAlias}/{page}` | Audit failure detection by alias (paginated) |
| `GET /api/replication-analysis-changelog` | Replication analysis changelog |
| `GET /api/audit-recovery` | Audit recovery operations |
| `GET /api/audit-recovery/{page}` | Audit recovery (paginated) |
| `GET /api/audit-recovery/id/{id}` | Audit recovery by ID |
| `GET /api/audit-recovery/uid/{uid}` | Audit recovery by UID |
| `GET /api/audit-recovery/cluster/{clusterName}` | Audit recovery by cluster |
| `GET /api/audit-recovery/cluster/{clusterName}/{page}` | Audit recovery by cluster (paginated) |
| `GET /api/audit-recovery/alias/{clusterAlias}` | Audit recovery by alias |
| `GET /api/audit-recovery/alias/{clusterAlias}/{page}` | Audit recovery by alias (paginated) |
| `GET /api/audit-recovery-steps/{uid}` | Get recovery steps by UID |
| `GET /api/active-cluster-recovery/{clusterName}` | Active recoveries for a cluster |
| `GET /api/recently-active-cluster-recovery/{clusterName}` | Recently active recoveries for a cluster |
| `GET /api/recently-active-instance-recovery/{host}/{port}` | Recently active recoveries for an instance |
| `GET /api/ack-recovery/cluster/{clusterHint}` | Acknowledge cluster recovery |
| `GET /api/ack-recovery/cluster/alias/{clusterAlias}` | Acknowledge recovery by cluster alias |
| `GET /api/ack-recovery/instance/{host}/{port}` | Acknowledge instance recovery |
| `GET /api/ack-recovery/{recoveryId}` | Acknowledge recovery by ID |
| `GET /api/ack-recovery/uid/{uid}` | Acknowledge recovery by UID |
| `GET /api/ack-all-recoveries` | Acknowledge all recoveries |
| `GET /api/blocked-recoveries` | List blocked recoveries |
| `GET /api/blocked-recoveries/cluster/{clusterName}` | List blocked recoveries for a cluster |
| `GET /api/disable-global-recoveries` | Disable recoveries globally |
| `GET /api/enable-global-recoveries` | Enable recoveries globally |
| `GET /api/check-global-recoveries` | Check global recovery status |

### 3.17 Problems and Audit

| Endpoint | Description |
|----------|-------------|
| `GET /api/problems` | List all detected problems |
| `GET /api/problems/{clusterName}` | List problems for a cluster |
| `GET /api/audit` | Audit log |
| `GET /api/audit/{page}` | Audit log (paginated) |
| `GET /api/audit/instance/{host}/{port}` | Audit log for an instance |
| `GET /api/audit/instance/{host}/{port}/{page}` | Audit log for an instance (paginated) |
| `GET /api/resolve/{host}/{port}` | Resolve hostname |

### 3.18 Health and Raft

These endpoints do NOT proxy through the raft leader.

| Endpoint | Description |
|----------|-------------|
| `GET /api/headers` | Show request headers (for auth debugging) |
| `GET /api/health` | Health check |
| `GET /api/lb-check` | Load-balancer health check |
| `GET /api/_ping` | Same as `lb-check` |
| `GET /api/leader-check` | Returns 200 if this node is the leader |
| `GET /api/leader-check/{errorStatusCode}` | Leader check with custom error status code |
| `GET /api/grab-election` | Grab leadership election |
| `GET /api/raft-add-peer/{addr}` | Add a raft peer (proxied to leader) |
| `GET /api/raft-remove-peer/{addr}` | Remove a raft peer (proxied to leader) |
| `GET /api/raft-yield/{node}` | Yield raft leadership to a specific node |
| `GET /api/raft-yield-hint/{hint}` | Yield raft leadership with hint |
| `GET /api/raft-peers` | List raft peers |
| `GET /api/raft-state` | Get raft state |
| `GET /api/raft-leader` | Get current raft leader |
| `GET /api/raft-health` | Raft health check |
| `GET /api/raft-status` | Raft status |
| `GET /api/raft-snapshot` | Trigger raft snapshot |
| `GET /api/raft-follower-health-report/{authenticationToken}/{raftBind}/{raftAdvertise}` | Raft follower health report |
| `GET /api/reload-configuration` | Reload configuration from file |
| `GET /api/hostname-resolve-cache` | Show hostname resolve cache |
| `GET /api/reset-hostname-resolve-cache` | Reset hostname resolve cache |

### 3.19 Hostname and Configuration

| Endpoint | Description |
|----------|-------------|
| `GET /api/routed-leader-check` | Leader check (proxied through raft) |
| `GET /api/reelect` | Trigger re-election |
| `GET /api/reload-cluster-alias` | Reload cluster alias configuration |
| `GET /api/deregister-hostname-unresolve/{host}/{port}` | Deregister hostname unresolve |
| `GET /api/register-hostname-unresolve/{host}/{port}/{virtualname}` | Register hostname unresolve |

### 3.20 Bulk Operations

| Endpoint | Description |
|----------|-------------|
| `GET /api/bulk-instances` | Sorted list of all instance names |
| `GET /api/bulk-promotion-rules` | List of all promotion rules |

### 3.21 Discovery Metrics

| Endpoint | Description |
|----------|-------------|
| `GET /api/discovery-metrics-raw/{seconds}` | Raw discovery metrics |
| `GET /api/discovery-metrics-aggregated/{seconds}` | Aggregated discovery metrics |
| `GET /api/discovery-queue-metrics-raw/{seconds}` | Raw discovery queue metrics |
| `GET /api/discovery-queue-metrics-aggregated/{seconds}` | Aggregated discovery queue metrics |
| `GET /api/discovery-queue-metrics-raw/{queue}/{seconds}` | Raw metrics for a specific queue |
| `GET /api/discovery-queue-metrics-aggregated/{queue}/{seconds}` | Aggregated metrics for a specific queue |
| `GET /api/backend-query-metrics-raw/{seconds}` | Raw backend query metrics |
| `GET /api/backend-query-metrics-aggregated/{seconds}` | Aggregated backend query metrics |
| `GET /api/write-buffer-metrics-raw/{seconds}` | Raw write buffer metrics |
| `GET /api/write-buffer-metrics-aggregated/{seconds}` | Aggregated write buffer metrics |

### 3.22 Agents

| Endpoint | Description |
|----------|-------------|
| `GET /api/agents` | List all agents |
| `GET /api/agent/{host}` | Get agent details |
| `GET /api/agent-umount/{host}` | Unmount agent LV |
| `GET /api/agent-mount/{host}` | Mount agent LV |
| `GET /api/agent-create-snapshot/{host}` | Create LVM snapshot |
| `GET /api/agent-removelv/{host}` | Remove LVM logical volume |
| `GET /api/agent-mysql-stop/{host}` | Stop MySQL on agent |
| `GET /api/agent-mysql-start/{host}` | Start MySQL on agent |
| `GET /api/agent-seed/{targetHost}/{sourceHost}` | Seed (clone) from source to target |
| `GET /api/agent-active-seeds/{host}` | Active seeds for a host |
| `GET /api/agent-recent-seeds/{host}` | Recent seeds for a host |
| `GET /api/agent-seed-details/{seedId}` | Seed details |
| `GET /api/agent-seed-states/{seedId}` | Seed states |
| `GET /api/agent-abort-seed/{seedId}` | Abort a seed |
| `GET /api/agent-custom-command/{host}/{command}` | Execute custom agent command |
| `GET /api/seeds` | List all seeds |

### 3.23 ProxySQL

| Endpoint | Description |
|----------|-------------|
| `GET /api/proxysql/servers` | List all servers from ProxySQL `runtime_mysql_servers` |
| `GET /api/proxysql/servers/{hostgroup}` | List servers filtered by hostgroup ID |

### 3.24 Status

| Endpoint | Description |
|----------|-------------|
| `GET /api/status` | Status check (when `StatusEndpoint` is configured) |

### 3.25 KV Stores

| Endpoint | Description |
|----------|-------------|
| `GET /api/submit-masters-to-kv-stores` | Submit all masters to KV stores |
| `GET /api/submit-masters-to-kv-stores/{clusterHint}` | Submit specific cluster master to KV stores |

---

## 4. API v2 Reference

API v2 uses structured JSON envelopes with consistent response format. All v2 endpoints are under `/api/v2/` (respects `URLPrefix`).

### Response Envelope

All v2 responses use this structure:

```json
{
  "status": "ok",
  "data": { ... },
  "message": ""
}
```

Error responses:

```json
{
  "status": "error",
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable message"
  }
}
```

### Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v2/clusters` | List all known clusters with metadata |
| `GET` | `/api/v2/clusters/{name}` | Detailed information about a specific cluster |
| `GET` | `/api/v2/clusters/{name}/instances` | All instances belonging to a given cluster |
| `GET` | `/api/v2/clusters/{name}/topology` | ASCII topology representation for a cluster |
| `GET` | `/api/v2/instances/{host}/{port}` | Detailed information about a specific MySQL instance |
| `GET` | `/api/v2/recoveries` | Recent recovery entries. Query params: `cluster`, `alias`, `page`. |
| `GET` | `/api/v2/recoveries/active` | Currently active (in-progress) recoveries |
| `GET` | `/api/v2/status` | Health status of the orchestrator node |
| `GET` | `/api/v2/proxysql/servers` | All servers from ProxySQL `runtime_mysql_servers` table |

### Example Requests

```bash
# List all clusters
curl http://localhost:3000/api/v2/clusters

# Get cluster detail
curl http://localhost:3000/api/v2/clusters/mycluster

# Get instances in a cluster
curl http://localhost:3000/api/v2/clusters/mycluster/instances

# Get instance detail
curl http://localhost:3000/api/v2/instances/db1.example.com/3306

# Get recent recoveries filtered by cluster
curl "http://localhost:3000/api/v2/recoveries?cluster=mycluster&page=0"

# Get active recoveries
curl http://localhost:3000/api/v2/recoveries/active

# Health status
curl http://localhost:3000/api/v2/status

# ProxySQL servers
curl http://localhost:3000/api/v2/proxysql/servers
```

---

## 5. ProxySQL Configuration

Orchestrator has built-in support for updating ProxySQL hostgroups during failover. When configured, orchestrator automatically drains the old master and promotes the new master in ProxySQL without custom scripts.

### Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `ProxySQLAdminAddress` | string | `""` | ProxySQL Admin host. Leave empty to disable all ProxySQL hooks. |
| `ProxySQLAdminPort` | int | `6032` | ProxySQL Admin port |
| `ProxySQLAdminUser` | string | `"admin"` | Admin interface username |
| `ProxySQLAdminPassword` | string | `""` | Admin interface password |
| `ProxySQLAdminUseTLS` | bool | `false` | Use TLS for Admin connection |
| `ProxySQLWriterHostgroup` | int | `0` | Writer hostgroup ID. Must be > 0 to enable hooks. |
| `ProxySQLReaderHostgroup` | int | `0` | Reader hostgroup ID. Optional. |
| `ProxySQLPreFailoverAction` | string | `"offline_soft"` | Action on old master before failover |

### Minimal Configuration Example

```json
{
  "ProxySQLAdminAddress": "127.0.0.1",
  "ProxySQLAdminPort": 6032,
  "ProxySQLAdminUser": "admin",
  "ProxySQLAdminPassword": "admin",
  "ProxySQLWriterHostgroup": 10,
  "ProxySQLReaderHostgroup": 20,
  "ProxySQLPreFailoverAction": "offline_soft"
}
```

### Pre-Failover Actions

| Action | Behavior |
|--------|----------|
| `offline_soft` | Sets old master's status to `OFFLINE_SOFT`. Existing connections complete; no new ones are routed. |
| `weight_zero` | Sets old master's weight to 0. Similar effect but preserves the status field. |
| `none` | No pre-failover ProxySQL update. |

### Post-Failover Behavior

1. Old master is removed from the writer hostgroup
2. New master is added to the writer hostgroup
3. If reader hostgroup is configured: new master is removed from readers
4. If reader hostgroup is configured: old master is added to reader hostgroup as `OFFLINE_SOFT`
5. `LOAD MYSQL SERVERS TO RUNTIME` is executed
6. `SAVE MYSQL SERVERS TO DISK` is executed

### Failover Timeline Integration

```
Dead master detected
  -> OnFailureDetectionProcesses (scripts)
    -> PreFailoverProcesses (scripts)
      -> ProxySQL pre-failover: drain old master
        -> [topology manipulation: elect new master]
          -> KV store updates (Consul/ZK)
            -> ProxySQL post-failover: promote new master
              -> PostMasterFailoverProcesses (scripts)
                -> PostFailoverProcesses (scripts)
```

ProxySQL hooks run alongside existing script-based hooks. They are non-blocking: if ProxySQL is unreachable, the failover proceeds normally. Post-failover errors are logged but do not mark the recovery as failed.

### CLI Commands

```bash
# Test ProxySQL connectivity
orchestrator -c proxysql-test

# Show ProxySQL server list
orchestrator -c proxysql-servers
```

### API Endpoints

```bash
# List all servers
GET /api/proxysql/servers

# List servers by hostgroup
GET /api/proxysql/servers/:hostgroup

# V2 endpoint
GET /api/v2/proxysql/servers
```

### Multiple ProxySQL Instances

For ProxySQL Cluster deployments, configure orchestrator to connect to one ProxySQL node. Changes propagate automatically via ProxySQL's cluster synchronization. For non-cluster setups, use `PostMasterFailoverProcesses` script hooks for additional ProxySQL instances.

---

## 6. Observability

### Prometheus Metrics

When `PrometheusEnabled` is `true` (default), orchestrator exposes a `/metrics` endpoint in Prometheus scraping format.

| Metric | Type | Description |
|--------|------|-------------|
| `orchestrator_discoveries_total` | Counter | Total number of discovery attempts |
| `orchestrator_discovery_errors_total` | Counter | Total number of failed discoveries |
| `orchestrator_instances_total` | Gauge | Total number of known instances |
| `orchestrator_clusters_total` | Gauge | Total number of known clusters |
| `orchestrator_recoveries_total` | Counter | Recovery attempts (labels: `type`, `result`) |
| `orchestrator_recovery_duration_seconds` | Histogram | Duration of recovery operations |

#### Prometheus Scrape Configuration

```yaml
scrape_configs:
  - job_name: orchestrator
    static_configs:
      - targets: ['orchestrator:3000']
    metrics_path: /metrics
    scrape_interval: 15s
```

### Health Check Endpoints

| Endpoint | Purpose | Success | Failure |
|----------|---------|---------|---------|
| `GET /health/live` | Liveness probe. Returns 200 if process is running. | `{"status": "alive"}` | N/A (process down) |
| `GET /health/ready` | Readiness probe. Returns 200 if backend DB is connected and health checks pass. | `{"status": "ready"}` | 503 `{"status": "not ready"}` |
| `GET /health/leader` | Leader check. Returns 200 if this is the raft leader or active node. | `{"status": "leader"}` | 503 `{"status": "not leader"}` |

### Additional Health Endpoints (API v1)

| Endpoint | Purpose |
|----------|---------|
| `GET /api/health` | General health check |
| `GET /api/lb-check` | Load-balancer health check |
| `GET /api/_ping` | Same as `lb-check` |
| `GET /api/leader-check` | Leader check for load balancers |
| `GET /api/raft-health` | Raft-specific health check |
| `GET /api/raft-status` | Raft status details |
| `GET /api/status` | Status check (configurable via `StatusEndpoint`) |
| `GET /api/v2/status` | V2 status endpoint |

### Graphite Integration

Configure `GraphiteAddr` and `GraphitePath` to push metrics to Graphite:

```json
{
  "GraphiteAddr": "graphite.example.com:2003",
  "GraphitePath": "orchestrator.{hostname}",
  "GraphiteConvertHostnameDotsToUnderscores": true,
  "GraphitePollSeconds": 60
}
```

### Kubernetes Deployment

```yaml
livenessProbe:
  httpGet:
    path: /health/live
    port: 3000
  initialDelaySeconds: 5
  periodSeconds: 10
readinessProbe:
  httpGet:
    path: /health/ready
    port: 3000
  initialDelaySeconds: 10
  periodSeconds: 5
```

Use `/health/leader` to direct traffic only to the leader in multi-node raft deployments.

### Discovery Metrics API

For internal monitoring, orchestrator exposes raw and aggregated metrics via the v1 API:

| Endpoint | Description |
|----------|-------------|
| `GET /api/discovery-metrics-raw/{seconds}` | Raw discovery metrics for the last N seconds |
| `GET /api/discovery-metrics-aggregated/{seconds}` | Aggregated discovery metrics |
| `GET /api/discovery-queue-metrics-raw/{seconds}` | Raw discovery queue metrics |
| `GET /api/discovery-queue-metrics-aggregated/{seconds}` | Aggregated discovery queue metrics |
| `GET /api/backend-query-metrics-raw/{seconds}` | Raw backend query metrics |
| `GET /api/backend-query-metrics-aggregated/{seconds}` | Aggregated backend query metrics |
| `GET /api/write-buffer-metrics-raw/{seconds}` | Raw write buffer metrics |
| `GET /api/write-buffer-metrics-aggregated/{seconds}` | Aggregated write buffer metrics |

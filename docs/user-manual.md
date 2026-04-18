# Orchestrator User Manual

## Table of Contents

- [Chapter 1: Introduction](#chapter-1-introduction)
- [Chapter 2: Installation and Configuration](#chapter-2-installation-and-configuration)
- [Chapter 3: Topology Discovery](#chapter-3-topology-discovery)
- [Chapter 4: Failure Detection](#chapter-4-failure-detection)
- [Chapter 5: Automated Recovery](#chapter-5-automated-recovery)
- [Chapter 6: ProxySQL Integration](#chapter-6-proxysql-integration)
- [Chapter 7: Monitoring and Observability](#chapter-7-monitoring-and-observability)
- [Chapter 8: High Availability](#chapter-8-high-availability)
- [Chapter 9: API Usage](#chapter-9-api-usage)
- [Chapter 10: Troubleshooting](#chapter-10-troubleshooting)

---

## Chapter 1: Introduction

### What Orchestrator Does

Orchestrator is a MySQL high availability and replication management tool. It runs as a service and provides command line access, an HTTP API, and a web interface. Its three core functions are:

- **Discovery**: Orchestrator actively crawls MySQL replication topologies, mapping master-replica relationships and reading replication status and configuration from each server.
- **Refactoring**: Orchestrator understands replication rules (binlog file:position, GTID, Pseudo-GTID, Binlog Servers) and can safely move replicas between masters. Illegal refactoring attempts are rejected.
- **Recovery**: Orchestrator uses a holistic approach to detect master and intermediate master failures. Based on the state of the topology at the time of failure, it can perform automated or manual failover.

### Architecture Overview

Orchestrator follows a continuous loop architecture:

1. **Discovery loop**: Every few seconds (`InstancePollSeconds`), orchestrator probes each known MySQL instance, reading replication status, server variables, and replica lists. New instances found through replication relationships are automatically added to the discovery queue.

2. **Analysis loop**: Every second, orchestrator runs failure analysis across all known clusters. It cross-references the health of masters with the replication status reported by their replicas to reach conclusions about failures.

3. **Recovery**: When a failure is detected and recovery is enabled for that cluster, orchestrator executes pre-recovery hooks, heals the topology (promotes a replica, rearranges siblings), and executes post-recovery hooks.

Orchestrator stores all topology data in a backend database (MySQL or SQLite). In high-availability deployments, multiple orchestrator nodes coordinate via raft consensus or a shared synchronous database backend.

### Components

Orchestrator exposes three interfaces:

- **HTTP service**: The primary mode of operation. Start with `orchestrator http`. Serves the web UI, the REST API, and runs the continuous discovery/analysis loop. Listens on port 3000 by default.

- **Command line interface (CLI)**: The `orchestrator` binary and the `orchestrator-client` shell script both provide CLI access. `orchestrator-client` is preferred for raft deployments because it communicates via the HTTP API rather than directly accessing the backend database.

- **Web UI**: A browser-based interface for visualizing topologies, dragging replicas between masters, viewing cluster analysis, auditing recoveries, and initiating manual operations.

---

## Chapter 2: Installation and Configuration

### Build from Source

Building orchestrator requires Go and gcc (for SQLite support via cgo):

```bash
# Clone the repository
git clone https://github.com/proxysql/orchestrator.git
cd orchestrator

# Build the binary
./script/build
# Binary is output to bin/orchestrator

# Or build directly with go:
go build -o bin/orchestrator go/cmd/orchestrator/main.go
```

Run unit tests:

```bash
go test ./go/...
```

Build distribution packages (requires fpm):

```bash
./build.sh        # build + package (.deb/.rpm/.tgz)
./build.sh -b     # build only, no packages
```

### Docker Deployment

Orchestrator provides Docker-based workflows through the `script/dock` helper:

```bash
script/dock alpine    # Build and run orchestrator service
script/dock test      # Build, run unit tests, integration tests, doc tests
script/dock pkg       # Build and create distribution packages
script/dock system    # Full CI environment with MySQL topology, HAProxy, Consul
```

### Configuration File Structure

Orchestrator reads configuration from a JSON file. It looks in these locations, in order:

1. `/etc/orchestrator.conf.json`
2. `conf/orchestrator.conf.json`
3. `orchestrator.conf.json`

You can specify a custom location:

```bash
orchestrator --config=/path/to/config.json http
```

The configuration is a single JSON object. A minimal configuration covers the backend database, MySQL topology credentials, and the listen address:

```json
{
  "Debug": false,
  "ListenAddress": ":3000",
  "BackendDB": "sqlite",
  "SQLite3DataFile": "/var/lib/orchestrator/orchestrator.db",
  "MySQLTopologyUser": "orchestrator",
  "MySQLTopologyPassword": "orc_topology_password",
  "InstancePollSeconds": 5,
  "RecoverMasterClusterFilters": ["*"],
  "RecoverIntermediateMasterClusterFilters": ["*"]
}
```

The full list of configuration variables is defined in `go/config/config.go`. Key configuration areas are covered throughout this manual.

### Backend Options: MySQL vs SQLite

Orchestrator supports two backend databases for storing its own topology data:

**SQLite** (simplest, no external dependencies):

```json
{
  "BackendDB": "sqlite",
  "SQLite3DataFile": "/var/lib/orchestrator/orchestrator.db"
}
```

SQLite is embedded within orchestrator. If the file does not exist, orchestrator creates it. The orchestrator process needs write permissions to the specified path.

**MySQL** (better performance for busy setups):

```json
{
  "MySQLOrchestratorHost": "orchestrator.backend.master.com",
  "MySQLOrchestratorPort": 3306,
  "MySQLOrchestratorDatabase": "orchestrator",
  "MySQLOrchestratorCredentialsConfigFile": "/etc/mysql/orchestrator-backend.cnf"
}
```

The credentials config file uses MySQL client format:

```ini
[client]
user=orchestrator_srv
password=${ORCHESTRATOR_PASSWORD}
```

Grant the necessary privileges on the backend MySQL server:

```sql
CREATE USER 'orchestrator_srv'@'orc_host' IDENTIFIED BY 'orc_server_password';
GRANT ALL ON orchestrator.* TO 'orchestrator_srv'@'orc_host';
```

Alternatively, provide credentials directly in the config (less secure):

```json
{
  "MySQLOrchestratorUser": "orchestrator_srv",
  "MySQLOrchestratorPassword": "orc_server_password"
}
```

### Network Requirements

- **Listen port** (default 3000): The HTTP service port. Must be accessible to users, API clients, and other orchestrator nodes in a raft setup.
- **Raft port** (default 10008): Used for inter-node communication in raft deployments. Must be open between all orchestrator nodes.
- **MySQL topology access**: Orchestrator must be able to reach every MySQL server it monitors on their MySQL ports.
- **MySQL backend access** (if using MySQL backend): Orchestrator must reach its backend database.
- **ProxySQL admin port** (default 6032): Required only if ProxySQL integration is enabled.
- **PostgreSQL topology access** (if using PostgreSQL mode): Orchestrator must be able to reach every PostgreSQL instance on its listen port (default 5432).

### PostgreSQL Prerequisites

When managing PostgreSQL topologies instead of MySQL, the following prerequisites apply:

- **PostgreSQL 12+** is required (for `pg_promote()` support).
- Streaming replication must already be configured between primary and standbys.
- All PostgreSQL instances must listen on the same port (configured via `DefaultInstancePort`).
- The orchestrator user needs the `pg_monitor` role on all instances:

```sql
CREATE USER orchestrator WITH PASSWORD 'orch_pass';
GRANT pg_monitor TO orchestrator;
```

- `pg_hba.conf` must allow connections from the orchestrator host on all instances.
- Set `ProviderType` to `"postgresql"` in the orchestrator configuration.

See [Tutorial 5](tutorials.md#tutorial-5-setting-up-orchestrator-with-postgresql-streaming-replication) for a complete walkthrough.

---

## Chapter 3: Topology Discovery

### How Discovery Works

Orchestrator continuously polls known MySQL instances. The process works as follows:

1. On startup, orchestrator has no knowledge of any topology. You must seed it with at least one instance per topology.
2. When orchestrator probes an instance, it reads replication status (`SHOW SLAVE STATUS`) and replica information (`SHOW SLAVE HOSTS` or the process list). It discovers the instance's master and its replicas.
3. Newly discovered instances are added to the discovery queue and will be probed on the next cycle.
4. This recursive crawl maps the entire replication topology from a single seed instance.

Each instance is probed once every `InstancePollSeconds` seconds (default: 5).

You can seed discovery through any of these methods:

```bash
# Via orchestrator-client
orchestrator-client -c discover -i mysql-master.example.com:3306

# Via the API
curl http://orchestrator:3000/api/discover/mysql-master.example.com/3306

# Via a cron job on each MySQL server (recommended for production)
0 0 * * * root "/usr/bin/perl -le 'sleep rand 600' && /usr/bin/orchestrator-client -c discover -i this.hostname.com"
```

To disable the continuous polling (for development/testing only):

```bash
orchestrator --discovery=false http
```

### Configuring MySQL Credentials

Grant orchestrator access to all MySQL topology servers:

```sql
CREATE USER 'orchestrator'@'orc_host' IDENTIFIED BY 'orc_topology_password';
GRANT SUPER, PROCESS, REPLICATION SLAVE, REPLICATION CLIENT, RELOAD ON *.* TO 'orchestrator'@'orc_host';
GRANT SELECT ON meta.* TO 'orchestrator'@'orc_host';
-- Only for NDB Cluster:
GRANT SELECT ON ndbinfo.processes TO 'orchestrator'@'orc_host';
-- Only for Group Replication / InnoDB Cluster:
GRANT SELECT ON performance_schema.replication_group_members TO 'orchestrator'@'orc_host';
```

Configure the credentials in orchestrator's config file. The recommended approach uses a credentials file:

```json
{
  "MySQLTopologyCredentialsConfigFile": "/etc/mysql/orchestrator-topology.cnf",
  "InstancePollSeconds": 5,
  "DiscoverByShowSlaveHosts": false
}
```

Where `/etc/mysql/orchestrator-topology.cnf` contains:

```ini
[client]
user=orchestrator
password=orc_topology_password
```

### Cluster Aliases

By default, orchestrator names clusters after the master's `hostname:port`. You can assign human-readable aliases using a detection query. Create a metadata table on your masters:

```sql
CREATE TABLE IF NOT EXISTS meta.cluster (
  anchor TINYINT NOT NULL,
  cluster_name VARCHAR(128) CHARSET ascii NOT NULL DEFAULT '',
  PRIMARY KEY (anchor)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

INSERT INTO meta.cluster VALUES (1, 'my_production_cluster');
```

Then configure orchestrator:

```json
{
  "DetectClusterAliasQuery": "SELECT cluster_name FROM meta.cluster WHERE anchor = 1"
}
```

Similarly, you can detect data center and other metadata:

```json
{
  "DetectDataCenterQuery": "SELECT dc FROM meta.cluster_info WHERE anchor = 1",
  "DataCenterPattern": "[.]([a-z]{2}[0-9]+)[.]",
  "PhysicalEnvironmentPattern": "[.]([a-z]{2}[0-9]+[-][a-z]+)[.]"
}
```

### Discovery Filters

You can exclude specific hosts from discovery using regex patterns:

```json
{
  "DiscoveryIgnoreHostnameFilters": [
    "utility-node[.]example[.]com:5000",
    ".*[.]staging[.]example[.]com:3306"
  ],
  "DiscoveryIgnoreReplicaHostnameFilters": [
    "backup-replica[.]example[.]com",
    ".*[.]unreachable-dc[.]example[.]com"
  ],
  "DiscoveryIgnoreMasterHostnameFilters": [
    "old-master[.]example[.]com:3306"
  ]
}
```

You can also filter by the replication user name used by a replica:

```json
{
  "DiscoveryIgnoreReplicationUsernameFilters": [
    "debezium_repl",
    "ghost_repl_.*"
  ]
}
```

Control logging of filtered discoveries:

```json
{
  "EnableDiscoveryFiltersLogs": true
}
```

### PostgreSQL Discovery

When `ProviderType` is set to `"postgresql"`, orchestrator uses a PostgreSQL-specific discovery mechanism instead of `SHOW SLAVE STATUS` / `SHOW SLAVE HOSTS`.

**How PostgreSQL discovery works:**

1. Orchestrator connects to a PostgreSQL instance and runs `SELECT pg_is_in_recovery()` to determine whether it is a primary or standby.

2. **For a primary**, orchestrator queries:
   - `SELECT pg_current_wal_lsn()::text` -- current WAL write position
   - `SELECT client_hostname, client_addr, client_port FROM pg_stat_replication` -- connected standbys

3. **For a standby**, orchestrator queries:
   - `SELECT status, conninfo FROM pg_stat_wal_receiver` -- WAL receiver status and connection to the primary
   - `SELECT pg_is_wal_replay_paused()` -- whether WAL replay is paused
   - `SELECT pg_last_wal_replay_lsn()::text` -- current replay position
   - `SELECT EXTRACT(EPOCH FROM now() - pg_last_xact_replay_timestamp())` -- replication lag in seconds

4. The primary's host and port are extracted from the standby's `conninfo` field (the `primary_conninfo` connection string).

**Key differences from MySQL discovery:**

- Discovery does not use `SHOW SLAVE HOSTS` or `PROCESSLIST`. Instead it reads PostgreSQL system views.
- The `client_port` in `pg_stat_replication` is an ephemeral port. Orchestrator uses `DefaultInstancePort` (set this to 5432) for all standby instance keys.
- WAL LSN values are converted to int64 for internal storage and comparison.
- PostgreSQL instances always report `GTIDMode: "ON"` since WAL-based replication is always position-aware.

---

## Chapter 4: Failure Detection

### How Orchestrator Detects Failures

Orchestrator does not rely on simple "can I connect to this server?" checks. Instead, it uses a holistic approach that cross-references information from multiple sources in the replication topology.

For example, to diagnose a `DeadMaster`:

1. Orchestrator fails to contact the master.
2. Orchestrator contacts the master's replicas and confirms they also cannot reach the master (replication is broken on all of them).

This approach triages failures by multiple independent observers rather than by time delays. When all replicas agree their master is unreachable, the replication topology is broken de facto, and failover is justified.

Detection runs every second. It is always enabled and independent of whether recovery is configured.

### Types of Failures

Orchestrator recognizes many failure scenarios. The most important ones:

**Master failures** (can trigger master failover):

| Failure Type | Condition |
|---|---|
| `DeadMaster` | Master unreachable, all replicas have broken replication |
| `DeadMasterAndSomeReplicas` | Master unreachable, some replicas also unreachable, remaining replicas have broken replication |
| `DeadMasterAndReplicas` | Master and all replicas unreachable |
| `DeadMasterWithoutReplicas` | Master unreachable, had no replicas |
| `UnreachableMaster` | Master unreachable but replicas still report replication is working (possible network glitch; triggers emergency re-read of replicas) |
| `UnreachableMasterWithLaggingReplicas` | Master unreachable, all replicas are lagging (possible overloaded master; orchestrator restarts replication on replicas to force a connection test) |
| `AllMasterReplicasNotReplicating` | Master is reachable but none of its replicas are replicating |

**Intermediate master failures** (can trigger intermediate master recovery):

| Failure Type | Condition |
|---|---|
| `DeadIntermediateMaster` | Intermediate master unreachable, all its replicas have broken replication |
| `DeadIntermediateMasterAndSomeReplicas` | Intermediate master and some of its replicas unreachable |
| `DeadIntermediateMasterWithSingleReplica` | Intermediate master unreachable, has a single replica with broken replication |
| `UnreachableIntermediateMaster` | Intermediate master unreachable but its replicas still report healthy replication |

**Semi-sync related:**

| Failure Type | Condition |
|---|---|
| `LockedSemiSyncMaster` | Semi-sync enabled master with insufficient connected semi-sync replicas and a high timeout, causing write locks |
| `MasterWithTooManySemiSyncReplicas` | More semi-sync replicas connected than the configured wait count (requires `EnforceExactSemiSyncReplicas`) |

**Not considered failures**: Simple replica failures (leaf nodes) and replication lag, even severe lag, are not treated as failure scenarios by orchestrator.

### Detection Timing and Configuration

Detection analysis runs every second. Configure anti-spam behavior:

```json
{
  "FailureDetectionPeriodBlockMinutes": 60
}
```

This prevents orchestrator from firing the same detection notification repeatedly. The detection itself still runs; only the hook invocation is throttled.

Configure detection hooks:

```json
{
  "OnFailureDetectionProcesses": [
    "echo 'Detected {failureType} on {failureCluster}. Affected replicas: {countReplicas}' >> /tmp/detection.log"
  ]
}
```

To improve detection speed, configure your MySQL servers:

```sql
-- Short heartbeat interval so replicas detect failure quickly
SET GLOBAL slave_net_timeout = 4;

-- Fast reconnection attempts to recover from brief network issues
CHANGE MASTER TO MASTER_CONNECT_RETRY=1, MASTER_RETRY_COUNT=86400;
```

Without `slave_net_timeout = 4`, some failure scenarios may take up to a minute to detect.

View the current analysis at any time:

```bash
# CLI
orchestrator-client -c replication-analysis

# API
curl http://orchestrator:3000/api/replication-analysis

# Web UI: Clusters -> Failure analysis
```

### PostgreSQL Failure Detection

When running in PostgreSQL mode (`ProviderType: "postgresql"`), orchestrator uses PostgreSQL-specific failure analysis. The analysis query reads from orchestrator's backend database (where instance data was stored during discovery) and produces analysis codes specific to PostgreSQL streaming replication.

**PostgreSQL-specific analysis codes:**

| Analysis Code | Condition |
|---|---|
| `DeadPrimary` | Primary is unreachable, standbys exist but none are replicating. This is the most common failover trigger. |
| `DeadPrimaryAndSomeStandbys` | Primary is unreachable and either no standbys are reachable, or only some standbys are still replicating. |
| `UnreachablePrimary` | Primary is unreachable but all standbys report they are still replicating (possible transient network issue). |
| `AllStandbyNotReplicating` | Primary is reachable but none of its standbys are replicating. |
| `StandbyNotReplicating` | Primary is reachable but one or more standbys are not replicating. |
| `DeadMasterWithoutReplicas` | Primary is unreachable and has no standbys (nothing to fail over to). |

**Differences from MySQL failure detection:**

- No intermediate master failure types (PostgreSQL does not have intermediate masters).
- No co-master failure types (PostgreSQL does not support multi-master replication).
- No semi-sync related failure types.
- No binlog server related failure types.
- The analysis only examines `replication_depth=0` instances (primaries) and their direct standbys.

---

## Chapter 5: Automated Recovery

### Recovery Types

Orchestrator supports failover for topologies using:

- **Oracle GTID** (with `MASTER_AUTO_POSITION=1`)
- **MariaDB GTID**
- **Pseudo-GTID** (orchestrator's own mechanism for non-GTID environments)
- **Binlog Servers**

For GTID and Pseudo-GTID, promotable servers must have `log_bin` and `log_slave_updates` enabled.

Automated recovery is opt-in. Enable it per cluster or globally:

```json
{
  "RecoverMasterClusterFilters": ["*"],
  "RecoverIntermediateMasterClusterFilters": ["*"],
  "RecoveryPeriodBlockSeconds": 3600
}
```

To enable only for specific clusters:

```json
{
  "RecoverMasterClusterFilters": [
    "production-cluster",
    "critical-cluster"
  ],
  "RecoverIntermediateMasterClusterFilters": ["*"]
}
```

A recovery proceeds through these phases:

1. Pre-recovery hooks execute sequentially. If any returns a non-zero exit code, recovery aborts.
2. Orchestrator heals the topology based on its current state (not hard-coded configuration).
3. Post-recovery hooks execute.

### Pre/Post Failover Hooks

Configure hooks as arrays of shell commands:

```json
{
  "PreFailoverProcesses": [
    "/usr/local/bin/check-failover-ok.sh {failureCluster} {failureType}"
  ],
  "PostMasterFailoverProcesses": [
    "/usr/local/bin/update-dns.sh {successorHost}",
    "/usr/local/bin/notify-team.sh 'Master failover on {failureClusterAlias}: {failedHost} -> {successorHost}'"
  ],
  "PostIntermediateMasterFailoverProcesses": [],
  "PostFailoverProcesses": [
    "echo 'Recovered {failureType} on {failureCluster}. Failed: {failedHost}:{failedPort}; Successor: {successorHost}:{successorPort}' >> /tmp/recovery.log"
  ],
  "PostUnsuccessfulFailoverProcesses": [
    "/usr/local/bin/alert-oncall.sh 'FAILED recovery for {failureType} on {failureCluster}'"
  ]
}
```

Hooks receive failure/recovery information through two mechanisms:

**Environment variables** (available in hook scripts):

- `ORC_FAILURE_TYPE`, `ORC_FAILED_HOST`, `ORC_FAILED_PORT`
- `ORC_FAILURE_CLUSTER`, `ORC_FAILURE_CLUSTER_ALIAS`
- `ORC_SUCCESSOR_HOST`, `ORC_SUCCESSOR_PORT` (on successful recovery)
- `ORC_IS_MASTER`, `ORC_COUNT_REPLICAS`, `ORC_LOST_REPLICAS`
- `ORC_COMMAND` (e.g., `graceful-master-takeover`, `force-master-failover`)

**Template variables** (replaced in the command string):

- `{failureType}`, `{failedHost}`, `{failedPort}`
- `{failureCluster}`, `{failureClusterAlias}`
- `{successorHost}`, `{successorPort}` (on successful recovery)
- `{countReplicas}`, `{lostReplicas}`, `{replicaHosts}`

Any command ending with `"&"` executes asynchronously and its failure is ignored.

### Recovery Blocking and Cooldown

Orchestrator prevents cascading failures (flapping) with a blocking period:

```json
{
  "RecoveryPeriodBlockSeconds": 3600
}
```

After a cluster experiences a recovery, no further automated recoveries will run on that same cluster for the specified duration. This block applies only to the same cluster; concurrent recoveries on different clusters are allowed.

Unblock recoveries before the cooldown expires by acknowledging:

```bash
orchestrator-client -c ack-cluster-recoveries -alias mycluster
```

Manual recovery (`orchestrator-client -c recover` or `force-master-failover`) ignores the blocking period.

### Promotion Rules

Control which replicas are preferred for promotion:

```bash
# Register a candidate with a promotion preference (expires after 1 hour)
orchestrator-client -c register-candidate -i replica.example.com --promotion-rule prefer
```

Supported promotion rules: `prefer`, `neutral`, `prefer_not`, `must_not`.

Set up a cron job to keep preferences current:

```
*/2 * * * * root "/usr/bin/perl -le 'sleep rand 10' && /usr/bin/orchestrator-client -c register-candidate -i this.hostname.com --promotion-rule prefer"
```

Additional promotion controls:

```json
{
  "ApplyMySQLPromotionAfterMasterFailover": true,
  "PreventCrossDataCenterMasterFailover": false,
  "PreventCrossRegionMasterFailover": false,
  "FailMasterPromotionIfSQLThreadNotUpToDate": true,
  "DelayMasterPromotionIfSQLThreadNotUpToDate": false,
  "DetachLostReplicasAfterMasterFailover": true,
  "MasterFailoverLostInstancesDowntimeMinutes": 10
}
```

### Graceful Master Takeover

For planned maintenance (upgrades, host migration), use graceful master takeover instead of waiting for a failure:

```bash
# Specify the designated new master
orchestrator-client -c graceful-master-takeover -alias mycluster -d new-master.example.com:3306

# Let orchestrator choose the best replica and start replication on demoted master
orchestrator-client -c graceful-master-takeover-auto -alias mycluster
```

The process:

1. The designated replica takes over its siblings as intermediate master.
2. The current master is set to `read-only`.
3. The designated replica catches up with replication.
4. The designated replica is promoted as the new master and set to writable.
5. The old master is demoted and placed as a replica of the new master.

Dedicated hooks are available:

```json
{
  "PreGracefulTakeoverProcesses": [
    "/usr/local/bin/silence-alerts.sh {failureCluster}"
  ],
  "PostGracefulTakeoverProcesses": [
    "/usr/local/bin/restore-alerts.sh {failureCluster}"
  ]
}
```

These run in addition to the standard failover hooks. Within the standard hooks, check the `{command}` placeholder or `ORC_COMMAND` environment variable for the value `graceful-master-takeover` to distinguish planned from unplanned failovers.

### Manual and Forced Failover

When auto-recovery is disabled or blocked, you can manually trigger recovery:

```bash
# Manual recovery (instance must be recognized as failed)
orchestrator-client -c recover -i dead.instance.com:3306

# Force master failover regardless of orchestrator's analysis
orchestrator-client -c force-master-failover --alias mycluster
```

### PostgreSQL Recovery

When `ProviderType` is `"postgresql"`, orchestrator performs PostgreSQL-specific recovery when a `DeadPrimary` is detected. The recovery process differs significantly from MySQL failover.

**Recovery flow for a dead PostgreSQL primary:**

1. **Pre-failover hooks** execute (same `PreFailoverProcesses` configuration as MySQL).
2. **Read replicas** of the dead primary from orchestrator's backend database.
3. **Select the best standby** for promotion. The selection criteria are:
   - Must have a valid last check (instance was recently reachable)
   - Must not be downtimed
   - If a candidate key is specified and the candidate is valid, it is preferred
   - Otherwise, the standby with the lowest replication lag and highest WAL LSN (most up-to-date) is chosen
4. **Promote the standby** by calling `pg_promote(true, 60)` on the selected standby. Orchestrator waits up to 30 seconds for the instance to exit recovery mode.
5. **Reconfigure remaining standbys** to replicate from the new primary:
   - For each standby (except the promoted one), orchestrator runs `ALTER SYSTEM SET primary_conninfo = '...'` with the new primary's host, port, and credentials
   - Then calls `pg_reload_conf()` to apply the change
   - Pauses and resumes WAL replay to force reconnection to the new primary
6. **Post-failover hooks** execute (`PostMasterFailoverProcesses` and `PostFailoverProcesses`).

**Differences from MySQL recovery:**

- No GTID/Pseudo-GTID coordination is needed. PostgreSQL standbys do not need to "catch up" to a specific binlog position before reparenting.
- No intermediate master recovery. PostgreSQL topologies are flat (primary + standbys).
- Promotion is a single `pg_promote()` call rather than a sequence of `STOP SLAVE` / `RESET SLAVE` / `CHANGE MASTER`.
- Standby reconfiguration uses `ALTER SYSTEM` instead of `CHANGE MASTER TO`.
- Standbys that fail to reconfigure are added to `LostReplicas` but do not block the recovery.
- Graceful master takeover is supported for PostgreSQL. Use `graceful-master-takeover`
  or `graceful-master-takeover-auto` CLI commands, or the equivalent API endpoints.
  The demoted primary requires an operator-managed restart with `standby.signal` —
  configure this via `PostGracefulTakeoverProcesses` hooks.

---

## Chapter 6: ProxySQL Integration

### Setting Up ProxySQL Hooks

Orchestrator has built-in support for updating ProxySQL hostgroups during failover. No custom scripts are needed.

Add the following to `orchestrator.conf.json`:

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

Configuration reference:

| Setting | Default | Description |
|---|---|---|
| `ProxySQLAdminAddress` | (empty) | ProxySQL admin host. Leave empty to disable hooks. |
| `ProxySQLAdminPort` | 6032 | ProxySQL admin port |
| `ProxySQLAdminUser` | admin | Admin interface username |
| `ProxySQLAdminPassword` | (empty) | Admin interface password |
| `ProxySQLAdminUseTLS` | false | Use TLS for admin connection |
| `ProxySQLWriterHostgroup` | 0 | Writer hostgroup ID. Must be > 0 to enable hooks. |
| `ProxySQLReaderHostgroup` | 0 | Reader hostgroup ID (optional) |
| `ProxySQLPreFailoverAction` | offline_soft | Pre-failover action: `offline_soft`, `weight_zero`, or `none` |

### How Failover Updates ProxySQL

**Pre-failover** (when a dead master is detected):

- `offline_soft`: Sets the old master to `OFFLINE_SOFT` in ProxySQL. Existing connections finish but no new ones are routed.
- `weight_zero`: Sets the old master's weight to 0.
- `none`: Skips pre-failover ProxySQL changes.

**Post-failover** (after a new master is promoted):

1. Old master is removed from the writer hostgroup.
2. New master is added to the writer hostgroup.
3. If a reader hostgroup is configured: new master is removed from readers; old master is added to readers as `OFFLINE_SOFT`.
4. `LOAD MYSQL SERVERS TO RUNTIME` and `SAVE MYSQL SERVERS TO DISK` are executed.

The failover timeline:

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

ProxySQL hooks run alongside existing script-based hooks. They are non-blocking: if ProxySQL is unreachable, failover proceeds normally. Post-failover ProxySQL errors are logged but do not mark the recovery as failed.

### ProxySQL Topology API

Query ProxySQL's runtime server list through orchestrator's API:

```bash
# List all servers
curl http://orchestrator:3000/api/proxysql/servers

# List servers in a specific hostgroup
curl http://orchestrator:3000/api/proxysql/servers/10
```

Response format:

```json
{
  "Code": "OK",
  "Message": "Found 4 servers",
  "Details": [
    {
      "hostgroup_id": 10,
      "hostname": "db1.example.com",
      "port": 3306,
      "status": "ONLINE",
      "weight": 1000,
      "max_connections": 100,
      "comment": ""
    }
  ]
}
```

### CLI Commands

```bash
# Test ProxySQL connectivity
orchestrator-client -c proxysql-test

# Show ProxySQL server list
orchestrator-client -c proxysql-servers
```

### Multiple ProxySQL Instances

For ProxySQL Cluster deployments, configure orchestrator to connect to one ProxySQL node. Changes propagate automatically via ProxySQL's built-in cluster synchronization.

For non-clustered ProxySQL, use `PostMasterFailoverProcesses` script hooks to update additional ProxySQL instances.

---

## Chapter 7: Monitoring and Observability

### Prometheus Metrics

Orchestrator exposes a `/metrics` endpoint in Prometheus scrape format. It is enabled by default.

```json
{
  "PrometheusEnabled": true
}
```

Available metrics:

| Metric | Type | Description |
|---|---|---|
| `orchestrator_discoveries_total` | Counter | Total discovery attempts |
| `orchestrator_discovery_errors_total` | Counter | Failed discoveries |
| `orchestrator_instances_total` | Gauge | Known instances |
| `orchestrator_clusters_total` | Gauge | Known clusters |
| `orchestrator_recoveries_total` | Counter | Recovery attempts (labels: `type`, `result`) |
| `orchestrator_recovery_duration_seconds` | Histogram | Duration of recovery operations |

Prometheus scrape configuration:

```yaml
scrape_configs:
  - job_name: orchestrator
    static_configs:
      - targets: ['orchestrator:3000']
    metrics_path: /metrics
    scrape_interval: 15s
```

### Health Endpoints for Kubernetes

Three health check endpoints are provided:

**`GET /health/live`** -- Liveness probe. Returns `200 OK` if the orchestrator process is running. Lightweight; does not query any backend.

```json
{"status": "alive"}
```

**`GET /health/ready`** -- Readiness probe. Returns `200 OK` if the backend database is connected and health check registration is succeeding. Returns `503` otherwise.

```json
{"status": "ready"}
```

**`GET /health/leader`** -- Leader check. Returns `200 OK` if this node is the raft leader (or the active node in non-raft setups). Returns `503` otherwise. Useful for directing writes only to the leader via a load balancer.

```json
{"status": "leader"}
```

Kubernetes deployment example:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: orchestrator
spec:
  template:
    spec:
      containers:
        - name: orchestrator
          ports:
            - containerPort: 3000
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

### Web UI Overview

The web interface (accessible at `http://orchestrator:3000/`) provides:

- **Cluster visualization**: Interactive topology diagrams showing masters, replicas, replication status, and problems.
- **Drag-and-drop refactoring**: Move replicas between masters by dragging them in the topology view.
- **Cluster analysis**: View current failure analysis under Clusters -> Failure analysis.
- **Recovery audit**: Audit past recoveries at `/web/audit-recovery`.
- **Manual actions**: Click on instances to begin downtime, recover, or inspect details.

### Graphite (Legacy)

Orchestrator can also publish metrics to Graphite. This is a legacy feature; Prometheus is the recommended monitoring integration. Refer to the Graphite-related configuration variables in `go/config/config.go` if needed.

---

## Chapter 8: High Availability

Orchestrator itself can be deployed in a highly available configuration. There are two primary approaches.

### Shared Backend Deployment

Multiple orchestrator nodes all connect to the same database backend. The backend must be a synchronous replication cluster:

- Galera
- Percona XtraDB Cluster
- InnoDB Cluster
- NDB Cluster

```
[ orchestrator-1 ] --\
[ orchestrator-2 ] ----> [ Galera / XtraDB Cluster ]
[ orchestrator-3 ] --/
```

Two variations:

- **Single-writer mode**: All orchestrator nodes talk to one writer DB via a proxy. If the writer fails, the synchronous cluster promotes a new writer and the proxy redirects traffic.
- **Multi-writer mode**: Each orchestrator node is paired with a local DB node. Since replication is synchronous, there is no split brain. Only one orchestrator node is the leader.

Access any healthy orchestrator node for API requests. For writes, direct traffic to the leader using `/api/leader-check` as a proxy health check.

### Raft Consensus Deployment

Orchestrator nodes communicate directly via the raft consensus algorithm. Each node has its own private backend database (MySQL or SQLite).

```
[ orchestrator-1 + SQLite ] <--raft--> [ orchestrator-2 + SQLite ] <--raft--> [ orchestrator-3 + SQLite ]
```

Configure on each node:

```json
{
  "RaftEnabled": true,
  "RaftDataDir": "/var/lib/orchestrator",
  "RaftBind": "10.0.0.1",
  "DefaultRaftPort": 10008,
  "RaftNodes": [
    "10.0.0.1",
    "10.0.0.2",
    "10.0.0.3"
  ]
}
```

Each node must have `RaftBind` set to its own address. `RaftNodes` lists all nodes and must be identical across the cluster.

For NAT or firewall scenarios:

```json
{
  "RaftAdvertise": "public-ip-or-fqdn",
  "HTTPAdvertise": "http://my.public.hostname:3000"
}
```

Key behaviors in a raft deployment:

- Only the leader runs recoveries.
- All nodes independently discover and probe MySQL topologies.
- All nodes run failure detection.
- Non-leader nodes reverse-proxy HTTP requests to the leader.
- Clients must only interact with the leader. Use a proxy with `/api/leader-check`, or use `orchestrator-client` with multiple backends (it auto-detects the leader).

Recommended setup: 3 or 5 nodes. SQLite requires no external dependencies; MySQL outperforms SQLite for busy environments.

### Active/Passive Setup

A simpler but less resilient approach uses a MySQL backend with standard replication:

```
[ orchestrator-1 (active) ]  --> [ MySQL master ]
[ orchestrator-2 (standby) ] --> [ MySQL master ]
                                       |
                                 [ MySQL replica ]
```

Multiple orchestrator nodes talk to the same MySQL master. If the MySQL master dies, someone or something else must failover the orchestrator backend. Orchestrator cannot failover its own backend database.

A master-master MySQL backend with a proxy (e.g., HAProxy using `first` algorithm) provides slightly better resilience, but split brain is possible depending on the physical setup and proxy configuration.

---

## Chapter 9: API Usage

### v1 API Overview

The v1 API is accessible under `/api/`. All endpoints use HTTP GET. The web UI is built entirely on these API calls.

Common endpoints:

```bash
# Instance operations
/api/instance/:host/:port          # Read instance details
/api/discover/:host/:port          # Trigger discovery of an instance
/api/relocate/:host/:port/:belowHost/:belowPort  # Move a replica

# Cluster operations
/api/cluster-info/:clusterHint     # Cluster metadata
/api/cluster/alias/:alias          # All instances in a cluster
/api/replication-analysis           # Current failure analysis

# Recovery operations
/api/recover/:host/:port           # Initiate recovery
/api/recover-lite/:host/:port      # Recover without invoking external hooks
/api/force-master-failover/:clusterHint  # Force immediate failover
/api/graceful-master-takeover/:clusterHint/:host/:port  # Planned failover

# Recovery management
/api/audit-recovery                # List past recoveries
/api/blocked-recoveries            # View blocked recoveries
/api/ack-recovery/cluster/:clusterHint  # Acknowledge a recovery
/api/disable-global-recoveries     # Disable all recoveries globally
/api/enable-global-recoveries      # Re-enable recoveries

# Health
/api/leader-check                  # 200 if this node is leader
```

Example workflows:

```bash
# Get cluster info
curl -s "http://orchestrator:3000/api/cluster-info/my_cluster" | jq .

# Find the master of a cluster
curl -s "http://orchestrator:3000/api/cluster/alias/my_cluster" | jq '.[] | select(.ReplicationDepth==0) .Key.Hostname' -r

# Find direct replicas of the master
curl -s "http://orchestrator:3000/api/cluster/alias/my_cluster" | jq '.[] | select(.ReplicationDepth==1) .Key.Hostname' -r

# Find instances without binary logging
curl -s "http://orchestrator:3000/api/cluster/alias/my_cluster" | jq '.[] | select(.LogBinEnabled==false) .Key.Hostname' -r
```

### v2 API with Structured Responses

The v2 API is mounted under `/api/v2/` and provides consistent JSON envelopes, proper HTTP status codes, and RESTful URL structure.

An OpenAPI 3.0 specification is available at `docs/api/openapi.yaml`.

All responses follow this format:

```json
{
  "status": "ok",
  "data": { ... }
}
```

Error responses:

```json
{
  "status": "error",
  "error": {
    "code": "NOT_FOUND",
    "message": "Cluster not found"
  }
}
```

HTTP status codes: `200` success, `400` bad input, `404` not found, `500` internal error, `503` service unavailable.

Available v2 endpoints:

```
GET /api/v2/clusters                        # List all clusters
GET /api/v2/clusters/{name}                 # Cluster details
GET /api/v2/clusters/{name}/instances       # Instances in a cluster
GET /api/v2/clusters/{name}/topology        # ASCII topology
GET /api/v2/instances/{host}/{port}         # Instance details
GET /api/v2/recoveries                      # Recent recoveries (?cluster=, ?alias=, ?page=)
GET /api/v2/recoveries/active               # In-progress recoveries
GET /api/v2/status                          # Node health status
GET /api/v2/proxysql/servers                # ProxySQL server list
```

### Common API Workflows

**Monitor cluster health** -- poll the analysis endpoint and alert on new failures:

```bash
curl -s http://orchestrator:3000/api/replication-analysis | jq '.[] | select(.IsActionableRecovery==true)'
```

**Automate discovery of new clusters** -- call the discover endpoint when provisioning new MySQL servers:

```bash
curl -s http://orchestrator:3000/api/discover/new-server.example.com/3306
```

**Check recovery status after a failover**:

```bash
# v1
curl -s http://orchestrator:3000/api/audit-recovery | jq '.[0]'

# v2
curl -s http://orchestrator:3000/api/v2/recoveries | jq '.data[0]'
```

**Integrate with CI/CD** -- verify no active recoveries before deploying:

```bash
active=$(curl -s http://orchestrator:3000/api/v2/recoveries/active | jq '.data | length')
if [ "$active" -gt 0 ]; then
  echo "Active recovery in progress, delaying deployment"
  exit 1
fi
```

---

## Chapter 10: Troubleshooting

### Common Issues and Solutions

**Orchestrator cannot connect to MySQL topology servers**

- Verify the grants: orchestrator needs `SUPER, PROCESS, REPLICATION SLAVE, REPLICATION CLIENT, RELOAD` on all topology servers.
- Check that `MySQLTopologyCredentialsConfigFile` points to a valid, readable file.
- Ensure network connectivity between the orchestrator host and all MySQL servers on their MySQL ports.
- Check for firewall rules blocking access.

**Orchestrator backend database errors**

- For MySQL backend: verify the backend MySQL server is running and the orchestrator user has `ALL` privileges on the `orchestrator` database.
- For SQLite: verify the directory for `SQLite3DataFile` exists and is writable by the orchestrator process.
- Check the `MySQLOrchestratorMaxAllowedPacket` setting if you see packet-size errors.

**Cluster shows as "unknown" or has no alias**

- Ensure `DetectClusterAliasQuery` is configured and returns a value from each cluster's master.
- The metadata table must exist and be populated on the master.
- Verify orchestrator has `SELECT` privileges on the metadata schema.

**Instances not appearing in topology**

- Seed discovery: `orchestrator-client -c discover -i hostname:port`
- Check `DiscoveryIgnoreHostnameFilters` and related filters to make sure the host is not excluded.
- Verify `DiscoverByShowSlaveHosts` matches your MySQL configuration. If set to `true`, replicas need `report_host` configured.

### Debug Logging

Enable verbose logging with command-line flags:

```bash
# Debug messages
orchestrator --debug http

# Debug messages with stack traces on errors
orchestrator --debug --stack http
```

The `Debug` configuration option can also be set in the config file:

```json
{
  "Debug": true
}
```

### Recovery Not Triggering

If orchestrator detects a failure but does not recover:

1. **Check if recovery is enabled for the cluster**:
   ```bash
   curl -s http://orchestrator:3000/api/cluster-info/mycluster | jq '.HasAutomatedMasterRecovery'
   ```
   Verify `RecoverMasterClusterFilters` includes the cluster name/alias or `"*"`.

2. **Check if global recoveries are disabled**:
   ```bash
   orchestrator-client -c check-global-recoveries
   ```

3. **Check if the instance is downtimed**:
   ```bash
   orchestrator-client -c replication-analysis
   ```
   Downtimed instances are skipped for automated recovery. Use `orchestrator-client -c end-downtime -i hostname:port` to remove downtime.

4. **Check for anti-flapping blocking**:
   ```bash
   curl -s http://orchestrator:3000/api/blocked-recoveries
   ```
   A recent recovery on the same cluster blocks further automated recoveries for `RecoveryPeriodBlockSeconds`. Acknowledge the previous recovery to unblock:
   ```bash
   orchestrator-client -c ack-cluster-recoveries -alias mycluster
   ```

5. **Check the failure type**: Not all failure types trigger recovery. `UnreachableMaster` (where replicas still report healthy replication) does not trigger recovery -- it triggers an emergency re-read of replicas instead.

6. **In raft mode, check that this node is the leader**: Only the leader runs recoveries.
   ```bash
   curl -s http://orchestrator:3000/api/leader-check
   ```

### ProxySQL Hook Failures

ProxySQL hook errors do not block failover. Check the orchestrator log for ProxySQL-related messages.

**ProxySQL not configured error**:
- Verify `ProxySQLAdminAddress` is set (non-empty) and `ProxySQLWriterHostgroup` is greater than 0.

**Connection refused to ProxySQL admin**:
- Verify `ProxySQLAdminPort` (default 6032), `ProxySQLAdminUser`, and `ProxySQLAdminPassword`.
- Test connectivity:
  ```bash
  orchestrator-client -c proxysql-test
  ```

**Changes not reflected in ProxySQL**:
- Orchestrator executes `LOAD MYSQL SERVERS TO RUNTIME` and `SAVE MYSQL SERVERS TO DISK` after changes. Verify these commands succeeded in the log.
- For ProxySQL Cluster setups, changes propagate via `proxysql_servers` synchronization. Verify the cluster is healthy.

**Post-failover ProxySQL update failed but recovery succeeded**:
- ProxySQL post-failover errors are logged but do not mark the MySQL recovery as failed. Manually update ProxySQL if needed:
  ```sql
  -- Connect to ProxySQL admin
  UPDATE mysql_servers SET hostgroup_id=10 WHERE hostname='new-master';
  LOAD MYSQL SERVERS TO RUNTIME;
  SAVE MYSQL SERVERS TO DISK;
  ```

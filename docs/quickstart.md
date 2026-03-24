# Quick Start Guide

Orchestrator is a MySQL high availability and replication management tool that discovers replication topologies, enables refactoring of replica trees, and performs automated or manual failover. It runs as a service with a web UI, HTTP API, and CLI.

This guide gets you from zero to a running orchestrator instance in under 5 minutes.

## Prerequisites

- **Go 1.25+** (for building from source)
- **gcc** (required by the SQLite driver via cgo)
- **MySQL 5.6+ or 8.0+** replication topology to manage (optional for initial setup)
- No external database required -- orchestrator can use a built-in SQLite backend

## Step 1: Build from source

```bash
git clone https://github.com/proxysql/orchestrator.git
cd orchestrator
go build -o bin/orchestrator ./go/cmd/orchestrator
```

Verify the build:

```bash
bin/orchestrator --help
```

## Step 2: Create a minimal configuration

Create a file called `orchestrator.conf.json` in the project root:

```json
{
  "Debug": true,
  "ListenAddress": ":3000",
  "MySQLTopologyUser": "orc_client_user",
  "MySQLTopologyPassword": "orc_client_password",
  "MySQLOrchestratorHost": "",
  "MySQLOrchestratorPort": 0,
  "MySQLOrchestratorDatabase": "",
  "BackendDB": "sqlite",
  "SQLite3DataFile": "/tmp/orchestrator.sqlite3",
  "DefaultInstancePort": 3306,
  "DiscoverByShowSlaveHosts": true,
  "InstancePollSeconds": 5,
  "RecoverMasterClusterFilters": ["*"],
  "RecoverIntermediateMasterClusterFilters": ["*"]
}
```

**Key fields explained:**

| Field | Purpose |
|-------|---------|
| `ListenAddress` | HTTP listen address. `:3000` means all interfaces, port 3000. |
| `MySQLTopologyUser` / `Password` | Credentials orchestrator uses to connect to your MySQL instances. This user needs `SUPER`, `PROCESS`, `REPLICATION SLAVE`, and `REPLICATION CLIENT` privileges. |
| `BackendDB` / `SQLite3DataFile` | Use SQLite as orchestrator's own backend -- no external database needed. |
| `InstancePollSeconds` | How often orchestrator polls each MySQL instance. |
| `RecoverMasterClusterFilters` | Which clusters are eligible for automatic master recovery. `["*"]` enables all. |

> **Tip:** For production deployments, use a MySQL backend instead of SQLite. See the [full configuration docs](configuration.md) for details.

## Step 3: Create a MySQL user on your topology

On your MySQL master (this will replicate to all replicas):

```sql
CREATE USER 'orc_client_user'@'%' IDENTIFIED BY 'orc_client_password';
GRANT SUPER, PROCESS, REPLICATION SLAVE, REPLICATION CLIENT ON *.* TO 'orc_client_user'@'%';
```

## Step 4: Start orchestrator

```bash
bin/orchestrator -config orchestrator.conf.json http
```

You should see output indicating the service has started and is listening on port 3000.

## Step 5: Discover your topology

Tell orchestrator about your MySQL master. Replace `your-master-host` with the actual hostname or IP:

```bash
curl http://localhost:3000/api/discover/your-master-host/3306
```

Orchestrator will connect to this instance, discover its replicas, and recursively crawl the entire topology.

## Step 6: View in the web UI

Open your browser to:

```
http://localhost:3000
```

You will see your MySQL replication topology visualized as an interactive tree. From here you can:

- Click on instances to see their details
- Drag and drop replicas to move them between masters
- View replication lag and errors at a glance

## Step 7: Verify via CLI

List discovered clusters:

```bash
curl http://localhost:3000/api/clusters
```

View the topology as ASCII art:

```bash
curl http://localhost:3000/api/topology/your-master-host/3306
```

Check cluster health:

```bash
curl http://localhost:3000/api/replication-analysis
```

## Next steps

- [Full configuration guide](configuration.md) -- backend database options, discovery tuning, security, and more
- [ProxySQL integration](proxysql-hooks.md) -- built-in failover hooks for ProxySQL
- [API v2 reference](api-v2.md) -- structured REST API with proper HTTP status codes
- [Failure detection & recovery](failure-detection.md) -- how orchestrator detects and recovers from failures
- [High availability](high-availability.md) -- running orchestrator itself in an HA configuration
- [First steps](first-steps.md) -- deeper walkthrough of CLI commands and topology operations
- [Tutorials](tutorials.md) -- step-by-step guides for common workflows

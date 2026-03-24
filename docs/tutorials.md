# Tutorials

Step-by-step guides for common orchestrator workflows.

---

## Tutorial 1: Setting up orchestrator with a MySQL topology

This tutorial walks you through setting up orchestrator to manage an existing MySQL master-replica topology.

### What you will need

- A running MySQL master with one or more replicas (MySQL 5.7+ or 8.0+)
- Go 1.25+ installed
- Network access from the orchestrator host to all MySQL instances on port 3306

### Step 1: Build orchestrator

```bash
git clone https://github.com/proxysql/orchestrator.git
cd orchestrator
go build -o bin/orchestrator ./go/cmd/orchestrator
```

### Step 2: Create a MySQL user for orchestrator

On your MySQL **master** (this will replicate to all replicas automatically):

```sql
CREATE USER 'orc_topology'@'orchestrator-host' IDENTIFIED BY 'a_secure_password';
GRANT SUPER, PROCESS, REPLICATION SLAVE, REPLICATION CLIENT ON *.* TO 'orc_topology'@'orchestrator-host';
```

Replace `orchestrator-host` with the hostname or IP of the machine running orchestrator. Use `%` for any host.

### Step 3: Create a MySQL backend database

For production use, orchestrator should store its data in MySQL rather than SQLite. On a MySQL instance (can be the same master, or a separate server):

```sql
CREATE DATABASE orchestrator;
CREATE USER 'orc_server'@'localhost' IDENTIFIED BY 'another_secure_password';
GRANT ALL ON orchestrator.* TO 'orc_server'@'localhost';
```

### Step 4: Write the configuration file

Create `orchestrator.conf.json`:

```json
{
  "Debug": false,
  "ListenAddress": ":3000",
  "MySQLTopologyUser": "orc_topology",
  "MySQLTopologyPassword": "a_secure_password",
  "MySQLOrchestratorHost": "127.0.0.1",
  "MySQLOrchestratorPort": 3306,
  "MySQLOrchestratorDatabase": "orchestrator",
  "MySQLOrchestratorUser": "orc_server",
  "MySQLOrchestratorPassword": "another_secure_password",
  "DefaultInstancePort": 3306,
  "DiscoverByShowSlaveHosts": true,
  "InstancePollSeconds": 5,
  "ReasonableReplicationLagSeconds": 10,
  "RecoverMasterClusterFilters": ["*"],
  "RecoverIntermediateMasterClusterFilters": ["*"],
  "ApplyMySQLPromotionAfterMasterFailover": true,
  "FailureDetectionPeriodBlockMinutes": 60,
  "RecoveryPeriodBlockSeconds": 3600
}
```

### Step 5: Start orchestrator

```bash
bin/orchestrator -config orchestrator.conf.json http
```

### Step 6: Discover the topology

```bash
curl http://localhost:3000/api/discover/your-master-host/3306
```

Wait a few seconds for orchestrator to crawl the replicas, then verify:

```bash
curl -s http://localhost:3000/api/topology/your-master-host/3306
```

You should see your full replication tree printed as indented text.

### Step 7: Verify in the web UI

Open `http://localhost:3000` in your browser. Click on **Clusters** in the navigation to see your topology visualized as a tree.

### Step 8: Test a topology operation

Move a replica to a different position (dry run with the API):

```bash
# List replicas of the master
curl -s http://localhost:3000/api/instance-replicas/your-master-host/3306
```

You now have a fully operational orchestrator instance managing your MySQL topology.

---

## Tutorial 2: Configuring ProxySQL failover hooks

This tutorial sets up orchestrator to automatically update ProxySQL hostgroups during master failover, so your application traffic is rerouted without any custom scripts.

### Prerequisites

- A working orchestrator setup (see Tutorial 1)
- ProxySQL installed and running with the Admin interface accessible
- Your MySQL servers already configured as backends in ProxySQL

### Step 1: Verify ProxySQL Admin access

```bash
mysql -h 127.0.0.1 -P 6032 -u admin -padmin -e "SELECT * FROM runtime_mysql_servers;"
```

You should see your MySQL servers listed with their hostgroups.

### Step 2: Note your hostgroup IDs

Identify which hostgroup ID is used for writers and which for readers:

```bash
mysql -h 127.0.0.1 -P 6032 -u admin -padmin \
  -e "SELECT hostgroup_id, hostname, port, status FROM runtime_mysql_servers;"
```

For example, if writers are in hostgroup `10` and readers in hostgroup `20`, you will use those values below.

### Step 3: Add ProxySQL settings to orchestrator config

Add these fields to your `orchestrator.conf.json`:

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

| Field | Description |
|-------|-------------|
| `ProxySQLWriterHostgroup` | The hostgroup ID where the current master lives. Must be > 0 to enable hooks. |
| `ProxySQLReaderHostgroup` | The hostgroup ID for read replicas. Optional but recommended. |
| `ProxySQLPreFailoverAction` | What to do with the old master before failover: `offline_soft` (drain connections), `weight_zero`, or `none`. |

### Step 4: Restart orchestrator

```bash
# Stop the running instance (Ctrl+C), then:
bin/orchestrator -config orchestrator.conf.json http
```

### Step 5: Verify ProxySQL connectivity

```bash
curl -s http://localhost:3000/api/proxysql/servers | python3 -m json.tool
```

You should see your ProxySQL server list returned as JSON.

### Step 6: Understand the failover flow

When orchestrator detects a dead master and performs recovery:

1. **Pre-failover:** The old master is set to `OFFLINE_SOFT` in ProxySQL (no new connections)
2. **Topology recovery:** Orchestrator promotes a replica to be the new master
3. **Post-failover:** The new master is added to the writer hostgroup; the old master is removed
4. ProxySQL applies changes immediately via `LOAD MYSQL SERVERS TO RUNTIME`

ProxySQL hooks are non-blocking: if ProxySQL is unreachable, the MySQL failover still proceeds.

### Step 7: Test with a graceful takeover

To verify everything works without an actual failure, perform a graceful master takeover:

```bash
# Identify the current master
curl -s http://localhost:3000/api/clusters

# Perform a graceful takeover (promotes a replica, demotes the master)
curl -s http://localhost:3000/api/graceful-master-takeover/your-cluster-alias/your-new-master-host/3306
```

Check ProxySQL to confirm the hostgroups updated:

```bash
mysql -h 127.0.0.1 -P 6032 -u admin -padmin \
  -e "SELECT hostgroup_id, hostname, port, status FROM runtime_mysql_servers;"
```

For more details, see the full [ProxySQL hooks documentation](proxysql-hooks.md).

---

## Tutorial 3: Monitoring orchestrator with Prometheus

This tutorial sets up Prometheus to scrape orchestrator metrics and shows useful queries for alerting.

### Prerequisites

- A running orchestrator instance
- Prometheus installed (see [prometheus.io/docs](https://prometheus.io/docs/introduction/first_steps/))

### Step 1: Enable Prometheus metrics in orchestrator

Prometheus metrics are enabled by default. Verify by adding this to your `orchestrator.conf.json` (or confirm it is not explicitly disabled):

```json
{
  "PrometheusEnabled": true
}
```

Restart orchestrator if you changed the config.

### Step 2: Verify the metrics endpoint

```bash
curl -s http://localhost:3000/metrics | head -20
```

You should see Prometheus-formatted metrics output.

### Step 3: Configure Prometheus to scrape orchestrator

Add a scrape job to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: orchestrator
    static_configs:
      - targets: ['orchestrator-host:3000']
    metrics_path: /metrics
    scrape_interval: 15s
```

Replace `orchestrator-host` with the actual hostname or IP. Reload Prometheus:

```bash
kill -HUP $(pgrep prometheus)
# or restart the Prometheus service
```

### Step 4: Verify in Prometheus

Open the Prometheus UI (typically `http://prometheus-host:9090`) and query:

```promql
orchestrator_instances_total
```

You should see the number of MySQL instances orchestrator is managing.

### Step 5: Useful queries

**Total known instances and clusters:**

```promql
orchestrator_instances_total
orchestrator_clusters_total
```

**Discovery error rate (over last 5 minutes):**

```promql
rate(orchestrator_discovery_errors_total[5m])
```

**Recovery operations by type:**

```promql
sum by (type) (orchestrator_recoveries_total)
```

**Recovery duration (p95 over last hour):**

```promql
histogram_quantile(0.95, rate(orchestrator_recovery_duration_seconds_bucket[1h]))
```

### Step 6: Set up alerting rules

Create an alerting rule file (e.g., `orchestrator-alerts.yml`):

```yaml
groups:
  - name: orchestrator
    rules:
      - alert: OrchestratorHighDiscoveryErrors
        expr: rate(orchestrator_discovery_errors_total[5m]) > 0.1
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Orchestrator has a high discovery error rate"
          description: "More than 0.1 discovery errors/second for the last 10 minutes."

      - alert: OrchestratorRecoveryOccurred
        expr: increase(orchestrator_recoveries_total[5m]) > 0
        labels:
          severity: critical
        annotations:
          summary: "Orchestrator performed a recovery"
          description: "A failover or recovery event occurred in the last 5 minutes."

      - alert: OrchestratorDown
        expr: up{job="orchestrator"} == 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "Orchestrator is unreachable"
```

Reference this file in your `prometheus.yml`:

```yaml
rule_files:
  - orchestrator-alerts.yml
```

### Step 7: Kubernetes health endpoints

If running orchestrator in Kubernetes, use the built-in health check endpoints for liveness and readiness probes:

```yaml
livenessProbe:
  httpGet:
    path: /api/status
    port: 3000
  initialDelaySeconds: 10
  periodSeconds: 10
readinessProbe:
  httpGet:
    path: /api/status
    port: 3000
  initialDelaySeconds: 5
  periodSeconds: 5
```

For the full list of metrics, see the [Observability documentation](observability.md).

---

## Tutorial 4: Using the API v2

This tutorial introduces the v2 REST API, which provides structured JSON responses and proper HTTP status codes.

### Prerequisites

- A running orchestrator instance with at least one discovered topology

### Step 1: Understand the response format

All v2 endpoints return a consistent JSON envelope:

```json
{
  "status": "ok",
  "data": { ... }
}
```

On errors:

```json
{
  "status": "error",
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable description"
  }
}
```

HTTP status codes (200, 400, 404, 500, 503) are used correctly, unlike the v1 API which always returns 200.

### Step 2: List all clusters

```bash
curl -s http://localhost:3000/api/v2/clusters | python3 -m json.tool
```

Example response:

```json
{
  "status": "ok",
  "data": [
    {
      "clusterName": "master.example.com:3306",
      "clusterAlias": "production",
      "instanceCount": 5
    }
  ]
}
```

### Step 3: Get cluster details

```bash
curl -s http://localhost:3000/api/v2/clusters/master.example.com:3306 | python3 -m json.tool
```

### Step 4: List instances in a cluster

```bash
curl -s http://localhost:3000/api/v2/clusters/master.example.com:3306/instances | python3 -m json.tool
```

### Step 5: Get a specific instance

```bash
curl -s http://localhost:3000/api/v2/instances/replica1.example.com/3306 | python3 -m json.tool
```

### Step 6: View the topology

```bash
curl -s http://localhost:3000/api/v2/clusters/master.example.com:3306/topology | python3 -m json.tool
```

### Step 7: Check orchestrator health

```bash
curl -s -o /dev/null -w "%{http_code}" http://localhost:3000/api/v2/status
```

A `200` response means the node is healthy. A `500` response means it is not.

### Step 8: View recent recoveries

```bash
# All recent recoveries
curl -s http://localhost:3000/api/v2/recoveries | python3 -m json.tool

# Filter by cluster
curl -s "http://localhost:3000/api/v2/recoveries?cluster=master.example.com:3306" | python3 -m json.tool

# Active recoveries only
curl -s http://localhost:3000/api/v2/recoveries/active | python3 -m json.tool
```

### Step 9: Query ProxySQL servers via API v2

If ProxySQL hooks are configured:

```bash
# All servers
curl -s http://localhost:3000/api/v2/proxysql/servers | python3 -m json.tool
```

If ProxySQL is not configured, you will receive a `503` status:

```json
{
  "status": "error",
  "error": {
    "code": "PROXYSQL_NOT_CONFIGURED",
    "message": "ProxySQL is not configured"
  }
}
```

### Step 10: Scripting with the v2 API

The structured responses make scripting straightforward. Example: get all instance hostnames in a cluster using `jq`:

```bash
curl -s http://localhost:3000/api/v2/clusters/master.example.com:3306/instances \
  | jq -r '.data[].Key.Hostname'
```

Check if any recoveries happened in the last hour:

```bash
STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:3000/api/v2/recoveries/active)
if [ "$STATUS" = "200" ]; then
  ACTIVE=$(curl -s http://localhost:3000/api/v2/recoveries/active | jq '.data | length')
  echo "Active recoveries: $ACTIVE"
fi
```

For the full endpoint reference, see the [API v2 documentation](api-v2.md). An [OpenAPI 3.0 specification](api/openapi.yaml) is also available for client generation.

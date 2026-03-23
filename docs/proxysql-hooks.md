# ProxySQL Failover Hooks

Orchestrator has built-in support for updating [ProxySQL](https://proxysql.com) hostgroups during failover. When configured, orchestrator will automatically:

1. **Before failover:** Drain the old master in ProxySQL (set `OFFLINE_SOFT` or `weight=0`)
2. **After failover:** Update ProxySQL hostgroups to route traffic to the new master

No custom scripts needed — orchestrator + ProxySQL works out of the box.

## Configuration

Add these settings to your `orchestrator.conf.json`:

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

### Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `ProxySQLAdminAddress` | (empty) | ProxySQL Admin host. Leave empty to disable hooks. |
| `ProxySQLAdminPort` | 6032 | ProxySQL Admin port |
| `ProxySQLAdminUser` | admin | Admin interface username |
| `ProxySQLAdminPassword` | (empty) | Admin interface password |
| `ProxySQLAdminUseTLS` | false | Use TLS for Admin connection |
| `ProxySQLWriterHostgroup` | 0 | Writer hostgroup ID. Must be > 0 to enable hooks. |
| `ProxySQLReaderHostgroup` | 0 | Reader hostgroup ID. Optional. |
| `ProxySQLPreFailoverAction` | offline_soft | Pre-failover action: `offline_soft`, `weight_zero`, or `none` |

## How It Works

### Pre-Failover

When orchestrator detects a dead master and begins recovery:

- **`offline_soft`**: Sets the old master's status to `OFFLINE_SOFT` in ProxySQL. Existing connections are allowed to complete, but no new connections are routed to it.
- **`weight_zero`**: Sets the old master's weight to 0. Similar effect but preserves the server entry's status.
- **`none`**: Skips pre-failover ProxySQL update.

### Post-Failover

After a new master is promoted:

1. Old master is removed from the writer hostgroup
2. New master is added to the writer hostgroup
3. If reader hostgroup is configured: new master is removed from readers
4. `LOAD MYSQL SERVERS TO RUNTIME` is executed to apply changes immediately
5. `SAVE MYSQL SERVERS TO DISK` is executed to persist changes

### Failover Timeline

```
Dead master detected
  → OnFailureDetectionProcesses (scripts)
    → PreFailoverProcesses (scripts)
      → ProxySQL pre-failover: drain old master     ← NEW
        → [topology manipulation: elect new master]
          → KV store updates (Consul/ZK)
            → ProxySQL post-failover: promote new master  ← NEW
              → PostMasterFailoverProcesses (scripts)
                → PostFailoverProcesses (scripts)
```

ProxySQL hooks run **alongside** existing script-based hooks — they don't replace `PreFailoverProcesses` or `PostFailoverProcesses`.

## CLI Commands

### Test connectivity

```bash
orchestrator-client -c proxysql-test
```

### Show ProxySQL server list

```bash
orchestrator-client -c proxysql-servers
```

## Multiple ProxySQL Instances

For ProxySQL Cluster deployments, configure orchestrator to connect to **one** ProxySQL node. Changes propagate automatically across the cluster via ProxySQL's built-in cluster synchronization (`proxysql_servers` table).

If not using ProxySQL Cluster, you can run multiple orchestrator hook configurations by setting up a ProxySQL load balancer, or by using the existing `PostMasterFailoverProcesses` script hooks for additional ProxySQL instances.

## Interaction with Existing Hooks

ProxySQL hooks are **non-blocking** during pre-failover: if ProxySQL is unreachable, the failover proceeds normally. Post-failover errors are logged but do not mark the recovery as failed.

This ensures that a ProxySQL outage never prevents MySQL failover from completing.

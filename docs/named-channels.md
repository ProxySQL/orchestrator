# Named Replication Channels

MySQL 5.7+ supports multi-source replication, where a single replica can replicate from multiple masters simultaneously. Each replication connection is identified by a **named channel**. Orchestrator supports discovery, management, and failover of instances that use named replication channels.

## Overview

In a traditional single-source topology, a replica has exactly one `SHOW SLAVE STATUS` row. With multi-source replication, each channel appears as a separate row in `SHOW SLAVE STATUS`, each with its own IO thread, SQL thread, binlog coordinates, and lag.

Orchestrator handles multi-source replicas by:

1. Discovering all replication channels on each instance.
2. Selecting one channel as the **managed channel** for topology purposes.
3. Using channel-aware SQL operations (`FOR CHANNEL`) during failover and topology changes.
4. Preserving non-managed channels during promotion and recovery.

## Discovery

When orchestrator reads an instance's replication status via `SHOW SLAVE STATUS`, it parses all rows. Each row becomes a `ChannelStatus` entry stored in the instance's `ReplicationChannels` slice.

For single-source instances (0 or 1 channels), behavior is identical to previous versions. For multi-source instances (2+ channels), orchestrator selects a **canonical channel** to represent the instance's primary replication relationship:

1. The default channel (empty name `""`) is preferred.
2. Otherwise, the first non-Group-Replication-internal channel is selected.
3. As a fallback, the first channel is used.

The selected channel's status (IO/SQL thread state, binlog coordinates, lag, master key, etc.) populates the instance's top-level replication fields. This ensures backward compatibility with all existing topology logic.

## Managed Channel Name

The `ManagedChannelName` field on an instance indicates which channel orchestrator manages for topology operations. When this field is empty, all operations use standard SQL without a `FOR CHANNEL` clause (single-source behavior).

When `ManagedChannelName` is set (multi-source instances), orchestrator appends `FOR CHANNEL '<name>'` to replication commands, ensuring only the managed channel is affected. Other channels remain untouched.

## Channel-Aware Operations

The following operations are channel-aware:

- `STOP SLAVE` / `STOP REPLICA` -- stops only the managed channel
- `START SLAVE` / `START REPLICA` -- starts only the managed channel
- `CHANGE MASTER TO` -- targets only the managed channel
- `RESET SLAVE` / `RESET REPLICA` -- resets only the managed channel

All of these append `FOR CHANNEL '<name>'` when a channel name is specified.

## Failover with Multi-Source Replicas

### Dead Master Recovery

When a master dies and orchestrator initiates recovery (`recoverDeadMaster`), replicas are regrouped using GTID or Pseudo-GTID. The underlying `StopReplication`, `ChangeMasterTo`, and `StartReplication` calls all respect the `ManagedChannelName`, so only the dead master's channel is affected on multi-source replicas. Other channels (replicating from other masters) continue operating normally.

### Candidate Selection

A multi-source replica where the dead master is one of its replication channels is a valid promotion candidate. During promotion, only the managed channel is modified; all other channels are preserved.

### Graceful Master Takeover

`GracefulMasterTakeover` uses `ChangeMasterToForChannel` and `StartReplicationForChannel` with the managed channel name. This ensures that when the demoted master is reconfigured to replicate from the promoted instance, only the relevant channel is set up, and any other channels on the demoted master remain intact.

## API Endpoints

### V1 API

- `GET /api/instance-channels/{host}/{port}` -- Returns the `ReplicationChannels` slice as JSON for the given instance. Each entry includes channel name, master key, IO/SQL thread state, binlog coordinates, lag, and error information.

- `GET /api/instance/{host}/{port}` -- The standard instance endpoint now includes `ReplicationChannels` and `ManagedChannelName` in its JSON response.

### V2 API

- `GET /api/v2/instances/{host}/{port}/channels` -- Returns channels in the V2 response envelope (`{"status": "ok", "data": [...]}`).

## Group Replication Channels

Group Replication uses internal channels named `group_replication_applier` and `group_replication_recovery`. These are automatically detected and excluded from canonical channel selection. Orchestrator will not select a GR internal channel as the managed channel unless no other channels exist.

## Limitations

- Orchestrator manages exactly one channel per instance for topology purposes. Manual management of other channels is expected.
- Channel-aware operations require MySQL 5.7+ or MariaDB 10.1+ (which support the `FOR CHANNEL` syntax).
- The backend database stores channel information in the `database_instance_channels` table. Ensure schema migrations have been applied.
- Multi-source replicas where multiple channels point to the same cluster may cause unexpected behavior in topology analysis.

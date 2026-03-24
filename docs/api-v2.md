# API v2

Orchestrator provides a versioned REST API at `/api/v2/` with consistent structured JSON responses.

The v1 API (`/api/`) remains fully available and is not deprecated. The v2 API provides a standardized response envelope and proper HTTP status codes.

## Response Envelope

All v2 endpoints return responses in a standard JSON envelope:

### Success response

```json
{
  "status": "ok",
  "data": { ... },
  "message": "optional human-readable message"
}
```

### Error response

```json
{
  "status": "error",
  "error": {
    "code": "MACHINE_READABLE_CODE",
    "message": "Human-readable error description"
  }
}
```

Fields with empty/nil values are omitted from the response (`omitempty`).

### HTTP Status Codes

Unlike the v1 API which returns 200 for most responses, v2 uses proper HTTP status codes:

| Code | Meaning |
|------|---------|
| 200  | Success |
| 400  | Bad request (invalid parameters) |
| 404  | Resource not found |
| 500  | Internal server error |
| 503  | Service unavailable (unhealthy) |

### Error Codes

| Code | Description |
|------|-------------|
| `NOT_FOUND` | The requested resource does not exist |
| `INVALID_PARAMS` | Request parameters are invalid |
| `INTERNAL_ERROR` | An internal server error occurred |
| `UNHEALTHY` | The orchestrator node is unhealthy |
| `NOT_CONFIGURED` | A required component is not configured |

## Endpoints

### Clusters

#### `GET /api/v2/clusters`

List all known clusters with metadata.

**Response:**
```json
{
  "status": "ok",
  "data": [
    {
      "ClusterName": "myhost:3306",
      "ClusterAlias": "mycluster",
      "ClusterDomain": "",
      "CountInstances": 3,
      "HeuristicLag": 0,
      "HasAutomatedMasterRecovery": true,
      "HasAutomatedIntermediateMasterRecovery": true
    }
  ]
}
```

#### `GET /api/v2/clusters/{alias}`

Get detailed info for one cluster, looked up by alias or cluster name.

**Response:**
```json
{
  "status": "ok",
  "data": {
    "ClusterName": "myhost:3306",
    "ClusterAlias": "mycluster",
    "CountInstances": 3
  }
}
```

**Error (404):**
```json
{
  "status": "error",
  "error": {
    "code": "NOT_FOUND",
    "message": "Cluster not found: unknown-alias"
  }
}
```

#### `GET /api/v2/clusters/{alias}/instances`

List all instances belonging to a cluster.

**Response:**
```json
{
  "status": "ok",
  "data": [
    {
      "Key": {"Hostname": "myhost", "Port": 3306},
      "ClusterName": "myhost:3306",
      "IsReplica": false,
      "ReplicationDepth": 0
    }
  ]
}
```

### Instances

#### `GET /api/v2/instances/{host}/{port}`

Get detailed information about a single MySQL instance.

**Response:**
```json
{
  "status": "ok",
  "data": {
    "Key": {"Hostname": "myhost", "Port": 3306},
    "Uptime": 86400,
    "ClusterName": "myhost:3306",
    "IsReplica": false,
    "ReplicationDepth": 0,
    "Replicas": []
  }
}
```

**Error (404):**
```json
{
  "status": "error",
  "error": {
    "code": "NOT_FOUND",
    "message": "Instance not found: myhost:3306"
  }
}
```

**Error (400):**
```json
{
  "status": "error",
  "error": {
    "code": "INVALID_PARAMS",
    "message": "Invalid instance key: badhost:badport"
  }
}
```

### Topology

#### `GET /api/v2/topology/{clusterAlias}`

Returns the topology (instances with replication relationships) for a cluster. The response contains all instances in the cluster; replication relationships can be derived from each instance's `MasterKey` and `Replicas` fields.

**Response:**
```json
{
  "status": "ok",
  "data": [
    {
      "Key": {"Hostname": "master", "Port": 3306},
      "MasterKey": {"Hostname": "", "Port": 0},
      "Replicas": [{"Hostname": "replica1", "Port": 3306}],
      "ReplicationDepth": 0
    },
    {
      "Key": {"Hostname": "replica1", "Port": 3306},
      "MasterKey": {"Hostname": "master", "Port": 3306},
      "Replicas": [],
      "ReplicationDepth": 1
    }
  ]
}
```

### Recoveries

#### `GET /api/v2/recoveries`

List recent recovery entries. Supports optional query parameters:

| Parameter | Type | Description |
|-----------|------|-------------|
| `cluster` | string | Filter by cluster name |
| `alias` | string | Filter by cluster alias |
| `unacknowledged` | boolean | If `true`, only show unacknowledged recoveries |
| `page` | integer | Page number for pagination (0-indexed) |

**Example:** `GET /api/v2/recoveries?cluster=myhost:3306&unacknowledged=true`

**Response:**
```json
{
  "status": "ok",
  "data": [
    {
      "Id": 1,
      "AnalysisEntry": { ... },
      "SuccessorKey": {"Hostname": "new-master", "Port": 3306},
      "IsActive": false,
      "IsSuccessful": true
    }
  ]
}
```

#### `GET /api/v2/recoveries/active`

Returns currently active (in-progress) recoveries.

**Response:**
```json
{
  "status": "ok",
  "data": []
}
```

### Health / Status

#### `GET /api/v2/status`

Returns orchestrator node health and status information.

**Success Response (200):**
```json
{
  "status": "ok",
  "data": {
    "Healthy": true,
    "Hostname": "orch-node-1",
    "Token": "abc123",
    "IsActiveNode": true,
    "RaftLeader": "",
    "IsRaftLeader": false
  },
  "message": "Application node is healthy"
}
```

**Unhealthy Response (503):**
```json
{
  "status": "error",
  "error": {
    "code": "UNHEALTHY",
    "message": "Application node is unhealthy: ..."
  }
}
```

### ProxySQL

#### `GET /api/v2/proxysql/servers`

Returns ProxySQL server entries from the runtime_mysql_servers table.

**Response:**
```json
{
  "status": "ok",
  "data": [
    {
      "Hostgroup": 10,
      "Hostname": "myhost",
      "Port": 3306,
      "Status": "ONLINE",
      "Weight": 1000
    }
  ],
  "message": "Found 1 servers"
}
```

**Error (404 - not configured):**
```json
{
  "status": "error",
  "error": {
    "code": "NOT_CONFIGURED",
    "message": "ProxySQL is not configured"
  }
}
```

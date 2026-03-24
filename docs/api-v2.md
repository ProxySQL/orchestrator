# API v2

Orchestrator API v2 provides a cleaner, more consistent REST API with structured JSON responses and proper HTTP status codes.

All v2 endpoints are mounted under `/api/v2` (respecting the configured `URLPrefix`).

## Response Format

All v2 endpoints return a consistent JSON envelope:

### Success

```json
{
  "status": "ok",
  "data": { ... }
}
```

### Error

```json
{
  "status": "error",
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error description"
  }
}
```

HTTP status codes are used appropriately:

- `200` for successful requests
- `400` for invalid input (bad instance key, etc.)
- `404` for resources not found (unknown cluster, instance, etc.)
- `500` for internal server errors
- `503` for unavailable services (e.g., ProxySQL not configured)

## Endpoints

### Clusters

#### `GET /api/v2/clusters`

Returns a list of all known clusters with metadata.

**Response:** Array of cluster info objects including cluster name, alias, master instance, count of instances, etc.

#### `GET /api/v2/clusters/{name}`

Returns detailed information about a specific cluster. The `{name}` parameter can be a cluster name, alias, or instance key hint.

**Response:** Single cluster info object.

**Errors:**
- `404` if the cluster cannot be resolved.

#### `GET /api/v2/clusters/{name}/instances`

Returns all instances belonging to a given cluster.

**Response:** Array of instance objects.

**Errors:**
- `404` if the cluster cannot be resolved.

#### `GET /api/v2/clusters/{name}/topology`

Returns the ASCII topology representation for a cluster.

**Response:**

```json
{
  "status": "ok",
  "data": {
    "clusterName": "my-cluster:3306",
    "topology": "..."
  }
}
```

**Errors:**
- `404` if the cluster cannot be resolved.

### Instances

#### `GET /api/v2/instances/{host}/{port}`

Returns detailed information about a specific MySQL instance.

**Response:** Single instance object with replication status, version info, etc.

**Errors:**
- `400` if the instance key is invalid.
- `404` if the instance is not found.

### Recoveries

#### `GET /api/v2/recoveries`

Returns recent recovery entries.

**Query Parameters:**
- `cluster` (optional): filter by cluster name
- `alias` (optional): filter by cluster alias
- `page` (optional): page number for pagination (default: 0)

**Response:** Array of recovery objects.

#### `GET /api/v2/recoveries/active`

Returns currently active (in-progress) recoveries.

**Response:** Array of active recovery objects.

### Status

#### `GET /api/v2/status`

Returns the health status of the orchestrator node.

**Response:** Health status object including hostname, active node info, raft status, etc.

**Errors:**
- `500` if the node is unhealthy.

### ProxySQL

#### `GET /api/v2/proxysql/servers`

Returns all servers from ProxySQL's `runtime_mysql_servers` table.

**Response:** Array of ProxySQL server entry objects.

**Errors:**
- `503` if ProxySQL is not configured.
- `500` if querying ProxySQL fails.

## Migration from API v1

The v2 API wraps the same underlying functions as v1 but provides:

1. **Consistent response envelope**: All responses use the `V2APIResponse` struct with `status`, `data`, `error`, and `message` fields.
2. **Proper HTTP status codes**: v1 always returned 200 with error info in the body; v2 uses appropriate 4xx/5xx codes.
3. **RESTful URL structure**: Resources are nested logically (e.g., `/clusters/{name}/instances` instead of `/cluster/{clusterHint}`).
4. **Structured error codes**: Machine-readable error codes (e.g., `NOT_FOUND`, `CLUSTER_LIST_ERROR`) alongside human-readable messages.

The v1 API remains fully functional. Both APIs can be used simultaneously.

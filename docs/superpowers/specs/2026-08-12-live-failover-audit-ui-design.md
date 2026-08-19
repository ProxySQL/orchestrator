# Live Failover and Audit UI Verification Design

## Objective

Exercise Orchestrator's history UI with real operational data from the existing three-node MySQL Docker lab. The pass must populate and inspect general audit operations, failure detections, recovery summaries, and a recovery detail page without recreating or deleting the MySQL containers or their volumes.

## Chosen approach

Use one controlled hard-primary failure because it produces the complete state chain needed by the UI:

1. enable backend audit persistence in the functional Orchestrator configuration;
2. recreate only the Orchestrator container with `--no-deps`;
3. rediscover the healthy `mysql1` primary with `mysql2` and `mysql3` replicas;
4. stop only `mysql1`;
5. wait for `DeadMaster` detection and a successful automated recovery;
6. restart `mysql1` and restore the original lab topology and ProxySQL hostgroups;
7. retain the Orchestrator SQLite session so its audit and recovery history remains available for browser testing.

This is preferable to a synthetic fixture because it validates the browser against the real API payloads. It is preferable to a forced logical failover because a logical failover does not cover the actual failure-detection history.

## Safety boundaries

- Record container IDs, health, topology roles, and ProxySQL writer state before mutation.
- Never run a broad `docker compose up`, `down`, volume removal, or dependency recreation.
- Recreate only Orchestrator and always use `--no-deps`.
- Stop and restart only the resolved current primary container.
- Preserve all MySQL volumes and container identities.
- Use bounded waits for detection, recovery, restart, and topology restoration.
- If recovery fails, restart the stopped primary immediately, capture diagnostics, and restore the original topology before further investigation.
- Confirm the original three MySQL container IDs and healthy state after restoration.

## UI states to verify

### Audit operations

- At least one table row renders with timestamp, operation type, instance, and message.
- Pagination controls remain disabled/enabled according to available pages.
- Instance links use working registered routes.
- Long messages and identifiers do not create document-level horizontal overflow.

### Failure detections

- A `DeadMaster` detection row renders with failed instance, affected replicas, cluster, and detection time.
- Expanding the detection reveals recorded context and a working related-recovery link.
- API-derived values are displayed as text rather than executable markup.

### Recoveries

- A successful recovery row renders with failed instance and promoted successor.
- Opening the recovery renders the summary, acknowledgement state, related detection, and recovery steps.
- Empty, populated, and unavailable states remain mutually exclusive.

### Existing topology pages

- Cluster dashboard and topology reflect the promoted primary during recovery.
- After cleanup, the lab returns to `mysql1` as writable primary and `mysql2`/`mysql3` as running replicas.

## Remediation policy

Browser defects discovered with populated data will be reproduced by a focused failing test before production changes. Fixes will preserve existing API contracts and routes, keep CSS scoped below the history workspace, escape API-derived content, and retain narrow-screen table scrolling without document overflow.

## Verification

- Focused regression tests for every discovered defect, including a demonstrated red-to-green cycle.
- Full `go test ./go/http -count=1`.
- All Node UI behavior tests and JavaScript syntax checks.
- Functional smoke suite after topology restoration.
- Browser audit at desktop and 390px widths for the three populated history pages and recovery detail.
- Browser console audit for errors and warnings.
- Pre/post MySQL container-ID comparison and final role/replication checks.

## Out of scope

- PostgreSQL failover UI verification.
- Agent-enabled UI verification.
- Redesigning the topology workspace or global navigation.
- Acknowledging or deleting the generated recovery record solely to make the page look cleaner.

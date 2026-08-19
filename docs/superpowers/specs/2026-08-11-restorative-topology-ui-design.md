# Restorative Topology UI Design

## Goal

Restore a coherent, operator-first cluster-detail experience without changing Orchestrator's topology APIs, recovery behavior, or existing operational semantics. The result should recover the clarity of the historical UI: a topology canvas is the focal point, and each instance is readable at a glance.

## Scope

The first release applies only to the cluster-detail workspace. It covers the shared chrome needed by that page, the cluster template, topology presentation, node cards, and cluster-scoped styling.

It does not redesign non-cluster pages, change API response formats, replace recovery logic, remove drag/drop capabilities, or merge unrelated pending pull requests.

## Design Direction

The interface uses a compact dark application bar, a narrow preference rail, and a light topology canvas. This preserves the historical screen's information hierarchy while replacing its inconsistent Bootstrap-3/Bootstrap-5 hybrid appearance.

The canvas displays the cluster name and a concise health summary above the topology. Replication links are visually quiet. Instance cards, rather than the framework chrome, carry operational meaning.

## Node Cards

Each visible instance card always shows:

- Hostname and port
- Role: primary or replica
- Reachability/health state
- Replication lag
- Version
- Writable or read-only state

Healthy cards are intentionally quiet. Warning, stale, and fatal cards have progressively stronger state treatment that remains understandable without relying on color alone. Existing actions remain available through one consistent overflow menu and the existing details/modal flow. Recovery and failover remain deliberate, confirmed operations.

The initial release preserves the current interaction contracts, including existing action endpoints and drag/drop behavior. It changes presentation and action discoverability, not operational authority.

## Implementation Architecture

The existing cluster API and topology model remain the source of truth. The client continues to fetch and render the same instance data.

The implementation creates a cluster-page visual boundary:

1. Update the cluster page's markup and scripts to use a small, page-specific component structure.
2. Replace Glyphicon-dependent controls in that page with maintained, locally served icons or accessible text labels.
3. Add cluster-scoped CSS for the application bar, preference rail, canvas, links, stateful node cards, and responsive behavior.
4. Preserve the current D3 tree geometry for the first release. A later renderer replacement can target that boundary without changing the topology API or the rest of the application.

No new global CSS compatibility shim is introduced. Any Bootstrap compatibility work remains isolated to existing legacy screens so the cluster view does not deepen the current hybrid dependency state.

## State and Failure Handling

While topology data is loading, the canvas shows a restrained loading state. If topology data is unavailable or stale, the page retains its structural layout and presents an explicit, non-destructive warning. Action controls remain unavailable until their target instance and current state have loaded.

Node health, lag, and maintenance conditions use existing backend values. The UI does not infer recovery safety or alter failover eligibility.

## Responsive Behavior

Desktop retains the horizontal topology canvas. At narrow widths, the application bar condenses, the preference rail becomes a compact control group, and node cards stack by replication depth without truncating hostname, role, or status information.

## Verification

The Docker Compose lab provides three instances: `mysql1` as primary and `mysql2`/`mysql3` as replicas. Verification includes:

- Cluster page renders against the real three-node topology.
- Primary/replica role, lag, read-only state, and links are legible.
- Existing node details and allowed actions remain reachable.
- Failure and stale-state visual treatment renders without hiding topology context.
- Desktop and narrow viewport layouts do not overlap or clip node content.
- Existing HTTP/template and relevant API tests continue to pass.

## Deferred Work

A full D3 renderer replacement, dashboard/list-page redesign, audit-page redesign, and broader dependency cleanup are separate follow-on projects. They must not be bundled into this restoration.

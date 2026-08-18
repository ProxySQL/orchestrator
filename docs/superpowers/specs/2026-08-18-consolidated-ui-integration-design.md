# Consolidated UI Integration Design

## Goal

Consolidate the useful work from UI PRs 122, 125, and 126 into one coherent Orchestrator interface that is visually consistent, operationally safe, responsive, and maintainable. PR 122 remains the integration branch because it already supplies the semantic workspaces and live-tested operational flows. PRs 125 and 126 are treated as source material, not merged wholesale.

## Current Evidence

The Docker lab was started with three MySQL instances and the PR 122 UI was inspected in the in-app browser at the normal 794-pixel application width. The Clusters, topology, failure-analysis, Audit, and Status pages render populated data without console errors. The strongest parts are the semantic topology cards, restrained operational color palette, explicit loading and empty states, and consistent workspace panels.

The remaining visible weaknesses are concentrated in the global shell and legacy compatibility layer: navigation becomes crowded before it collapses, Glyphicon-dependent controls are inconsistent, typography and spacing vary between legacy and redesigned pages, and interaction compatibility is spread between templates and application JavaScript.

PR 125 provides useful Bootstrap Icons, local Bootstrap 5 assets, cache-busting, and a D3 v7 port, but it is an independent one-commit rewrite that conflicts with PR 122 in seventeen UI files. Its current review also identifies an asset-version ordering bug, duplicate modal behavior, a tree-coordinate regression, and accessibility details. PR 126 restores a Bootstrap 3/IE8 shell and obsolete Openark destinations, so its rollback is not compatible with the chosen direction.

## Integration Policy

1. PR 122 is the only implementation base and remains the final integration PR.
2. Valuable PR 125 changes are reapplied deliberately in small, reviewable commits with focused regression tests. The PR 125 commit is not merged or cherry-picked wholesale.
3. PR 126 is not merged. Its valid intent—local, reliable framework assets and working legacy interactions—is already covered by the Bootstrap 5 compatibility design below.
4. PRs 125 and 126 remain open during implementation. After verification, each receives a concise record of what was incorporated or rejected. Closing or superseding either PR remains a maintainer decision.

## Visual System

The final interface keeps PR 122's charcoal chrome, warm neutral page background, white operational panels, blue primary actions, orange product accent, and state colors reserved for health and severity.

The global header becomes the single source of visual navigation behavior:

- the ProxySQL Orchestrator brand remains left aligned and links to the current repository;
- Home, Clusters, and Audit remain the primary navigation groups;
- search is a compact labelled control rather than an isolated narrow field;
- recovery, read-only, refresh, user, and problem indicators remain grouped at the right;
- navigation collapses at the large breakpoint so the current 794-pixel browser width receives a usable menu rather than a crowded desktop row;
- every icon-only action has an accessible name, visible focus state, and a text tooltip where useful.

Bootstrap Icons replace Glyphicon presentation in the active Bootstrap 5 shell and redesigned pages. Text labels remain for important operational actions, so meaning never depends on icon shape or color alone.

The semantic PR 122 workspaces remain intact. This integration standardizes their header heights, content widths, vertical rhythm, panel borders, table density, buttons, and responsive behavior without reverting them to generic Bootstrap examples.

## Asset and Compatibility Architecture

PR 122's existing local `/bootstrap5` assets remain the canonical Bootstrap bundle. This pass adds the local Bootstrap Icons font but does not delete the old `/bootstrap` directory; removing unused vendor assets is separate cleanup and is not required to make the interface correct.

A single global asset version is injected into render data before either the content template or layout template executes. All first-party CSS and JavaScript references use that token. This replaces scattered date-based query strings and prevents a layout from receiving a value that content templates cannot see.

Bootstrap 5 compatibility behavior is moved out of the layout template into one small `bootstrap-legacy-bridge.js` module. It owns only:

- normalization of required legacy `data-toggle`, `data-target`, and `data-dismiss` attributes;
- the limited jQuery wrappers still used by Orchestrator for modals, dropdowns, popovers, tooltips, and alerts;
- idempotent initialization so DOM-ready processing cannot register handlers twice.

Application behavior remains in the existing page scripts. The bridge must not own topology, recovery, audit, or modal business logic.

## D3 Topology Migration

PR 125's D3 v7 migration is incorporated as a behavior-preserving modernization, not a graph redesign. The port must retain:

- the topology hierarchy and link geometry;
- collapse and expand behavior;
- node dragging and move-equivalent actions;
- stable transition origins through `x0` and `y0`;
- recalculated `x` and `y` positions after every hierarchy update;
- PR 122's semantic node-card markup and responsive canvas sizing.

The old D3 v3 asset is removed only after automated graph-contract tests and browser collapse, expand, drag-exclusion, and modal checks pass against D3 v7. If parity cannot be demonstrated within this work, the icon, shell, and cache fixes remain independently shippable and the D3 migration stays in PR 125.

## Page and Interaction Scope

Browser review covers every route exposed by the global navigation plus the populated topology route:

- Clusters and cluster topology;
- Failure analysis;
- Discover and Search;
- Audit operations, failure detections, recoveries, and recovery detail;
- Status, About, FAQ, Agents, and Seeds;
- node details, View menu, problem dropdown, navigation dropdowns, modal dismissal, audit pagination, and topology collapse/expand.

Backend discovery, recovery decisions, authorization, polling intervals, and API response contracts are unchanged. The lab may seed Orchestrator discovery metadata, but UI validation must not deliberately change MySQL roles or run a failover unless a separate test explicitly requires it.

## Data Flow and Error Handling

The server renders a layout and content template with the same asset version and provider-aware template data. The browser loads local framework CSS, icon CSS, the Bootstrap bundle, the compatibility bridge, shared application scripts, and finally page-specific assets in a deterministic order.

Page scripts retain the existing APIs and intentional loading, populated, empty, and unavailable states. A failed API request must become a visible unavailable state rather than a healthy empty state. Missing icons or optional visual assets must not block navigation or operational actions. Duplicate initialization must not produce duplicate requests, modal openings, or command execution.

## Testing and Browser Verification

Implementation follows red-green TDD. Focused tests cover:

- asset-version availability in both layout and content templates;
- one-time compatibility-bridge initialization and delegated handlers;
- registered navigation destinations and current repository/documentation links;
- accessible icon-only controls and responsive navigation hooks;
- modal, dropdown, and problem-panel behavior;
- D3 v7 hierarchy coordinates, collapse/expand state, and semantic card preservation;
- absence of external Bootstrap CDN dependencies;
- absence of obsolete Openark links in the rendered shell.

The complete Go, JavaScript, shell, and Docker smoke suites run after focused tests. Browser verification uses populated lab data at desktop width, the current 794-pixel application width, and 390-by-844 mobile width. Each route is checked for readable hierarchy, body-level overflow, console errors, working navigation, focusable controls, and stable populated or empty states. Code changes are followed by an explicit browser reload before visual judgment.

## Acceptance Criteria

- PR 122 presents one visually coherent product across all primary routes.
- The header is uncluttered and fully usable at desktop, 794-pixel, and mobile widths.
- Locally served Bootstrap 5 and Bootstrap Icons load without external CDN dependencies.
- Legacy modal, dropdown, popover, alert, and dismissal interactions work once per action.
- The topology graph retains PR 122's semantic cards and passes D3 v7 behavior parity, or the D3 migration is explicitly left out rather than partially merged.
- No API contract, authorization rule, recovery behavior, or MySQL topology is changed by the UI consolidation.
- Automated suites and browser QA pass before PR 122 is marked ready.
- PRs 125 and 126 receive an evidence-based disposition without being closed automatically.

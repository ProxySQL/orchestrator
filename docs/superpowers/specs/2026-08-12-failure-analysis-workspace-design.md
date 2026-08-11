# Failure Analysis Workspace Design

## Goal

Replace the legacy floating-popover presentation on `/web/clusters-analysis` with a clear operational incident workspace that matches the restored Clusters and topology pages. Preserve the existing analysis APIs, recovery semantics, navigation destinations, authorization rules, and refresh behavior.

## Scope

This change covers only the Failure analysis page:

- page shell and responsive layout;
- semantic rendering of clusters and analysis entries;
- severity, downtime, and blocked-recovery presentation;
- topology navigation;
- loading, empty, and unavailable states;
- focused automated and live-lab verification.

It does not change failure-detection logic, recovery decisions, API response shapes, polling intervals, or other audit pages.

## Visual Direction

Use the same restrained operational style as the restored Clusters page: charcoal page header, warm neutral background, white bordered rows, compact typography, and blue primary actions. Severity color is reserved for state indicators rather than large decorative surfaces.

The page consists of:

1. A compact header containing the eyebrow “Recovery operations,” the title “Failure analysis,” a live incident-count summary, and a link back to the cluster dashboard.
2. A narrow summary strip identifying the row columns: cluster, active analysis, and impact/action.
3. One stacked row per affected cluster. Each row contains cluster identity, instance count, analysis entries, impact counts, and an “Open topology” action.
4. A calm empty-state panel when no actionable analysis exists.

Rows remain single-column cards at narrow widths. No page-level horizontal scrolling is permitted.

## Components and Responsibilities

### Template shell

`clusters_analysis.tmpl` owns the stable semantic structure: the workspace section, labelled header, live summary, list container, loading status, and scoped stylesheet/script references. JavaScript fills only the dynamic incident list and status text.

### Analysis state preparation

`clusters-analysis.js` keeps the existing API sequence and association logic, but separates data preparation from DOM rendering. It will derive a display model for each affected cluster containing:

- canonical topology URL;
- display alias and cluster name;
- total instance count;
- sorted analysis entries;
- blocked and downtime flags;
- affected or participating replica counts;
- a page-level incident total.

Default aliases equal to the cluster name are not displayed twice and use the canonical cluster-name route.

### Incident rendering

Each cluster becomes one semantic article with a stable cluster-name data hook. Analysis entries are rendered as a list rather than nested `<hr>` fragments. Every entry exposes:

- the analysis code as the primary label;
- the analyzed instance;
- a readable “Downtimed” or “Recovery blocked” state when applicable;
- the relevant replica-impact count.

Existing links to prior blocking recoveries remain available in the global alert area.

### Scoped styles

A dedicated `clusters-analysis-workspace.css` styles only `#clusters_analysis_workspace`. It must not alter legacy popovers used elsewhere. Layout and state selectors use semantic workspace classes and `data-analysis-state` attributes, not Bootstrap popover internals.

## Data Flow and States

On document ready, the page shows its loading state and requests clusters, replication analysis, and blocked recoveries using the existing endpoints. Once all required data is available, JavaScript prepares the display model, updates the live incident count, and renders either affected-cluster rows or the empty state. Authorized users retain the existing refresh timer.

If an API request fails, the loader is removed and the workspace displays a concise unavailable-state message with a reload action. Partial data must not be presented as a healthy empty state.

The empty state says that no incidents currently require failover attention and briefly explains that the page reports actionable failure analysis. It does not print the entire internal `interestingAnalysis` list.

## Accessibility and Interaction

- The workspace title labels the main section.
- The incident count uses `role="status"` and polite live updates.
- Cluster rows are semantic articles with descriptive headings.
- State is conveyed by text and iconography in addition to color.
- Links have visible focus treatment and descriptive labels.
- Loading, empty, and unavailable states are readable without JavaScript-created popovers.

## Testing

Implementation follows red-green TDD. Tests cover:

- rendered template shell and stylesheet contract;
- canonical topology routes for default and distinct aliases;
- incident-model derivation for normal, blocked, downtimed, and structural analysis;
- empty and unavailable rendering states;
- absence of legacy popover markup in the page renderer;
- JavaScript syntax and the complete Go HTTP test suite;
- the live Docker smoke suite and direct HTTP checks for the page and assets;
- browser verification of populated and empty layouts, topology navigation, focusable actions, console errors, and a narrow viewport.

## Acceptance Criteria

- Failure analysis visually belongs to the same product as the restored Clusters and topology pages.
- A user can identify the affected cluster, failing instance, analysis type, impact, and blocked/downtimed state at a glance.
- Every displayed topology action reaches a valid route.
- The page has intentional loading, empty, and error states.
- Existing backend analysis and recovery behavior is unchanged.
- The layout is usable without body-level horizontal overflow on narrow screens.

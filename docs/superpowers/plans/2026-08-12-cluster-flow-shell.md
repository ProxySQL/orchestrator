# Cluster Flow Shell Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `/web/clusters` a useful operational landing page and make its transition into cluster topology visually coherent.

**Architecture:** Retain current routes, APIs, and JavaScript data flow. Add a page-scoped clusters landing stylesheet and template landmarks, then reuse the compact shell vocabulary on cluster detail by moving existing display controls into a labelled inline View menu.

**Tech Stack:** Go templates/tests, existing jQuery/D3 v3, CSS, functional Docker lab.

## Global Constraints

- Preserve routes, APIs, recovery/failover, drag/drop, D3 positioning, and node modal behavior.
- Preserve existing cluster command values while changing their presentation.
- Keep page-specific CSS scoped; do not extend global Bootstrap compatibility shims.
- The `/` redirect remains `/web/clusters`; the landing page must stand on its own.

---

## File Map

| File | Responsibility |
| --- | --- |
| `resources/templates/clusters.tmpl` | Landing-page shell and operational list landmarks. |
| `resources/public/css/clusters-workspace.css` | Scoped landing-page layout and responsive styles. |
| `resources/templates/cluster.tmpl` | Compact identity row and inline View menu. |
| `resources/public/css/cluster-workspace.css` | Detail-shell refinement without topology changes. |
| `resources/public/js/cluster.js` | Existing command delegation adapted to the inline menu only if required. |
| `go/http/render_test.go` | Template contracts for landing and retained detail controls. |
| `tests/functional/test-smoke.sh` | Landing, redirect, and detail-page HTTP smoke checks. |

## Task 1: Protect landing and detail page contracts

**Files:** `go/http/render_test.go`

- [ ] Add a failing rendered `templates/clusters` test asserting `id="clusters_workspace"`, `id="clusters_list"`, and `/css/clusters-workspace.css`.
- [ ] Add a detail-template assertion for an inline `View` control while retaining all `data-command` values.
- [ ] Run `go test ./go/http -run 'TestRenderClustersWorkspace|TestRenderClusterWorkspace'` and confirm it is red before markup.
- [ ] Implement no production code in this task; commit the contract.

```bash
git add go/http/render_test.go
git commit -m "test(ui): define cluster flow shell contracts"
```

## Task 2: Build the clusters operational landing page

**Files:** `resources/templates/clusters.tmpl`, `resources/public/css/clusters-workspace.css`

- [ ] Load the new stylesheet and wrap existing cluster results in `#clusters_workspace` and `#clusters_list` without replacing existing JavaScript IDs/classes.
- [ ] Add a compact landing header with title, known-cluster count region, and a labelled discovery action.
- [ ] Style cluster rows/cards for name, primary, members, health/problems, and explicit open action; use the data already rendered by `clusters.js`.
- [ ] Keep all CSS below `#clusters_workspace` and add a narrow-screen single-column fallback.
- [ ] Run `go test ./go/http -run TestRenderClustersWorkspace` and `node --check resources/public/js/clusters.js`.
- [ ] Commit the landing page.

```bash
git add resources/templates/clusters.tmpl resources/public/css/clusters-workspace.css go/http/render_test.go
git commit -m "feat(ui): add operational clusters landing page"
```

## Task 3: Flatten cluster-detail chrome

**Files:** `resources/templates/cluster.tmpl`, `resources/public/css/cluster-workspace.css`, `resources/public/js/cluster.js`

- [ ] Replace the visually dominant rail treatment with a compact inline View menu beside the existing identity/status row.
- [ ] Move no `data-command` elements semantically: preserve their values, delegated handlers, keyboard behavior, and default-navigation guard.
- [ ] Keep topology immediately below the compact header; do not alter D3 sizing, graph geometry, cards, or modal behavior.
- [ ] Add CSS for the menu’s open/focus state and narrow-screen wrapping, all scoped under `#cluster_workspace`.
- [ ] Run `go test ./go/http`, `node --check resources/public/js/cluster.js`, and `git diff --check`.
- [ ] Commit the detail-shell refinement.

```bash
git add resources/templates/cluster.tmpl resources/public/css/cluster-workspace.css resources/public/js/cluster.js go/http/render_test.go
git commit -m "feat(ui): simplify cluster topology chrome"
```

## Task 4: Verify the redirected user flow

**Files:** `tests/functional/test-smoke.sh`

- [ ] Extend smoke coverage for `/` redirecting to `/web/clusters`, the landing shell, and `/web/cluster/mysql1:3306` retaining its topology shell.
- [ ] Rebuild/recreate only the Orchestrator lab container from this worktree, preserving the MySQL topology.
- [ ] Run `bash tests/functional/test-smoke.sh` and assert `curl -fsSI http://localhost:3099/` returns the cluster landing redirect.
- [ ] Manually inspect landing → cluster detail, menu controls, node Details, and narrow viewport in the user browser.
- [ ] Commit smoke coverage.

```bash
git add tests/functional/test-smoke.sh
git commit -m "test(ui): verify cluster landing flow"
```

## Task 5: Final verification

- [ ] Run `go test ./go/http`, `git diff --check`, and the functional smoke script.
- [ ] Confirm `git status --short` contains no tracked scratch files.
- [ ] Review the complete landing-to-topology route in the Docker lab and summarize preserved operational behavior.


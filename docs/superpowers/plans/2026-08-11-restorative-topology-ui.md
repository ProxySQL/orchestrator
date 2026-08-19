# Restorative Topology UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restore the cluster-detail page to a legible, calm topology workspace that evokes the historical Orchestrator UI while retaining the current API and topology behavior.

**Architecture:** Keep the D3 v3 topology renderer and its absolute-positioned instance cards intact for the first release. Add a cluster-scoped workspace shell and semantic card hooks around the existing renderer, with all presentation rules isolated in a new stylesheet. The global Bootstrap 5 layout remains unchanged.

**Tech Stack:** Go HTML templates, Go `httptest` template tests, jQuery, D3 v3, existing functional Docker lab, CSS.

## Global Constraints

- Do not change recovery, failover, drag-and-drop, API, or discovery semantics.
- Scope new visual rules below `#cluster_workspace`; do not extend the global Bootstrap compatibility layer.
- Preserve current `data-command`, instance modal, and D3 card positioning hooks.
- Make state understandable with text and icons as well as color.
- Keep the first release desktop-first, with a narrow-screen fallback that remains usable.
- Do not add or commit `.superpowers/` visual-companion artifacts.

---

## File Map

| File | Responsibility |
| --- | --- |
| `resources/templates/cluster.tmpl` | Add the cluster workspace shell, toolbar, canvas, and semantic navigation landmarks. |
| `resources/public/css/cluster-workspace.css` | Contain the restorative topology layout, card states, menu, and responsive rules. |
| `resources/public/js/orchestrator.js` | Render stable semantic card regions and a compact action trigger. |
| `resources/public/js/cluster.js` | Delegate the new action trigger without disturbing existing node controls. |
| `resources/public/js/cluster-tree.js` | Keep the D3 canvas sized to the workspace viewport. |
| `go/http/render_test.go` | Verify cluster template markup and stylesheet inclusion. |
| `go/http/static_assets_test.go` | Verify the new static asset and card interaction hooks are shipped. |
| `tests/functional/test-smoke.sh` | Assert the live lab serves the cluster workspace shell. |

## Task 1: Establish a rendering contract

**Files:** `go/http/render_test.go`, `go/http/static_assets_test.go`

- [ ] Add `TestRenderClusterWorkspace` using `templates/cluster` and the existing sample template data.
- [ ] Assert the response has `id="cluster_workspace"`, `id="cluster_canvas"`, and `/css/cluster-workspace.css`.
- [ ] Add static source assertions for the new CSS file and the semantic card/action hook names.
- [ ] Run the focused Go tests:

```bash
go test ./go/http -run 'TestRenderClusterWorkspace|TestStatic'
```

Expected: the focused test suite passes before markup behavior is changed.

- [ ] Commit the contract test first.

```bash
git add go/http/render_test.go go/http/static_assets_test.go
git commit -m "test(ui): define cluster workspace rendering contract"
```

## Task 2: Add the scoped cluster workspace shell

**Files:** `resources/templates/cluster.tmpl`, `resources/public/css/cluster-workspace.css`

- [ ] Include the stylesheet through the cluster page’s template block.
- [ ] Wrap the existing sidebar and topology area in `#cluster_workspace` without renaming legacy IDs.
- [ ] Add landmarks for a compact cluster header, command rail, topology canvas, and an accessible live status region.
- [ ] Move no controls: existing `data-command` links must remain present and operational.
- [ ] Implement the base palette: a restrained dark application chrome, light canvas, and a narrow low-noise rail.
- [ ] Run `go test ./go/http -run TestRenderClusterWorkspace` and inspect the rendered page source with curl.
- [ ] Commit the shell and CSS foundation.

```bash
git add resources/templates/cluster.tmpl resources/public/css/cluster-workspace.css
git commit -m "feat(ui): add restorative cluster workspace shell"
```

## Task 3: Render semantic node cards

**Files:** `resources/public/js/orchestrator.js`, `resources/public/css/cluster-workspace.css`, `resources/public/js/cluster.js`

- [ ] Refactor `renderInstanceElement` to emit named card regions for identity, role, health, replication, and actions.
- [ ] Preserve each existing status calculation, warning text, and the instance element’s current dimensions/positioning contract.
- [ ] Replace icon-only card affordances with a concise, labelled details/action trigger while retaining the modal entry point.
- [ ] Give warning and fatal states explicit label treatment in addition to their color treatment.
- [ ] Update delegated click handling so the new trigger opens the same node modal and ordinary card dragging is not intercepted.
- [ ] Add CSS for quiet normal cards, a clear primary badge, replica state, warning/fatal emphasis, and a visible selected state.
- [ ] Run the focused Go static-asset tests and `git diff --check`.
- [ ] Commit semantic card rendering.

```bash
git add resources/public/js/orchestrator.js resources/public/js/cluster.js resources/public/css/cluster-workspace.css go/http/static_assets_test.go
git commit -m "feat(ui): restore semantic topology node cards"
```

## Task 4: Fit the topology renderer into its workspace

**Files:** `resources/public/js/cluster-tree.js`, `resources/public/css/cluster-workspace.css`

- [ ] Measure the D3 viewport from `#cluster_canvas` while retaining a safe fallback to `#cluster_container`.
- [ ] Keep the existing tree geometry, line drawing, pan/zoom behavior, and `repositionIntanceDiv` integration.
- [ ] Add responsive canvas and card rules for a narrow browser window; retain horizontal exploration rather than crushing nodes.
- [ ] Check syntax by loading the page in the running lab and checking the browser console manually.
- [ ] Commit the renderer integration.

```bash
git add resources/public/js/cluster-tree.js resources/public/css/cluster-workspace.css
git commit -m "feat(ui): fit topology graph to workspace canvas"
```

## Task 5: Verify with the three-node Docker lab

**Files:** `tests/functional/test-smoke.sh`

- [ ] Extend the existing smoke script with a request to `/web/cluster/mysql1:3306`.
- [ ] Assert the returned HTML includes the workspace shell and its stylesheet; retain the existing API checks.
- [ ] Run the local functional lab and then:

```bash
bash tests/functional/test-smoke.sh
curl -fsS http://localhost:3099/web/cluster/mysql1:3306 | rg 'cluster_workspace|cluster-workspace.css'
```

Expected: smoke tests pass and both workspace identifiers are present.

- [ ] Perform manual visual checks in the user’s open browser: 3-node tree, primary/replica distinction, warning state, modal details, and narrow viewport fallback.
- [ ] Commit the smoke coverage.

```bash
git add tests/functional/test-smoke.sh
git commit -m "test(ui): smoke-test cluster workspace"
```

## Task 6: Final verification and handoff

- [ ] Run `go test ./go/http`.
- [ ] Run `git diff --check` and inspect `git status --short` to confirm only intended source files are tracked.
- [ ] Run the functional smoke script against the Docker lab.
- [ ] Review the rendered cluster page in the local browser at desktop and narrow widths.
- [ ] Summarize preserved behavior, visual changes, and any intentionally deferred renderer modernization.

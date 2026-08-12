# Failure Analysis Workspace Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `/web/clusters-analysis` legacy popovers with a responsive, semantic incident workspace consistent with the restored cluster dashboard.

**Architecture:** Keep the existing three API endpoints and refresh policy. Split the browser code into pure data-model and markup functions, with a thin document-ready adapter that loads data, renders one of loading/content/empty/error states, and preserves blocked-recovery alerts. A dedicated stylesheet is scoped under `#clusters_analysis_workspace` so the rest of Orchestrator remains untouched.

**Tech Stack:** Go HTML templates and `httptest`, browser JavaScript with jQuery, Node's built-in test runner and `vm`, scoped CSS, Docker Compose live lab.

## Global Constraints

- Do not change failure-detection logic, recovery decisions, API response shapes, polling intervals, or other audit pages.
- Preserve `/api/clusters-info`, `/api/replication-analysis`, and `/api/blocked-recoveries` as the data sources.
- Preserve authorized-user refresh behavior and blocked-recovery audit links.
- Default aliases equal to the cluster name must not be displayed twice and must use the canonical cluster-name topology route.
- No page-level horizontal scrolling is permitted at narrow widths.
- State must be conveyed by text in addition to color.
- Dynamic API strings must be HTML-escaped before insertion into markup.

---

### Task 1: Semantic Failure Analysis Shell

**Files:**
- Modify: `go/http/render_test.go`
- Modify: `resources/templates/clusters_analysis.tmpl`
- Create: `resources/public/css/clusters-analysis-workspace.css`

**Interfaces:**
- Produces: `#clusters_analysis_workspace`, `#clusters_analysis_summary`, `#clusters_analysis_list`, `#clusters_analysis_loading`, and `.clusters-analysis-dashboard-link` for later renderer and browser checks.
- Consumes: existing template fields `.prefix` and `.removeTextFromHostnameDisplay`.

- [ ] **Step 1: Write the failing shell test**

Add this focused test to `go/http/render_test.go`:

```go
func TestRenderClustersAnalysisWorkspace(t *testing.T) {
	chdirToRepoRoot(t)
	clearContentTemplateCache()

	rec := httptest.NewRecorder()
	renderHTML(rec, http.StatusOK, "templates/clusters_analysis", sampleTemplateData())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, expected := range []string{
		`id="clusters_analysis_workspace"`,
		`aria-labelledby="clusters_analysis_title"`,
		`id="clusters_analysis_summary"`,
		`role="status"`,
		`id="clusters_analysis_loading"`,
		`id="clusters_analysis_list"`,
		`href="/web/clusters"`,
		`/css/clusters-analysis-workspace.css`,
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("expected failure analysis workspace contract %q", expected)
		}
	}
}
```

- [ ] **Step 2: Run the shell test and verify RED**

Run: `go test ./go/http -run TestRenderClustersAnalysisWorkspace -count=1`

Expected: FAIL because `clusters_analysis.tmpl` has only the legacy `#clusters_analysis` container.

- [ ] **Step 3: Implement the semantic template shell**

Replace the legacy container in `resources/templates/clusters_analysis.tmpl` with this structure, retaining the hostname helper and script dependencies below it:

```html
<link href="{{.prefix}}/css/clusters-analysis-workspace.css?v=20260812-ui" rel="stylesheet">

<section id="clusters_analysis_workspace" aria-labelledby="clusters_analysis_title">
  <header class="clusters-analysis-header">
    <div>
      <p class="clusters-analysis-eyebrow">Recovery operations</p>
      <h1 id="clusters_analysis_title">Failure analysis</h1>
      <p id="clusters_analysis_summary" role="status" aria-live="polite" aria-atomic="true">Loading active incidents&hellip;</p>
    </div>
    <a class="clusters-analysis-dashboard-link" href="{{.prefix}}/web/clusters">Cluster dashboard</a>
  </header>
  <main class="clusters-analysis-content" aria-label="Active failure analysis">
    <div class="clusters-analysis-columns" aria-hidden="true">
      <span>Cluster</span><span>Active analysis</span><span>Impact / action</span>
    </div>
    <div id="clusters_analysis_loading" class="clusters-analysis-state">Loading active incidents&hellip;</div>
    <div id="clusters_analysis_list"></div>
  </main>
</section>
```

Create `clusters-analysis-workspace.css` with an initial root rule only:

```css
#clusters_analysis_workspace {
  --analysis-accent: #2f6b9e;
  --analysis-background: #f4f3ef;
  --analysis-border: #d5d9dc;
  --analysis-chrome: #202b36;
  --analysis-muted: #687582;
  --analysis-panel: #ffffff;
  --analysis-text: #263442;
}
```

- [ ] **Step 4: Run the shell and complete HTTP tests**

Run: `gofmt -w go/http/render_test.go && go test ./go/http -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the shell**

```bash
git add go/http/render_test.go resources/templates/clusters_analysis.tmpl resources/public/css/clusters-analysis-workspace.css
git commit -m "feat(ui): add failure analysis workspace shell"
```

---

### Task 2: Pure Incident Display Model

**Files:**
- Modify: `go/http/testdata/clusters_analysis_state_test.js`
- Modify: `resources/public/js/clusters-analysis.js`

**Interfaces:**
- Produces: `buildClustersAnalysisModel(clusters, replicationAnalysis, blockedRecoveries, interestingAnalysisMap) -> {clusters: AnalysisCluster[], incidentCount: number}`.
- Produces each `AnalysisCluster` with `{clusterName, displayName, alias, topologyPath, countInstances, allDowntimed, state, entries}`.
- Produces each entry with `{analysis, instance, state, statusLabel, impactLabel, replicaCount, downtimeEndTimestamp}`.
- Consumes: `clusterAnalysisTopologyPath(cluster, compact)` from the same file.

- [ ] **Step 1: Add failing behavior tests for model derivation**

Extend `go/http/testdata/clusters_analysis_state_test.js` with literal fixtures and expectations:

```js
test("incident model derives actionable, blocked, downtimed, and structural entries", function() {
  const clusters = [{ClusterName: "mysql1:3306", ClusterAlias: "mysql1:3306", CountInstances: 3}];
  const replicationAnalysis = {Details: [{
    Analysis: "DeadMaster",
    AnalyzedInstanceKey: {Hostname: "mysql1", Port: 3306},
    ClusterDetails: {ClusterName: "mysql1:3306"},
    CountReplicas: 2,
    IsDowntimed: false,
    StructureAnalysis: ["ErrantGTIDStructureWarning"],
  }]};
  const blocked = [{
    FailedInstanceKey: {Hostname: "mysql1", Port: 3306},
    Analysis: "DeadMaster",
  }];

  const model = sandbox.buildClustersAnalysisModel(
    clusters,
    replicationAnalysis,
    blocked,
    {DeadMaster: true}
  );

  assert.equal(model.incidentCount, 2);
  assert.equal(model.clusters.length, 1);
  assert.equal(model.clusters[0].topologyPath, "/web/cluster/mysql1:3306?compact=true");
  assert.equal(model.clusters[0].state, "blocked");
  assert.deepEqual(JSON.parse(JSON.stringify(model.clusters[0].entries)), [
    {
      analysis: "DeadMaster",
      instance: "mysql1:3306",
      state: "blocked",
      statusLabel: "Recovery blocked",
      impactLabel: "Affected replicas",
      replicaCount: 2,
      downtimeEndTimestamp: "",
    },
    {
      analysis: "ErrantGTIDStructureWarning",
      instance: "mysql1:3306",
      state: "warning",
      statusLabel: "Structural warning",
      impactLabel: "Participating replicas",
      replicaCount: 2,
      downtimeEndTimestamp: "",
    },
  ]);
});

test("incident model reports a downtimed analysis without mutating API input", function() {
  const entry = {
    Analysis: "DeadMaster",
    AnalyzedInstanceKey: {Hostname: "mysql1", Port: 3306},
    ClusterDetails: {ClusterName: "mysql1:3306"},
    CountReplicas: 2,
    IsDowntimed: true,
    DowntimeEndTimestamp: "2026-08-12 04:00:00",
    StructureAnalysis: [],
  };
  const model = sandbox.buildClustersAnalysisModel(
    [{ClusterName: "mysql1:3306", ClusterAlias: "production", CountInstances: 3}],
    {Details: [entry]},
    [],
    {DeadMaster: true}
  );

  assert.equal(model.clusters[0].alias, "production");
  assert.equal(model.clusters[0].state, "downtimed");
  assert.equal(model.clusters[0].entries[0].statusLabel, "Downtimed");
  assert.equal(model.clusters[0].entries[0].downtimeEndTimestamp, "2026-08-12 04:00:00");
  assert.equal(entry.IsStructureAnalysis, undefined);
  assert.equal(entry.Analysis, "DeadMaster");
});
```

- [ ] **Step 2: Run the model tests and verify RED**

Run: `node --test go/http/testdata/clusters_analysis_state_test.js`

Expected: FAIL with `buildClustersAnalysisModel is not a function`.

- [ ] **Step 3: Implement the pure model builder**

Add top-level helpers before `$(document).ready(...)` in `clusters-analysis.js`:

```js
function clustersAnalysisBlockedKey(hostname, port, analysis) {
  return hostname + ":" + port + ":" + analysis;
}

function buildClustersAnalysisModel(clusters, replicationAnalysis, blockedRecoveries, interestingAnalysisMap) {
  var blocked = {};
  (blockedRecoveries || []).forEach(function(recovery) {
    var key = clustersAnalysisBlockedKey(
      recovery.FailedInstanceKey.Hostname,
      recovery.FailedInstanceKey.Port,
      recovery.Analysis
    );
    blocked[key] = true;
  });

  var byName = {};
  (clusters || []).forEach(function(cluster) {
    byName[cluster.ClusterName] = {
      clusterName: cluster.ClusterName,
      displayName: cluster.ClusterName,
      alias: cluster.ClusterAlias && cluster.ClusterAlias != cluster.ClusterName ? cluster.ClusterAlias : "",
      topologyPath: clusterAnalysisTopologyPath(cluster, true),
      countInstances: cluster.CountInstances,
      allDowntimed: true,
      state: "downtimed",
      entries: [],
    };
  });

  function appendEntry(apiEntry, analysis, structural) {
    var cluster = byName[apiEntry.ClusterDetails.ClusterName];
    if (!cluster) {
      return;
    }
    var isBlocked = !!blocked[clustersAnalysisBlockedKey(
      apiEntry.AnalyzedInstanceKey.Hostname,
      apiEntry.AnalyzedInstanceKey.Port,
      analysis
    )];
    var state = structural ? "warning" : (isBlocked ? "blocked" : (apiEntry.IsDowntimed ? "downtimed" : "actionable"));
    var labels = {
      actionable: "Requires attention",
      blocked: "Recovery blocked",
      downtimed: "Downtimed",
      warning: "Structural warning",
    };
    cluster.entries.push({
      analysis: analysis,
      instance: apiEntry.AnalyzedInstanceKey.Hostname + ":" + apiEntry.AnalyzedInstanceKey.Port,
      state: state,
      statusLabel: labels[state],
      impactLabel: structural ? "Participating replicas" : "Affected replicas",
      replicaCount: apiEntry.CountReplicas,
      downtimeEndTimestamp: apiEntry.IsDowntimed ? (apiEntry.DowntimeEndTimestamp || "") : "",
    });
    if (!apiEntry.IsDowntimed) {
      cluster.allDowntimed = false;
    }
  }

  ((replicationAnalysis && replicationAnalysis.Details) || []).forEach(function(entry) {
    if (Object.prototype.hasOwnProperty.call(interestingAnalysisMap, entry.Analysis)) {
      appendEntry(entry, entry.Analysis, false);
    }
    (entry.StructureAnalysis || []).forEach(function(analysis) {
      appendEntry(entry, analysis, true);
    });
  });

  var precedence = {blocked: 4, actionable: 3, warning: 2, downtimed: 1};
  var affected = Object.keys(byName).map(function(name) {
    var cluster = byName[name];
    cluster.entries.forEach(function(entry) {
      if (precedence[entry.state] > precedence[cluster.state]) {
        cluster.state = entry.state;
      }
    });
    return cluster;
  }).filter(function(cluster) {
    return cluster.entries.length > 0;
  });

  affected.sort(function(a, b) {
    if (a.allDowntimed != b.allDowntimed) {
      return a.allDowntimed ? 1 : -1;
    }
    return (b.countInstances - a.countInstances) || a.clusterName.localeCompare(b.clusterName);
  });

  return {
    clusters: affected,
    incidentCount: affected.reduce(function(total, cluster) { return total + cluster.entries.length; }, 0),
  };
}
```

After model construction, apply `removeTextFromHostnameDisplay()` to a copied `displayName` only; do not alter `clusterName` or URLs.

- [ ] **Step 4: Run model tests and all JavaScript behavior tests**

Run: `node --test go/http/testdata/*.js`

Expected: PASS with the new model tests and existing route/policy tests.

- [ ] **Step 5: Commit the incident model**

```bash
git add go/http/testdata/clusters_analysis_state_test.js resources/public/js/clusters-analysis.js
git commit -m "feat(ui): derive failure analysis incident model"
```

---

### Task 3: Semantic Incident, Empty, and Error Rendering

**Files:**
- Modify: `go/http/testdata/clusters_analysis_state_test.js`
- Modify: `resources/public/js/clusters-analysis.js`
- Modify: `resources/templates/clusters_analysis.tmpl`

**Interfaces:**
- Consumes: the `AnalysisCluster` model from Task 2.
- Produces: `renderClustersAnalysisMarkup(model) -> string`.
- Produces: `renderClustersAnalysisEmptyState() -> string` and `renderClustersAnalysisUnavailableState() -> string`.
- Produces: `escapeClustersAnalysisHTML(value) -> string` for all API-derived text and attributes.
- The document adapter writes markup only to `#clusters_analysis_list`, hides `#clusters_analysis_loading`, and updates `#clusters_analysis_summary`.

- [ ] **Step 1: Add failing renderer tests**

Add these tests to `clusters_analysis_state_test.js`:

```js
test("incident markup is semantic, escaped, and contains one clear topology action", function() {
  const html = sandbox.renderClustersAnalysisMarkup({incidentCount: 1, clusters: [{
    clusterName: "mysql1:3306",
    displayName: "<mysql1>",
    alias: "",
    topologyPath: "/web/cluster/mysql1:3306?compact=true",
    countInstances: 3,
    allDowntimed: false,
    state: "actionable",
    entries: [{
      analysis: "DeadMaster",
      instance: "mysql1:3306",
      state: "actionable",
      statusLabel: "Requires attention",
      impactLabel: "Affected replicas",
      replicaCount: 2,
      downtimeEndTimestamp: "",
    }],
  }]});

  assert.match(html, /<article[^>]+data-cluster-name="mysql1:3306"/);
  assert.match(html, /&lt;mysql1&gt;/);
  assert.doesNotMatch(html, /<mysql1>/);
  assert.match(html, /DeadMaster/);
  assert.match(html, /Affected replicas/);
  assert.match(html, /href="\/web\/cluster\/mysql1:3306\?compact=true"/);
  assert.equal((html.match(/Open topology/g) || []).length, 1);
  assert.doesNotMatch(html, /popover|popover-title|popover-content/);
});

test("empty and unavailable states cannot be confused", function() {
  const empty = sandbox.renderClustersAnalysisEmptyState();
  const unavailable = sandbox.renderClustersAnalysisUnavailableState();

  assert.match(empty, /No active failover incidents/);
  assert.doesNotMatch(empty, /DeadMaster|interestingAnalysis/);
  assert.match(unavailable, /Failure analysis is temporarily unavailable/);
  assert.match(unavailable, /Reload page/);
});
```

- [ ] **Step 2: Run renderer tests and verify RED**

Run: `node --test go/http/testdata/clusters_analysis_state_test.js`

Expected: FAIL because the three render functions do not exist.

- [ ] **Step 3: Implement escaped semantic markup**

Implement `escapeClustersAnalysisHTML` using replacements for `&`, `<`, `>`, `"`, and `'`. Implement the three pure render functions with these class hooks:

```html
<article class="analysis-cluster" data-analysis-state="actionable" data-cluster-name="mysql1:3306">
  <header class="analysis-cluster-identity">...</header>
  <ul class="analysis-entry-list">
    <li class="analysis-entry" data-analysis-state="actionable">...</li>
  </ul>
  <aside class="analysis-cluster-impact">...</aside>
</article>
```

Every API-derived value and URL must pass through `escapeClustersAnalysisHTML`. Each state must include visible copy: `Requires attention`, `Recovery blocked`, `Downtimed`, or `Structural warning`.

- [ ] **Step 4: Replace the legacy document adapter**

In the document-ready block:

- call `buildClustersAnalysisModel(...)` once all three required API calls succeed;
- hide `#clusters_analysis_loading`;
- update summary to `N active incident(s) across M cluster(s)`;
- insert `renderClustersAnalysisMarkup(model)` or `renderClustersAnalysisEmptyState()`;
- on any required request failure, hide the loader, set summary to `Analysis unavailable`, and insert `renderClustersAnalysisUnavailableState()`;
- keep the separate blocked-recovery alerts and authorized refresh timer;
- remove all `popover`, `popover-title`, `popover-content`, `.popover()`, and `.show()` rendering code.

Use the existing jQuery Deferred failure callbacks; do not change endpoints or polling.

- [ ] **Step 5: Run behavior, syntax, and HTTP tests**

Run:

```bash
node --test go/http/testdata/*.js
node --check resources/public/js/clusters-analysis.js
go test ./go/http -count=1
```

Expected: all commands PASS.

- [ ] **Step 6: Commit the semantic renderer**

```bash
git add go/http/testdata/clusters_analysis_state_test.js resources/public/js/clusters-analysis.js resources/templates/clusters_analysis.tmpl
git commit -m "feat(ui): render semantic failure incidents"
```

---

### Task 4: Restorative Responsive Styling

**Files:**
- Modify: `resources/public/css/clusters-analysis-workspace.css`
- Modify: `go/http/static_assets_test.go`

**Interfaces:**
- Consumes: Task 1 shell hooks and Task 3 semantic incident hooks.
- Produces: desktop three-column incident rows and narrow single-column cards without body overflow.

- [ ] **Step 1: Add the failing CSS scope contract**

Add a test to `go/http/static_assets_test.go` that reads `clusters-analysis-workspace.css`, requires selectors for all semantic hooks, and rejects unscoped rule starts:

```go
func TestClustersAnalysisWorkspaceStylesAreScoped(t *testing.T) {
	chdirToRepoRoot(t)
	source, err := os.ReadFile(filepath.Join("resources", "public", "css", "clusters-analysis-workspace.css"))
	if err != nil {
		t.Fatal(err)
	}
	css := string(source)
	for _, selector := range []string{
		"#clusters_analysis_workspace .clusters-analysis-header",
		"#clusters_analysis_workspace .analysis-cluster",
		"#clusters_analysis_workspace .analysis-entry",
		"#clusters_analysis_workspace .analysis-cluster-impact",
		"#clusters_analysis_workspace .clusters-analysis-empty",
		"#clusters_analysis_workspace .clusters-analysis-unavailable",
	} {
		if !strings.Contains(css, selector) {
			t.Errorf("missing scoped selector %q", selector)
		}
	}
	if strings.Contains(css, "\n.popover") || strings.Contains(css, "\n.container") {
		t.Fatal("failure analysis stylesheet leaks legacy global selectors")
	}
}
```

- [ ] **Step 2: Run the CSS contract and verify RED**

Run: `go test ./go/http -run TestClustersAnalysisWorkspaceStylesAreScoped -count=1`

Expected: FAIL on missing semantic selectors.

- [ ] **Step 3: Implement the complete scoped stylesheet**

Build on the Task 1 variables and implement:

- full-bleed warm workspace with `min-height: calc(100vh - 56px)`;
- 88px charcoal flex header matching `clusters-workspace.css`;
- centered content at `max-width: 1180px`;
- desktop row grid `minmax(220px, .9fr) minmax(360px, 1.6fr) minmax(180px, .7fr)`;
- white 5px-radius rows with 1px borders and a 4px state strip;
- compact analysis-entry list with readable status pills and impact numbers;
- blue `Open topology` action and visible `:focus` outlines;
- muted, centered empty/unavailable panels;
- state colors: actionable `#b94a48`, blocked `#8e3b46`, warning `#b17817`, downtimed `#687582`;
- `@media (max-width: 760px)` hiding column labels and making rows one column;
- `overflow-wrap: anywhere` on identity and instance text;
- no fixed content widths or negative horizontal offsets inside rows.

- [ ] **Step 4: Run CSS, HTTP, and diff checks**

Run:

```bash
go test ./go/http -count=1
git diff --check
```

Expected: PASS.

- [ ] **Step 5: Commit responsive styling**

```bash
git add resources/public/css/clusters-analysis-workspace.css go/http/static_assets_test.go
git commit -m "feat(ui): style failure analysis workspace"
```

---

### Task 5: Live Lab and Browser Verification

**Files:**
- Modify: `tests/functional/test-smoke.sh`
- Modify if browser evidence identifies a defect: only files introduced or modified in Tasks 1-4

**Interfaces:**
- Consumes: completed workspace route and static assets.
- Produces: repeatable smoke coverage and verified desktop/narrow behavior in the three-MySQL lab.

- [ ] **Step 1: Add failing live smoke coverage**

Under the Web UI section of `tests/functional/test-smoke.sh`, add:

```bash
test_endpoint "Failure analysis workspace" "$ORC_URL/web/clusters-analysis" "200"
test_body_contains "Failure analysis shell" "$ORC_URL/web/clusters-analysis" 'id="clusters_analysis_workspace"'
test_body_contains "Failure analysis stylesheet" "$ORC_URL/web/clusters-analysis" 'clusters-analysis-workspace\.css'
```

- [ ] **Step 2: Run smoke against the pre-restart container and verify RED**

Run: `bash tests/functional/test-smoke.sh`

Expected: at least the new shell or stylesheet assertion fails if the running container still serves the previous worktree state.

- [ ] **Step 3: Rebuild and restart only Orchestrator**

Build the current branch's binary, then recreate the service:

```bash
go build -o bin/orchestrator ./go/cmd/orchestrator
docker compose -f tests/functional/docker-compose.yml up -d --force-recreate orchestrator
```

Do not recreate mysql1, mysql2, or mysql3.

- [ ] **Step 4: Run complete automated verification**

Run:

```bash
gofmt -w go/http/render_test.go go/http/static_assets_test.go
node --check resources/public/js/clusters-analysis.js
node --test go/http/testdata/*.js
go test ./go/http -count=1
bash tests/functional/test-smoke.sh
git diff --check
```

Expected: all commands PASS, with the smoke total increased by three.

- [ ] **Step 5: Verify all rendered top-navigation destinations**

Fetch `/web/clusters`, extract every unique internal `/web/` href, request each destination, and confirm each returns HTTP 200. Specifically verify `/web/clusters-analysis` and every topology URL rendered by the Failure analysis model.

- [ ] **Step 6: Perform browser QA on the live lab**

At `http://localhost:3099/web/clusters-analysis` verify:

- header, summary, incident rows or empty state render with no legacy popover styling;
- cluster identity, analysis, affected/participating replicas, and state copy are legible;
- Cluster dashboard and Open topology links navigate successfully;
- no browser console errors or warnings originate from the page;
- keyboard focus is visible on both primary links;
- at a viewport no wider than 480px, rows stack, controls remain visible, and `document.documentElement.scrollWidth === document.documentElement.clientWidth`;
- temporarily render `renderClustersAnalysisUnavailableState()` into `#clusters_analysis_list` through the claimed local tab, verify its copy and layout, then reload the page to restore live state;
- temporarily render `renderClustersAnalysisEmptyState()` the same way, verify its copy and layout, then reload again.

If browser QA finds a defect, add a focused failing automated test before changing production code, implement the minimal correction, and rerun Steps 4-6.

- [ ] **Step 7: Commit verification coverage and any tested corrections**

```bash
git add tests/functional/test-smoke.sh
git commit -m "test(ui): verify failure analysis workspace"
```

- [ ] **Step 8: Confirm a clean handoff**

Run: `git status --short && git log -5 --oneline`

Expected: clean tracked worktree and the five implementation commits visible.

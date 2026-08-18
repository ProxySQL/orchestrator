# Consolidated UI Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Consolidate the sound parts of UI PRs 125 and 126 into PR 122 so Orchestrator has one coherent, locally served Bootstrap 5 interface with stable legacy interactions, responsive navigation, current icons, and a behavior-compatible D3 v7 topology graph.

**Architecture:** PR 122 remains the only implementation branch. Shared asset versioning is injected before content and layout rendering, Bootstrap compatibility lives in one idempotent bridge, and page behavior stays in existing scripts. PR 125 changes are reapplied in focused commits; PR 126's Bootstrap 3 rollback is not merged.

**Tech Stack:** Go 1.25.7, `html/template`, Chi HTTP routing, jQuery, Bootstrap 5.3, Bootstrap Icons, D3 v7, Node's built-in test runner, Docker Compose, and the in-app Browser.

**Spec:** `docs/superpowers/specs/2026-08-18-consolidated-ui-integration-design.md`

## Global Constraints

- Work only on `codex/ui-restorative-topology-worktree`, the head branch of PR 122.
- Do not merge or cherry-pick PR 125 or PR 126 wholesale.
- Keep PR 122's semantic workspaces, API contracts, authorization, polling, recovery behavior, and MySQL topology unchanged.
- Keep `/bootstrap5` as the canonical Bootstrap bundle; do not delete the legacy `/bootstrap` directory in this work.
- Use locally served Bootstrap Icons; do not introduce a CDN dependency.
- A delegated interaction must execute once per user action.
- D3 v3 may be removed only after D3 v7 automated and browser parity checks pass.
- Browser QA must cover desktop, 794-pixel, and 390-by-844 viewports with an explicit reload after every build or asset change.
- Do not run a live failover or intentionally change MySQL roles during this plan.
- Do not close PR 125 or PR 126 automatically.

## File Structure

| Responsibility | Files |
|---|---|
| Shared asset version and cache policy | `go/http/render.go`, `go/http/render_test.go`, `go/app/http.go`, `go/app/http_test.go` |
| Local icon assets | `resources/public/bootstrap-icons/font/bootstrap-icons.min.css`, `resources/public/bootstrap-icons/font/fonts/bootstrap-icons.woff`, `resources/public/bootstrap-icons/font/fonts/bootstrap-icons.woff2` |
| Bootstrap compatibility adapter | `resources/public/js/bootstrap-legacy-bridge.js`, `go/http/testdata/bootstrap_legacy_bridge_test.js` |
| Global shell and icon presentation | `resources/templates/layout.tmpl`, `resources/public/css/orchestrator.css`, `resources/public/js/orchestrator.js`, `resources/public/js/cluster.js`, `resources/public/js/clusters.js`, `resources/public/js/audit-recovery.js`, `resources/public/js/cluster-pools.js`, `resources/templates/cluster.tmpl`, `resources/templates/clusters.tmpl` |
| Legacy page structure and workspace rhythm | `resources/templates/agent.tmpl`, `resources/templates/audit.tmpl`, `resources/templates/audit_failure_detection.tmpl`, `resources/templates/audit_recovery.tmpl`, `resources/public/css/legacy-workspace.css`, `resources/public/css/cluster-workspace.css`, `resources/public/css/clusters-workspace.css`, `resources/public/css/clusters-analysis-workspace.css` |
| D3 v7 topology adapter | `resources/public/js/d3.v7.min.js`, `resources/public/js/cluster-tree-layout.js`, `resources/public/js/cluster-tree.js`, `go/http/testdata/cluster_tree_layout_test.js`, `resources/templates/cluster.tmpl` |
| Static and live integration contracts | `go/http/static_assets_test.go`, `go/http/render_test.go`, `tests/functional/test-smoke.sh` |

---

### Task 1: Unified Asset Versioning and Local Icon Delivery

**Files:**
- Modify: `go/http/render.go:20-130`
- Modify: `go/http/render_test.go`
- Modify: `go/app/http.go:108-125`
- Modify: `go/app/http_test.go`
- Modify: `resources/templates/agents.tmpl`
- Modify: `resources/templates/audit.tmpl`
- Modify: `resources/templates/audit_failure_detection.tmpl`
- Modify: `resources/templates/audit_recovery.tmpl`
- Modify: `resources/templates/cluster.tmpl`
- Modify: `resources/templates/clusters.tmpl`
- Modify: `resources/templates/clusters_analysis.tmpl`
- Modify: `resources/templates/layout.tmpl`
- Modify: `resources/templates/seeds.tmpl`
- Modify: `resources/templates/status.tmpl`
- Create: `resources/public/bootstrap-icons/font/bootstrap-icons.min.css`
- Create: `resources/public/bootstrap-icons/font/fonts/bootstrap-icons.woff`
- Create: `resources/public/bootstrap-icons/font/fonts/bootstrap-icons.woff2`
- Test: `go/http/render_test.go`
- Test: `go/http/static_assets_test.go`
- Test: `go/app/http_test.go`

**Interfaces:**
- Produces: `computeAssetVersion() string`, stable for the process lifetime.
- Produces: `injectAssetVersion(data interface{}, version string) interface{}`, which adds `assetVersion` only to a non-nil `map[string]interface{}` that does not already contain it.
- Produces: `revalidateStaticAssets(next nethttp.Handler) nethttp.Handler`, which sets `Cache-Control: no-cache, must-revalidate` before serving an asset.
- Produces: template field `{{.assetVersion}}`, available to both content and layout templates.
- Produces: local Bootstrap Icons paths under `/bootstrap-icons/font/`.

- [ ] **Step 1: Add failing render and static-asset tests**

Add a render test that temporarily sets the package asset token, renders `templates/clusters_analysis`, and requires the same token in one layout asset and one content asset:

```go
func setAssetVersionForTest(t *testing.T, version string) {
	t.Helper()
	old := assetVersion
	assetVersion = version
	t.Cleanup(func() { assetVersion = old })
}

func TestRenderHTMLSharesAssetVersionWithContentAndLayout(t *testing.T) {
	chdirToRepoRoot(t)
	clearContentTemplateCache()
	setAssetVersionForTest(t, "asset-test-42")

	rec := httptest.NewRecorder()
	renderHTML(rec, http.StatusOK, "templates/clusters_analysis", sampleTemplateData())
	body := rec.Body.String()
	for _, want := range []string{
		`/css/orchestrator.css?v=asset-test-42`,
		`/js/clusters-analysis.js?v=asset-test-42`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered page missing shared asset version %q", want)
		}
	}
}
```

Call `setAssetVersionForTest(t, "asset-test-42")` at the start of `TestRenderHTMLUsesLocalBootstrapAssets` before rendering the template.

Extend `TestRenderHTMLUsesLocalBootstrapAssets` to require:

```go
for _, want := range []string{
	`/bootstrap5/css/bootstrap.min.css?v=asset-test-42`,
	`/bootstrap5/js/bootstrap.bundle.min.js?v=asset-test-42`,
	`/bootstrap-icons/font/bootstrap-icons.min.css?v=asset-test-42`,
} {
	if !strings.Contains(body, want) {
		t.Errorf("rendered layout missing versioned local asset %q", want)
	}
}
```

Add `TestBootstrapIconAssetsAreVendored` to `static_assets_test.go`, checking that the CSS references both `bootstrap-icons.woff` and `bootstrap-icons.woff2` and that all three files exist.

- [ ] **Step 2: Add a failing cache-policy test**

Extract the planned middleware name in a test before implementing it:

```go
func TestRevalidateStaticAssets(t *testing.T) {
	next := nethttp.HandlerFunc(func(w nethttp.ResponseWriter, _ *nethttp.Request) {
		w.WriteHeader(nethttp.StatusNoContent)
	})
	rec := httptest.NewRecorder()
	revalidateStaticAssets(next).ServeHTTP(rec, httptest.NewRequest(nethttp.MethodGet, "/css/orchestrator.css", nil))
	if got := rec.Header().Get("Cache-Control"); got != "no-cache, must-revalidate" {
		t.Fatalf("Cache-Control = %q", got)
	}
}
```

- [ ] **Step 3: Run the focused tests and record RED**

Run:

```bash
go test ./go/http -run 'Test(RenderHTMLSharesAssetVersionWithContentAndLayout|RenderHTMLUsesLocalBootstrapAssets|BootstrapIconAssetsAreVendored)$' -count=1
go test ./go/app -run '^TestRevalidateStaticAssets$' -count=1
```

Expected: failures for the undefined asset helpers, absent icon files, and dated content asset URLs.

- [ ] **Step 4: Implement the shared version before content rendering**

Add these functions to `render.go` and call `injectAssetVersion(data, assetVersion)` before `content.Execute`:

```go
var assetVersion = computeAssetVersion()

func computeAssetVersion() string {
	if exe, err := os.Executable(); err == nil {
		if info, err := os.Stat(exe); err == nil {
			return strconv.FormatInt(info.ModTime().UnixNano(), 10)
		}
	}
	return strconv.Itoa(os.Getpid())
}

func injectAssetVersion(data interface{}, version string) interface{} {
	if values, ok := data.(map[string]interface{}); ok && values != nil {
		if _, exists := values["assetVersion"]; !exists {
			values["assetVersion"] = version
		}
	}
	return data
}
```

Do not place the injection after `content.Execute`; content and layout must see the same map value.

- [ ] **Step 5: Implement static revalidation**

Add the named wrapper and use it for both prefixed and unprefixed file-server registrations:

```go
func revalidateStaticAssets(next nethttp.Handler) nethttp.Handler {
	return nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 6: Import only PR 125's icon assets and version every first-party stylesheet/script URL**

Import the three new vendor files from `origin/pr-125` without importing its Bootstrap replacement or application code:

```bash
git archive origin/pr-125 \
  resources/public/bootstrap-icons/font/bootstrap-icons.min.css \
  resources/public/bootstrap-icons/font/fonts/bootstrap-icons.woff \
  resources/public/bootstrap-icons/font/fonts/bootstrap-icons.woff2 | tar -x
```

Update template `<link>` and `<script>` assets from fixed date tokens to `?v={{.assetVersion}}`. Leave image URLs and external documentation links unchanged.

- [ ] **Step 7: Run GREEN verification**

Run:

```bash
gofmt -w go/http/render.go go/http/render_test.go go/app/http.go go/app/http_test.go
go test ./go/http -run 'Test(RenderHTMLSharesAssetVersionWithContentAndLayout|RenderHTMLUsesLocalBootstrapAssets|BootstrapIconAssetsAreVendored)$' -count=1
go test ./go/app -run '^TestRevalidateStaticAssets$' -count=1
go test ./go/http ./go/app -count=1
git diff --check
```

Expected: all commands exit 0.

- [ ] **Step 8: Commit**

```bash
git add go/http/render.go go/http/render_test.go go/http/static_assets_test.go go/app/http.go go/app/http_test.go resources/templates resources/public/bootstrap-icons
git commit -m "feat(ui): unify local asset delivery"
```

---

### Task 2: Idempotent Bootstrap 5 Compatibility Bridge

**Files:**
- Create: `resources/public/js/bootstrap-legacy-bridge.js`
- Create: `go/http/testdata/bootstrap_legacy_bridge_test.js`
- Modify: `resources/templates/layout.tmpl:1-300`
- Modify: `go/http/render_test.go`
- Modify: `go/http/static_assets_test.go`

**Interfaces:**
- Consumes: `{{.assetVersion}}` from Task 1.
- Produces: `window.OrchestratorBootstrapBridge.init(document, window.jQuery, window.bootstrap)`.
- Produces: the same API through `module.exports` for Node tests.
- Produces: `normalizeAttributes(root)` and `installJQueryAdapters($, bootstrap)` as testable members of `OrchestratorBootstrapBridge`.
- Guarantees: delegated dismiss/toggle handlers are registered once even when `init` is called repeatedly.

- [ ] **Step 1: Write the bridge behavior test**

Create a Node test that loads the CommonJS export, uses a fake document that counts `addEventListener` calls, and asserts:

```js
const assert = require('node:assert/strict');
const test = require('node:test');
const bridge = require('../../../resources/public/js/bootstrap-legacy-bridge.js');

function fakeElement(attributes) {
  const values = new Map(Object.entries(attributes || {}));
  return {
    hasAttribute: name => values.has(name),
    getAttribute: name => values.get(name) ?? null,
    setAttribute: (name, value) => values.set(name, value),
    closest: () => null
  };
}

function fakeDocument(elements) {
  const listeners = new Map();
  return {
    querySelectorAll: selector => elements.filter(element =>
      element.hasAttribute(selector.slice(1, -1))),
    addEventListener: (name, callback) => {
      if (!listeners.has(name)) listeners.set(name, []);
      listeners.get(name).push(callback);
    },
    listenerCount: name => (listeners.get(name) || []).length
  };
}

test('bridge initialization is idempotent and normalizes legacy attributes', () => {
  const legacyToggle = fakeElement({'data-toggle': 'dropdown'});
  const legacyDismiss = fakeElement({'data-dismiss': 'modal'});
  const document = fakeDocument([legacyToggle, legacyDismiss]);
  bridge.init(document, null, null);
  bridge.init(document, null, null);
  assert.equal(document.listenerCount('click'), 1);
  assert.equal(document.listenerCount('DOMContentLoaded'), 1);
  assert.equal(legacyToggle.getAttribute('data-bs-toggle'), 'dropdown');
  assert.equal(legacyDismiss.getAttribute('data-bs-dismiss'), 'modal');
});
```

Add a second test using this jQuery and Bootstrap fixture, then invoke `modal('hide')`, `dropdown('toggle')`, and `popover('dispose')` and assert each corresponding counter is `1`:

```js
function jqueryFixture() {
  function Collection(element) { this.element = element; }
  Collection.prototype.each = function(callback) {
    callback.call(this.element);
    return this;
  };
  function $(element) { return new Collection(element); }
  $.fn = Collection.prototype;
  return $;
}

function componentFixture(calls) {
  return {
    getOrCreateInstance: () => ({
      show: () => { calls.show += 1; },
      hide: () => { calls.hide += 1; },
      toggle: () => { calls.toggle += 1; },
      dispose: () => { calls.dispose += 1; }
    })
  };
}
```

- [ ] **Step 2: Add a failing layout contract**

Extend the render test to require the versioned bridge after the Bootstrap bundle and to reject the current inline compatibility listener:

```go
bundle := strings.Index(body, `/bootstrap5/js/bootstrap.bundle.min.js?v=asset-test-42`)
bridge := strings.Index(body, `/js/bootstrap-legacy-bridge.js?v=asset-test-42`)
if bundle < 0 || bridge < bundle {
	t.Fatal("Bootstrap bundle must load before the compatibility bridge")
}
if strings.Contains(body, "Bootstrap 5 compatibility: map legacy") {
	t.Fatal("layout still embeds the compatibility bridge inline")
}
```

- [ ] **Step 3: Run RED tests**

Run:

```bash
node --test go/http/testdata/bootstrap_legacy_bridge_test.js
go test ./go/http -run 'TestRenderHTMLUsesLocalBootstrapAssets' -count=1
```

Expected: missing bridge file/API and layout script failures.

- [ ] **Step 4: Implement the bridge as a focused global module**

Use this public shape:

```js
(function(root) {
  var clickListenerInstalled = false;
  var readyListenerInstalled = false;
  var bootstrapAPI = null;

  function normalizeAttributes(scope) {
    var mappings = [
      ['data-toggle', 'data-bs-toggle'],
      ['data-target', 'data-bs-target'],
      ['data-dismiss', 'data-bs-dismiss']
    ];
    mappings.forEach(function(mapping) {
      scope.querySelectorAll('[' + mapping[0] + ']').forEach(function(element) {
        if (!element.hasAttribute(mapping[1])) {
          element.setAttribute(mapping[1], element.getAttribute(mapping[0]));
        }
      });
    });
  }

  function installJQueryAdapters($, bootstrap) {
    if (!$ || !$.fn || !bootstrap) return;
    var commandAliases = {destroy: 'dispose'};
    function install(name, Component) {
      if (!Component) return;
      $.fn[name] = function(commandOrOptions) {
        return this.each(function() {
          var options = typeof commandOrOptions === 'object' ? commandOrOptions : undefined;
          var instance = Component.getOrCreateInstance(this, options);
          if (typeof commandOrOptions === 'string') {
            var command = commandAliases[commandOrOptions] || commandOrOptions;
            if (typeof instance[command] === 'function') instance[command]();
          } else if (name === 'modal' && (!options || options.show !== false)) {
            instance.show();
          }
        });
      };
    }
    install('modal', bootstrap.Modal);
    install('dropdown', bootstrap.Dropdown);
    install('popover', bootstrap.Popover);
    install('tooltip', bootstrap.Tooltip);
    install('alert', bootstrap.Alert);
  }

  function handleLegacyDismissals(event) {
    var trigger = event.target.closest('[data-dismiss]');
    if (!trigger || !bootstrapAPI) return;
    var kind = trigger.getAttribute('data-dismiss');
    var host = trigger.closest(kind === 'modal' ? '.modal' : '.alert');
    if (!host) return;
    if (kind === 'modal') bootstrapAPI.Modal.getOrCreateInstance(host).hide();
    if (kind === 'alert') bootstrapAPI.Alert.getOrCreateInstance(host).close();
  }

  function init(doc, $, bootstrap) {
    bootstrapAPI = bootstrap || bootstrapAPI;
    normalizeAttributes(doc);
    installJQueryAdapters($, bootstrap);
    if (!clickListenerInstalled) {
      doc.addEventListener('click', handleLegacyDismissals);
      clickListenerInstalled = true;
    }
    if (!readyListenerInstalled) {
      doc.addEventListener('DOMContentLoaded', function() {
        normalizeAttributes(doc);
        installJQueryAdapters($, bootstrap);
      });
      readyListenerInstalled = true;
    }
  }

  var api = {
    init: init,
    normalizeAttributes: normalizeAttributes,
    installJQueryAdapters: installJQueryAdapters
  };
  root.OrchestratorBootstrapBridge = api;
  if (typeof module === 'object' && module.exports) module.exports = api;
})(typeof window === 'undefined' ? globalThis : window);
```

Do not add topology actions or page-specific commands to this module.

- [ ] **Step 5: Replace the inline layout script**

Load the local bundle, bridge, and initializer in this order near the end of `layout.tmpl`:

```html
<script src="{{.prefix}}/bootstrap5/js/bootstrap.bundle.min.js?v={{.assetVersion}}"></script>
<script src="{{.prefix}}/js/bootstrap-legacy-bridge.js?v={{.assetVersion}}"></script>
<script>
  OrchestratorBootstrapBridge.init(document, window.jQuery, window.bootstrap);
</script>
```

- [ ] **Step 6: Run GREEN verification**

Run:

```bash
node --test go/http/testdata/bootstrap_legacy_bridge_test.js
node --check resources/public/js/bootstrap-legacy-bridge.js
go test ./go/http -run 'Test(RenderHTMLUsesLocalBootstrapAssets|AllContentTemplatesRenderWithLayout)$' -count=1
git diff --check
```

Expected: all commands exit 0 and the Node report shows zero failed tests.

- [ ] **Step 7: Commit**

```bash
git add resources/public/js/bootstrap-legacy-bridge.js go/http/testdata/bootstrap_legacy_bridge_test.js resources/templates/layout.tmpl go/http/render_test.go go/http/static_assets_test.go
git commit -m "fix(ui): centralize Bootstrap compatibility"
```

---

### Task 3: Responsive Global Shell, Icons, and Workspace Consistency

**Files:**
- Modify: `resources/templates/layout.tmpl`
- Modify: `resources/templates/cluster.tmpl`
- Modify: `resources/templates/clusters.tmpl`
- Modify: `resources/templates/agent.tmpl`
- Modify: `resources/templates/audit.tmpl`
- Modify: `resources/templates/audit_failure_detection.tmpl`
- Modify: `resources/templates/audit_recovery.tmpl`
- Modify: `resources/public/css/orchestrator.css`
- Modify: `resources/public/css/legacy-workspace.css`
- Modify: `resources/public/css/cluster-workspace.css`
- Modify: `resources/public/css/clusters-workspace.css`
- Modify: `resources/public/css/clusters-analysis-workspace.css`
- Modify: `resources/public/js/orchestrator.js`
- Modify: `resources/public/js/cluster.js`
- Modify: `resources/public/js/clusters.js`
- Modify: `resources/public/js/audit-recovery.js`
- Modify: `resources/public/js/cluster-pools.js`
- Test: `go/http/render_test.go`
- Test: `go/http/static_assets_test.go`
- Test: existing `go/http/testdata/*_test.js`

**Interfaces:**
- Consumes: local Bootstrap Icons and bridge from Tasks 1-2.
- Produces: `navbar-expand-lg` shell with labelled search and grouped status controls.
- Produces: shared CSS variables `--orc-ink`, `--orc-muted`, `--orc-line`, `--orc-surface`, `--orc-background`, `--orc-accent`, and `--orc-primary` on `:root`.
- Preserves: all existing `data-btn`, `data-command`, element IDs, authorization checks, and delegated command handlers.

- [ ] **Step 1: Add failing shell, icon, and structure contracts**

Implement the shell contract in `render_test.go`:

```go
func TestResponsiveShellUsesBootstrapIcons(t *testing.T) {
	chdirToRepoRoot(t)
	setAssetVersionForTest(t, "asset-test-42")
	rec := httptest.NewRecorder()
	renderHTML(rec, http.StatusOK, "templates/clusters", sampleTemplateData())
	body := rec.Body.String()
	for _, want := range []string{
		`navbar-expand-lg`,
		`aria-label="Search instances"`,
		`aria-label="Submit search"`,
		`class="bi bi-search"`,
		`id="nav_operational_status"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("responsive shell missing %q", want)
		}
	}
}

func TestAgentDetailUsesBootstrapGrid(t *testing.T) {
	chdirToRepoRoot(t)
	source, err := os.ReadFile("resources/templates/agent.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	body := string(source)
	if !strings.Contains(body, `class="row g-3"`) || strings.Count(body, `class="col-md-6"`) != 2 {
		t.Fatal("agent Info and Snapshots must share one two-column Bootstrap row")
	}
}
```

Add `TestActiveUIUsesBootstrapIcons` to `static_assets_test.go`. Read these exact files and reject the literal `glyphicon`: `layout.tmpl`, `cluster.tmpl`, `clusters.tmpl`, `orchestrator.js`, `cluster.js`, `clusters.js`, `audit-recovery.js`, and `cluster-pools.js`. Do not scan `bootbox.min.js`, vendored Bootstrap 3 files, or archived fixtures. Preserve the existing route-registration test from PR 119.

- [ ] **Step 2: Run RED tests**

Run:

```bash
go test ./go/http -run 'Test(RenderedNavigationUsesCurrentProjectDestinations|LayoutWebLinksHaveRegisteredRoutes|ResponsiveShellUsesBootstrapIcons|AgentDetailUsesBootstrapGrid)$' -count=1
```

Expected: failures for the `md` breakpoint, missing icon classes, status group, and agent row.

- [ ] **Step 3: Implement the global shell**

Change the nav to `navbar-expand-lg`. Give the search input `aria-label="Search instances"`, give its button `aria-label="Submit search"`, and wrap context, recovery, read-only, user, refresh, and problem controls in `#nav_operational_status`. Keep Problems visible outside the collapsed menu only when there is room; inside collapsed navigation it must remain keyboard reachable.

Use this icon mapping for active templates and generated markup:

| Legacy meaning | Bootstrap Icon |
|---|---|
| search | `bi-search` |
| settings/context | `bi-gear` |
| refresh/repeat | `bi-arrow-clockwise` |
| pause | `bi-pause-fill` |
| read-only | `bi-eye` |
| writable | `bi-pencil` |
| warning/problem | `bi-exclamation-triangle-fill` |
| healthy/success | `bi-check-circle-fill` |
| error/remove | `bi-x-circle-fill` |
| maintenance | `bi-wrench-adjustable` |
| topology/GTID | `bi-diagram-3` |
| start/stop replication | `bi-play-fill` / `bi-stop-fill` |

Retain text labels for modal actions and add `aria-hidden="true"` to decorative `<i>` elements.

- [ ] **Step 4: Remove duplicate modal activation and preserve drag exclusion**

In `orchestrator.js`, keep the single title/details delegated handler that opens `#node_modal`. Remove the redundant unhealthy-node heading handler. Its drag exclusion must continue to treat buttons, links, inputs, and `[data-node-details]` as interactive descendants.

- [ ] **Step 5: Repair agent and audit structures**

Wrap the two agent columns in one `.row g-3`, use `.col-md-6`, and correct the invalid nested table closing tags without changing data hooks. Keep the existing text pagination labels and `aria-label` values in all three audit templates; decorative arrows receive `aria-hidden="true"` spans.

- [ ] **Step 6: Unify workspace tokens and responsive rhythm**

Define the seven `--orc-*` variables in `orchestrator.css`, then make the four workspace styles consume them for page background, ink, muted text, borders, surface, accent, and primary actions. Align workspace maximum width to `1180px`, header radius to `0.75rem`, and panel border to `1px solid var(--orc-line)`. Preserve each workspace's semantic state colors and topology canvas dimensions.

At widths below `992px`, the nav collapses and workspaces retain at least `14px` horizontal padding. At widths below `620px`, hero/header content stacks and tables scroll inside their `.legacy-table-shell`; `body` must not acquire horizontal overflow.

- [ ] **Step 7: Run focused GREEN verification**

Run:

```bash
node --check resources/public/js/orchestrator.js
node --check resources/public/js/cluster.js
node --check resources/public/js/clusters.js
for file in go/http/testdata/*_test.js; do node --test "$file" || exit 1; done
go test ./go/http -run 'Test(RenderedNavigationUsesCurrentProjectDestinations|LayoutWebLinksHaveRegisteredRoutes|ResponsiveShellUsesBootstrapIcons|AgentDetailUsesBootstrapGrid|AllContentTemplatesRenderWithLayout)$' -count=1
git diff --check
```

Expected: all commands exit 0.

- [ ] **Step 8: Browser checkpoint at the actual 794-pixel width**

Rebuild and restart only the Orchestrator container, reload the in-app browser, and verify `/web/clusters`, `/web/cluster/mysql1:3306`, `/web/clusters-analysis`, `/web/audit`, and `/web/status`. Confirm the nav is collapsed, all five pages have readable full-width headers, there is no body-level horizontal overflow, dropdowns open once, and browser error/warning logs are empty.

- [ ] **Step 9: Commit**

```bash
git add \
  resources/templates/layout.tmpl \
  resources/templates/agent.tmpl \
  resources/templates/audit.tmpl \
  resources/templates/audit_failure_detection.tmpl \
  resources/templates/audit_recovery.tmpl \
  resources/templates/cluster.tmpl \
  resources/templates/clusters.tmpl \
  resources/public/css/orchestrator.css \
  resources/public/css/legacy-workspace.css \
  resources/public/css/cluster-workspace.css \
  resources/public/css/clusters-workspace.css \
  resources/public/css/clusters-analysis-workspace.css \
  resources/public/js/orchestrator.js \
  resources/public/js/cluster.js \
  resources/public/js/clusters.js \
  resources/public/js/audit-recovery.js \
  resources/public/js/cluster-pools.js \
  go/http/render_test.go \
  go/http/static_assets_test.go
git commit -m "feat(ui): unify responsive workspace chrome"
```

---

### Task 4: Behavior-Compatible D3 v7 Topology

**Files:**
- Create: `resources/public/js/d3.v7.min.js`
- Create: `resources/public/js/cluster-tree-layout.js`
- Create: `go/http/testdata/cluster_tree_layout_test.js`
- Modify: `resources/public/js/cluster-tree.js`
- Modify: `resources/templates/cluster.tmpl`
- Modify: `go/http/static_assets_test.go`
- Delete after parity passes: `resources/public/js/d3.v3.min.js`

**Interfaces:**
- Produces: `window.OrchestratorTreeLayout.layout(d3, root, treeLayout, horizontalSpacing) -> {nodes, links}`.
- `nodes` is `Array<object>` of original topology objects with current `x` and normalized `y` coordinates.
- `links` is `Array<{source: object, target: object}>` referring to original topology objects.
- Preserves: `x0` and `y0` transition origins on each original node.

- [ ] **Step 1: Write the pure layout RED test**

Load vendored D3 v7 and the planned adapter in a `vm` context. Use a root with two children, then remove one child so the remaining node must receive a new vertical coordinate:

```js
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

test('D3 v7 layout recalculates coordinates without replacing transition origins', () => {
  const context = {globalThis: null, window: null};
  context.globalThis = context;
  context.window = context;
  vm.createContext(context);
  vm.runInContext(fs.readFileSync(path.join(__dirname, '../../../resources/public/js/d3.v7.min.js'), 'utf8'), context);
  vm.runInContext(fs.readFileSync(path.join(__dirname, '../../../resources/public/js/cluster-tree-layout.js'), 'utf8'), context);

  const replica1 = {id: 'replica1', virtualDepth: 1, children: [], x0: 123, y0: 320};
  const replica2 = {id: 'replica2', virtualDepth: 1, children: []};
  const root = {id: 'primary', virtualDepth: 0, children: [replica1, replica2]};
  const tree = context.d3.tree().size([600, 640]);
  const first = context.OrchestratorTreeLayout.layout(context.d3, root, tree, 320);
  const firstByID = Object.fromEntries(first.nodes.map(node => [node.id, node]));
  const firstReplicaX = firstByID.replica1.x;

  root.children = [replica1];
  const second = context.OrchestratorTreeLayout.layout(context.d3, root, tree, 320);
  const secondByID = Object.fromEntries(second.nodes.map(node => [node.id, node]));

  assert.equal(first.nodes.length, 3);
  assert.equal(first.links.length, 2);
  assert.equal(firstByID.replica1.y, 320);
  assert.notEqual(secondByID.replica1.x, firstReplicaX);
  assert.equal(secondByID.replica1.x0, 123);
  assert.equal(secondByID.replica1.y0, 320);
});
```

Add this source guard to `static_assets_test.go`:

```go
func TestTopologyUsesD3V7WithoutCoordinateRestore(t *testing.T) {
	chdirToRepoRoot(t)
	treeSource, err := os.ReadFile("resources/public/js/cluster-tree.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"prevX", "prevY", "d3.layout.tree", "d3.svg.diagonal"} {
		if strings.Contains(string(treeSource), forbidden) {
			t.Errorf("cluster-tree.js retains legacy topology expression %q", forbidden)
		}
	}
	templateSource, err := os.ReadFile("resources/templates/cluster.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"d3.v7.min.js?v={{.assetVersion}}", "cluster-tree-layout.js?v={{.assetVersion}}"} {
		if !strings.Contains(string(templateSource), required) {
			t.Errorf("cluster template missing %q", required)
		}
	}
}
```

- [ ] **Step 2: Run RED tests**

Run:

```bash
node --test go/http/testdata/cluster_tree_layout_test.js
go test ./go/http -run '^TestTopologyUsesD3V7WithoutCoordinateRestore$' -count=1
```

Expected: missing D3 v7/adapter and legacy API guard failures.

- [ ] **Step 3: Import D3 v7 and implement the pure adapter**

Import only `resources/public/js/d3.v7.min.js` from `origin/pr-125`:

```bash
git archive origin/pr-125 resources/public/js/d3.v7.min.js | tar -x
```

Implement the adapter as a global module:

```js
(function(root) {
  function layout(d3, topologyRoot, treeLayout, horizontalSpacing) {
    var hierarchyRoot = d3.hierarchy(topologyRoot, function(node) { return node.children; });
    treeLayout(hierarchyRoot);
    hierarchyRoot.each(function(hierarchyNode) {
      var node = hierarchyNode.data;
      node.x = hierarchyNode.x;
      node.y = node.isAnchor
        ? node.virtualDepth * horizontalSpacing - horizontalSpacing / 2
        : node.virtualDepth * horizontalSpacing;
    });
    return {
      nodes: hierarchyRoot.descendants().map(function(item) { return item.data; }).reverse(),
      links: hierarchyRoot.links().map(function(link) {
        return {source: link.source.data, target: link.target.data};
      })
    };
  }
  root.OrchestratorTreeLayout = {layout: layout};
})(typeof window === 'undefined' ? globalThis : window);
```

Do not save and restore `x` or `y`. The rendering update stores current positions into `x0` and `y0` only after nodes and links have moved.

- [ ] **Step 4: Port `cluster-tree.js` to D3 v7**

Use `d3.tree`, `d3.hierarchy`, `d3.linkHorizontal`, and `nodeEnter.merge(node)`. Update click handlers to `(event, datum)`. Preserve PR 122's viewport fallback and ensure SVG width is `Math.max(viewport.width() - margins, topologyWidth)`, so deep topologies remain horizontally scrollable rather than clipped.

Load scripts in `cluster.tmpl` in this order:

```html
<script src="{{.prefix}}/js/d3.v7.min.js?v={{.assetVersion}}"></script>
<script src="{{.prefix}}/js/cluster-tree-layout.js?v={{.assetVersion}}"></script>
<script src="{{.prefix}}/js/cluster-tree.js?v={{.assetVersion}}"></script>
```

- [ ] **Step 5: Run automated GREEN verification**

Run:

```bash
node --test go/http/testdata/cluster_tree_layout_test.js
node --check resources/public/js/cluster-tree-layout.js
node --check resources/public/js/cluster-tree.js
go test ./go/http -run 'Test(TopologyUsesD3V7WithoutCoordinateRestore|RenderClusterWorkspace|RenderClusterWorkspacePreservesLegacyHooks)$' -count=1
git diff --check
```

Expected: all commands exit 0.

- [ ] **Step 6: Run the D3 browser parity gate**

Rebuild and restart only Orchestrator, reload `/web/cluster/mysql1:3306`, and verify:

1. three semantic instance cards and two links render;
2. clicking a node circle collapses and expands its descendants and existing nodes move to recalculated coordinates;
3. clicking Details opens exactly one modal;
4. clicking interactive card controls does not begin a drag;
5. the View dropdown works;
6. the page has no console errors or body-level horizontal overflow at 794 and 390 pixels.

Only after all six checks pass, remove `d3.v3.min.js` and add a static test proving no template references it.

- [ ] **Step 7: Commit**

```bash
git add resources/public/js/d3.v7.min.js resources/public/js/cluster-tree-layout.js resources/public/js/cluster-tree.js resources/templates/cluster.tmpl go/http/testdata/cluster_tree_layout_test.js go/http/static_assets_test.go
git rm resources/public/js/d3.v3.min.js
git commit -m "feat(ui): migrate topology rendering to D3 v7"
```

---

### Task 5: Full Lab and Browser Acceptance Matrix

**Files:**
- Modify: `tests/functional/test-smoke.sh`
- Modify: `go/http/static_assets_test.go`
- Create: `.superpowers/sdd/2026-08-18-consolidated-ui-integration/browser-qa.md`

**Interfaces:**
- Consumes: final rendered shell and topology assets from Tasks 1-4.
- Produces: smoke assertions for versioned local assets, compatibility bridge, Bootstrap Icons, D3 v7, and every global-navigation route.
- Produces: browser evidence table with route, viewport, state, interaction, overflow, and console results.

- [ ] **Step 1: Add RED smoke/static contracts**

Add these exact smoke calls after topology discovery:

```bash
test_body_contains "Clusters workspace" "$ORC_URL/web/clusters" 'id="clusters_workspace"'
test_body_contains "Topology workspace" "$ORC_URL/web/cluster/mysql1:3306" 'id="cluster_workspace"'
test_body_contains "Failure analysis workspace" "$ORC_URL/web/clusters-analysis" 'id="clusters_analysis_workspace"'
test_body_contains "Discover workspace" "$ORC_URL/web/discover" 'id="discover_workspace"'
test_body_contains "Audit workspace" "$ORC_URL/web/audit" 'id="audit"'
test_body_contains "Failure detection workspace" "$ORC_URL/web/audit-failure-detection" 'Failure detections'
test_body_contains "Recovery workspace" "$ORC_URL/web/audit-recovery" 'Recoveries'
test_body_contains "Status workspace" "$ORC_URL/web/status" 'id="status_workspace"'
test_body_contains "About workspace" "$ORC_URL/web/about" 'id="about_workspace"'
test_endpoint "Agents route" "$ORC_URL/web/agents" "200"
test_endpoint "Seeds route" "$ORC_URL/web/seeds" "200"
test_body_contains "D3 v7" "$ORC_URL/web/cluster/mysql1:3306" 'd3\.v7\.min\.js\?v='
test_body_contains "Topology layout adapter" "$ORC_URL/web/cluster/mysql1:3306" 'cluster-tree-layout\.js\?v='
test_body_contains "Bootstrap bridge" "$ORC_URL/web/clusters" 'bootstrap-legacy-bridge\.js\?v='
test_body_contains "Bootstrap Icons" "$ORC_URL/web/clusters" 'bootstrap-icons\.min\.css\?v='
```

Add this static contract to `static_assets_test.go`:

```go
func TestConsolidatedUIAssetContracts(t *testing.T) {
	chdirToRepoRoot(t)
	clusterTemplate, err := os.ReadFile("resources/templates/cluster.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	layoutTemplate, err := os.ReadFile("resources/templates/layout.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	combined := string(clusterTemplate) + string(layoutTemplate)
	for _, required := range []string{
		"d3.v7.min.js?v={{.assetVersion}}",
		"cluster-tree-layout.js?v={{.assetVersion}}",
		"bootstrap-legacy-bridge.js?v={{.assetVersion}}",
		"bootstrap-icons.min.css?v={{.assetVersion}}",
	} {
		if !strings.Contains(combined, required) {
			t.Errorf("consolidated UI is missing %q", required)
		}
	}
	for _, forbidden := range []string{"cdn.jsdelivr.net", "github.com/openark/orchestrator", "d3.v3.min.js"} {
		if strings.Contains(combined, forbidden) {
			t.Errorf("consolidated UI retains forbidden reference %q", forbidden)
		}
	}
}
```

The earlier render test proves that content and layout use one shared `?v=` token; this contract proves the final asset set and rejects remote or obsolete references.

- [ ] **Step 2: Run RED smoke contracts against the pre-restart server**

Run:

```bash
go test ./go/http -run '^TestConsolidatedUIAssetContracts$' -count=1
bash tests/functional/test-smoke.sh
```

Expected: the new asset assertions fail until the rebuilt Orchestrator server is mounted.

- [ ] **Step 3: Rebuild and recreate only Orchestrator safely**

Record MySQL container IDs, rebuild the Linux/arm64 binary, recreate only Orchestrator, and reseed discovery:

```bash
MYSQL_IDS_BEFORE=$(docker inspect -f '{{.Id}}' functional-mysql1-1 functional-mysql2-1 functional-mysql3-1)
docker run --rm --platform linux/arm64 \
  -v "$PWD:/work" -w /work golang:1.25.7 \
  go build -o bin/orchestrator ./go/cmd/orchestrator
docker compose -f tests/functional/docker-compose.yml up -d --no-deps --force-recreate orchestrator
source tests/functional/lib.sh
wait_for_orchestrator
discover_topology mysql1
MYSQL_IDS_AFTER=$(docker inspect -f '{{.Id}}' functional-mysql1-1 functional-mysql2-1 functional-mysql3-1)
test "$MYSQL_IDS_BEFORE" = "$MYSQL_IDS_AFTER"
```

Do not recreate or start MySQL or ProxySQL dependencies.

- [ ] **Step 4: Run the full automated suite**

Run:

```bash
gofmt -s -l go/
docker run --rm -v "$PWD:/app" -w /app golangci/golangci-lint:v2.11.4 golangci-lint run
go test ./... -count=1
for file in go/http/testdata/*_test.js; do node --test "$file" || exit 1; done
for file in tests/functional/*.sh; do bash -n "$file" || exit 1; done
docker compose -f tests/functional/docker-compose.yml -f tests/functional/docker-compose.mariadb.yml config --quiet
bash tests/functional/test-smoke.sh
git diff --check
```

Expected: `gofmt` prints nothing; every other command exits 0; MySQL container IDs match the pre-rebuild record.

- [ ] **Step 5: Execute the desktop and responsive browser matrix**

Use the in-app Browser and record each route at 1440-by-900, the natural 794-pixel application width, and 390-by-844. For every route, record:

```markdown
| Route | Viewport | State | Navigation | Interaction | Body overflow | Console |
|---|---:|---|---|---|---|---|
```

Check populated and empty states where the lab supports both. Audit and topology tables may scroll inside their own shells; `document.body.scrollWidth` must not exceed `innerWidth`. Reset the viewport override after the matrix and leave `/web/clusters` open as the deliverable tab.

- [ ] **Step 6: Commit verification contracts and evidence**

```bash
git add tests/functional/test-smoke.sh go/http/static_assets_test.go .superpowers/sdd/2026-08-18-consolidated-ui-integration/browser-qa.md
git commit -m "test(ui): verify consolidated browser workspaces"
```

---

### Task 6: Publish PR 122 and Record PR 125/126 Disposition

**External targets:**
- Update: PR 122 description on GitHub
- Comment: PR 125 and PR 126
- Repository source remains unchanged in this task

**Interfaces:**
- Consumes: green automated and browser evidence from Task 5.
- Produces: pushed PR 122 head, updated PR 122 validation summary, and evidence-based comments on PRs 125 and 126.

- [ ] **Step 1: Review the complete integration diff**

Run:

```bash
git status -sb
git diff --check origin/master...HEAD
git diff --stat origin/master...HEAD
git log --oneline origin/master..HEAD
```

Confirm no PR 126 Bootstrap 3 layout, IE8 shim, Openark link, or unrelated backend behavior entered the branch.

- [ ] **Step 2: Run final verification immediately before publishing**

Run:

```bash
go test ./... -count=1
for file in go/http/testdata/*_test.js; do node --test "$file" || exit 1; done
bash tests/functional/test-smoke.sh
git show --check --oneline HEAD
git status --short
```

Expected: all tests exit 0, the smoke summary has zero failures, and the tracked worktree is clean.

- [ ] **Step 3: Push PR 122**

```bash
git push origin codex/ui-restorative-topology-worktree
```

- [ ] **Step 4: Update PR 122's description**

Add a “Related UI PRs” section stating that PR 125's local icons, cache policy, compatibility behavior, and D3 v7 work were reapplied with the listed fixes; PR 126's local-asset intent was satisfied without adopting its Bootstrap 3/IE8 rollback. Add the final automated counts and browser matrix summary.

- [ ] **Step 5: Comment on PR 125 without closing it**

Post this evidence-based disposition:

```markdown
PR 122 now incorporates this PR's local Bootstrap Icons, shared asset cache-busting, Bootstrap 5 compatibility behavior, and D3 v7 topology migration. The integration also fixes the asset-version render order, duplicate modal activation, tree-coordinate restoration, agent grid structure, and icon-only accessibility findings. The full Go, Node, functional-smoke, and desktop/794px/390px browser results are recorded in PR 122 and its committed browser QA report. This PR remains open for the maintainers to disposition; it was not merged wholesale because it independently overlaps PR 122 in seventeen UI files.
```

- [ ] **Step 6: Comment on PR 126 without closing it**

Post:

```markdown
PR 122 now satisfies the reliable local-asset and working-navigation goals of this PR while retaining the approved Bootstrap 5 interface and current ProxySQL documentation/repository links. The Bootstrap 3/IE8 and Openark rollback in this patch was therefore not incorporated. This PR remains open for the maintainers to disposition.
```

- [ ] **Step 7: Check GitHub state**

Run:

```bash
gh pr view 122 --repo ProxySQL/orchestrator --json mergeable,mergeStateStatus,isDraft,headRefOid,statusCheckRollup,url
gh pr view 125 --repo ProxySQL/orchestrator --json state,url,comments
gh pr view 126 --repo ProxySQL/orchestrator --json state,url,comments
```

Report PR 122's mergeability and exact CI state. Do not mark the draft ready or close PR 125/126 without a separate maintainer instruction.

/*
   Copyright 2014 Outbrain Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package http

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/proxysql/orchestrator/go/config"
)

// chdirToRepoRoot finds the repository root (directory containing resources/templates)
// so template paths resolve during tests.
func chdirToRepoRoot(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "resources", "templates", "layout.tmpl")); err == nil {
			if err := os.Chdir(dir); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chdir(wd) })
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root containing resources/templates/layout.tmpl")
		}
		dir = parent
	}
}

func clearContentTemplateCache() {
	contentTemplateCache.Lock()
	contentTemplateCache.m = make(map[string]*template.Template)
	contentTemplateCache.Unlock()
}

func setAssetVersionForTest(t *testing.T, version string) {
	t.Helper()
	old := assetVersion
	assetVersion = version
	t.Cleanup(func() { assetVersion = old })
}

// contentTemplateNames returns every content template under resources/templates
// (everything except layout). Discovered from disk so new templates are covered.
func contentTemplateNames(t *testing.T) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join("resources", "templates", "*.tmpl"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, path := range matches {
		base := filepath.Base(path)
		if base == "layout.tmpl" {
			continue
		}
		name := strings.TrimSuffix(base, ".tmpl")
		names = append(names, "templates/"+name)
	}
	if len(names) == 0 {
		t.Fatal("no content templates found under resources/templates")
	}
	return names
}

func sampleTemplateData() map[string]interface{} {
	return map[string]interface{}{
		"title":                         "test",
		"prefix":                        "",
		"agentsHttpActive":              false,
		"autoshow_problems":             false,
		"authorizedForAction":           false,
		"userId":                        "",
		"removeTextFromHostnameDisplay": "",
		"webMessage":                    "",
		"clusterName":                   "test-cluster",
		"page":                          0,
		"searchString":                  "",
		"auditHostname":                 "",
		"auditPort":                     0,
		"agentHost":                     "",
		"seedId":                        "",
		"detectionId":                   0,
		"clusterAlias":                  "",
		"recoveryId":                    "",
		"recoveryUid":                   "",
		"pseudoGTIDModeEnabled":         false,
		"contextMenuVisible":            false,
		"providerName":                  "MySQL",
		"defaultInstancePort":           3306,
	}
}

func TestRenderedNavigationUsesCurrentProjectDestinations(t *testing.T) {
	chdirToRepoRoot(t)
	clearContentTemplateCache()

	rec := httptest.NewRecorder()
	renderHTML(rec, http.StatusOK, "templates/about", sampleTemplateData())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, expected := range []string{
		`href="https://github.com/ProxySQL/orchestrator"`,
		`href="https://proxysql.github.io/orchestrator/"`,
		`href="https://github.com/ProxySQL/orchestrator/blob/master/docs/faq.md"`,
		`>Documentation</a>`,
		`>Operations</a>`,
		`>Failure detections</a>`,
		`>Recoveries</a>`,
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("rendered navigation is missing %q", expected)
		}
	}
	if strings.Contains(strings.ToLower(body), "github.com/openark/orchestrator") {
		t.Fatal("rendered navigation still links to the archived openark repository")
	}
	if strings.Contains(body, `href="/web/agents"`) || strings.Contains(body, `href="/web/seeds"`) {
		t.Fatal("agent-only navigation must be hidden while the agent HTTP service is disabled")
	}
}

func TestRenderedNavigationShowsAgentToolsWhenEnabled(t *testing.T) {
	chdirToRepoRoot(t)
	clearContentTemplateCache()

	data := sampleTemplateData()
	data["agentsHttpActive"] = true
	rec := httptest.NewRecorder()
	renderHTML(rec, http.StatusOK, "templates/about", data)
	body := rec.Body.String()
	for _, expected := range []string{`href="/web/agents"`, `href="/web/seeds"`} {
		if !strings.Contains(body, expected) {
			t.Errorf("agent-enabled navigation is missing %q", expected)
		}
	}
}

func TestRenderAboutDescribesCurrentProject(t *testing.T) {
	chdirToRepoRoot(t)
	clearContentTemplateCache()

	rec := httptest.NewRecorder()
	renderHTML(rec, http.StatusOK, "templates/about", sampleTemplateData())
	body := rec.Body.String()
	for _, expected := range []string{
		`id="about_workspace"`,
		`MySQL 9.7`,
		`PostgreSQL 12`,
		`ProxySQL/orchestrator`,
		`Apache License 2.0`,
		`/css/legacy-workspace.css`,
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("modern About page is missing %q", expected)
		}
	}
}

func TestRenderOperationalPagesHaveIntentionalStates(t *testing.T) {
	chdirToRepoRoot(t)
	clearContentTemplateCache()

	tests := []struct {
		template string
		hook     string
		message  string
	}{
		{template: "templates/status", hook: `id="status_workspace"`, message: "Loading node status"},
		{template: "templates/audit", hook: `id="audit_empty"`, message: "No audit operations recorded"},
		{template: "templates/audit_failure_detection", hook: `id="audit_empty"`, message: "No failure detections recorded"},
		{template: "templates/audit_recovery", hook: `id="audit_empty"`, message: "No recoveries recorded"},
		{template: "templates/agents", hook: `id="agents_disabled"`, message: "Agent HTTP service is disabled"},
		{template: "templates/seeds", hook: `id="seeds_disabled"`, message: "Agent HTTP service is disabled"},
	}
	for _, tt := range tests {
		t.Run(tt.template, func(t *testing.T) {
			rec := httptest.NewRecorder()
			renderHTML(rec, http.StatusOK, tt.template, sampleTemplateData())
			body := rec.Body.String()
			for _, expected := range []string{tt.hook, tt.message, `/css/legacy-workspace.css`} {
				if !strings.Contains(body, expected) {
					t.Errorf("rendered page is missing %q", expected)
				}
			}
		})
	}
}

func TestRenderStatusWorkspaceHasStructuredHealthTable(t *testing.T) {
	chdirToRepoRoot(t)
	clearContentTemplateCache()

	rec := httptest.NewRecorder()
	renderHTML(rec, http.StatusOK, "templates/status", sampleTemplateData())
	body := rec.Body.String()
	for _, expected := range []string{
		`id="status_summary"`,
		`<thead>`,
		`<tbody>`,
		`<th scope="col">Node</th>`,
		`<th scope="col">Hostname</th>`,
		`id="status_actions"`,
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("structured Status workspace is missing %q", expected)
		}
	}
}

func TestDiscoverUsesConfiguredProviderAndPort(t *testing.T) {
	chdirToRepoRoot(t)
	clearContentTemplateCache()

	oldProvider, oldPort := config.Config.ProviderType, config.Config.DefaultInstancePort
	config.Config.ProviderType, config.Config.DefaultInstancePort = "postgresql", 5432
	t.Cleanup(func() {
		config.Config.ProviderType, config.Config.DefaultInstancePort = oldProvider, oldPort
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/web/discover", nil)
	Web.Discover(rec, req)
	body := rec.Body.String()
	for _, expected := range []string{
		`id="discover_workspace"`,
		`Enter a PostgreSQL hostname and port`,
		`value="5432"`,
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("provider-aware Discover page is missing %q", expected)
		}
	}
}

func TestAgentDetailRoutesExplainWhenAgentHTTPIsDisabled(t *testing.T) {
	chdirToRepoRoot(t)
	clearContentTemplateCache()

	oldServeAgents := config.Config.ServeAgentsHttp
	config.Config.ServeAgentsHttp = false
	t.Cleanup(func() { config.Config.ServeAgentsHttp = oldServeAgents })

	tests := []struct {
		name       string
		path       string
		handler    http.HandlerFunc
		disabledID string
		legacyJS   string
	}{
		{name: "agent detail", path: "/web/agent/mysql1", handler: Web.Agent, disabledID: `id="agents_disabled"`, legacyJS: `/js/agent.js`},
		{name: "seed detail", path: "/web/seed-details/1", handler: Web.AgentSeedDetails, disabledID: `id="seeds_disabled"`, legacyJS: `/js/seed.js`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tt.handler(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))
			body := rec.Body.String()
			if !strings.Contains(body, tt.disabledID) || !strings.Contains(body, "Agent HTTP service is disabled") {
				t.Fatalf("disabled agent route did not explain its state: %s", truncate(body, 600))
			}
			if strings.Contains(body, tt.legacyJS) {
				t.Errorf("disabled agent route still loads %q", tt.legacyJS)
			}
		})
	}
}

func TestRenderHTMLYield(t *testing.T) {
	chdirToRepoRoot(t)
	clearContentTemplateCache()

	rec := httptest.NewRecorder()
	renderHTML(rec, http.StatusOK, "templates/clusters", map[string]interface{}{
		"title":  "clusters",
		"prefix": "",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "Internal Server Error") {
		t.Fatalf("got error body: %s", body)
	}
	if !strings.Contains(body, `id="clusters"`) {
		t.Fatalf("expected clusters content via yield, body snippet: %s", truncate(body, 500))
	}
	if !strings.Contains(body, "<!doctype html>") {
		t.Fatalf("expected layout wrapper, body snippet: %s", truncate(body, 200))
	}
	if !strings.Contains(body, "Orchestrator - clusters") {
		t.Fatalf("expected title from layout data, body snippet: %s", truncate(body, 300))
	}
}

func TestRenderHTMLUsesLocalBootstrapAssets(t *testing.T) {
	chdirToRepoRoot(t)
	clearContentTemplateCache()
	setAssetVersionForTest(t, "asset-test-42")

	rec := httptest.NewRecorder()
	renderHTML(rec, http.StatusOK, "templates/clusters", sampleTemplateData())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, expected := range []string{
		`/bootstrap5/css/bootstrap.min.css?v=asset-test-42`,
		`/bootstrap5/js/bootstrap.bundle.min.js?v=asset-test-42`,
		`/js/bootstrap-legacy-bridge.js?v=asset-test-42`,
		`/bootstrap-icons/font/bootstrap-icons.min.css?v=asset-test-42`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected locally served Bootstrap asset %q, body snippet: %s", expected, truncate(body, 500))
		}
	}
	if strings.Contains(body, "cdn.jsdelivr.net/npm/bootstrap") {
		t.Fatal("rendered layout still depends on the external Bootstrap CDN")
	}
	bundle := strings.Index(body, `/bootstrap5/js/bootstrap.bundle.min.js?v=asset-test-42`)
	bridge := strings.Index(body, `/js/bootstrap-legacy-bridge.js?v=asset-test-42`)
	if bundle < 0 || bridge < bundle {
		t.Fatal("Bootstrap bundle must load before the compatibility bridge")
	}
	if strings.Contains(body, "Bootstrap 5 compatibility: map legacy") {
		t.Fatal("layout still embeds the compatibility bridge inline")
	}
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

func TestRenderedNavigationUsesRegisteredClusterRoutes(t *testing.T) {
	chdirToRepoRoot(t)
	clearContentTemplateCache()

	rec := httptest.NewRecorder()
	renderHTML(rec, http.StatusOK, "templates/clusters", sampleTemplateData())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, expected := range []string{
		`href="/web/clusters"`,
		`href="/web/clusters-analysis"`,
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("rendered navigation is missing registered route %q", expected)
		}
	}
	for _, broken := range []string{
		`href="/web/clusters/"`,
		`href="/web/clusters-analysis/"`,
	} {
		if strings.Contains(body, broken) {
			t.Errorf("rendered navigation links to unregistered route %q", broken)
		}
	}
}

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

func TestRenderClustersWorkspace(t *testing.T) {
	chdirToRepoRoot(t)
	clearContentTemplateCache()

	rec := httptest.NewRecorder()
	renderHTML(rec, http.StatusOK, "templates/clusters", sampleTemplateData())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, expected := range []string{
		`id="clusters_workspace"`,
		`aria-labelledby="clusters_workspace_title"`,
		`id="clusters_workspace_title"`,
		`id="clusters_known_count"`,
		`role="status"`,
		`href="/web/discover"`,
		`>Discover instance</a>`,
		`id="clusters_list"`,
		`id="clusters"`,
		`/css/clusters-workspace.css`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected clusters workspace contract %q, body snippet: %s", expected, truncate(body, 500))
		}
	}
}

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

func TestRenderClustersAnalysisWorkspaceUsesCurrentAssetRevision(t *testing.T) {
	chdirToRepoRoot(t)
	clearContentTemplateCache()
	setAssetVersionForTest(t, "asset-test-42")

	rec := httptest.NewRecorder()
	renderHTML(rec, http.StatusOK, "templates/clusters_analysis", sampleTemplateData())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, expected := range []string{
		`href="/css/clusters-analysis-workspace.css?v=asset-test-42"`,
		`src="/js/clusters-analysis.js?v=asset-test-42"`,
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("expected current failure analysis asset revision %q", expected)
		}
	}
}

func TestRenderClusterWorkspace(t *testing.T) {
	chdirToRepoRoot(t)
	clearContentTemplateCache()

	rec := httptest.NewRecorder()
	renderHTML(rec, http.StatusOK, "templates/cluster", sampleTemplateData())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, expected := range []string{
		`id="cluster_workspace"`,
		`id="cluster_canvas"`,
		`/css/cluster-workspace.css`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected cluster workspace contract %q, body snippet: %s", expected, truncate(body, 500))
		}
	}
}

func TestRenderClusterWorkspacePreservesLegacyHooks(t *testing.T) {
	chdirToRepoRoot(t)
	clearContentTemplateCache()

	rec := httptest.NewRecorder()
	renderHTML(rec, http.StatusOK, "templates/cluster", sampleTemplateData())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, expected := range []string{
		`id="cluster_sidebar"`,
		`id="cluster_container"`,
		`id="node_modal"`,
		`id="cluster_view_toggle"`,
		`data-bs-toggle="dropdown"`,
		`aria-controls="cluster_view_menu"`,
		`>View</button>`,
		`id="cluster_view_menu"`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected preserved cluster hook %q, body snippet: %s", expected, truncate(body, 500))
		}
	}

	headerEnd := strings.Index(body, `</header>`)
	viewMenu := strings.Index(body, `id="cluster_sidebar"`)
	if headerEnd == -1 || viewMenu == -1 || viewMenu > headerEnd {
		t.Fatal("expected cluster View menu to remain inline with the cluster header")
	}
	infoCommand := strings.Index(body, `data-command="info"`)
	if infoCommand == -1 {
		t.Fatal("expected rendered cluster information command")
	}
	infoCommandEnd := strings.Index(body[infoCommand:], `</a>`)
	if infoCommandEnd == -1 {
		t.Fatal("expected rendered cluster information command")
	}
	infoMarkup := body[infoCommand : infoCommand+infoCommandEnd]
	if count := strings.Count(infoMarkup, `<i`); count != 1 {
		t.Fatalf("cluster information command rendered %d icons, want one Bootstrap Icon", count)
	}
	if !strings.Contains(infoMarkup, `class="bi `) || !strings.Contains(infoMarkup, `aria-hidden="true"`) {
		t.Fatal("cluster information command icon must be a decorative Bootstrap Icon")
	}

	for _, command := range []string{
		"info",
		"colorize-dc",
		"compact-display",
		"pool-indicator",
		"anonymize",
		"alias",
		"silent-ui",
	} {
		hook := `data-command="` + command + `"`
		if count := strings.Count(body, hook); count != 1 {
			t.Errorf("cluster View menu command %q rendered %d times, want exactly once", command, count)
		}
	}
}

func TestLayoutWebLinksHaveRegisteredRoutes(t *testing.T) {
	chdirToRepoRoot(t)
	clearContentTemplateCache()

	rec := httptest.NewRecorder()
	renderHTML(rec, http.StatusOK, "templates/clusters", sampleTemplateData())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	router := chi.NewRouter()
	web := HttpWeb{}
	web.RegisterRequests(router)

	registeredGETRoutes := make(map[string]struct{})
	err := chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if method == http.MethodGet {
			registeredGETRoutes[route] = struct{}{}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	hrefPattern := regexp.MustCompile(`href="(/web/[^"#?]+)"`)
	matches := hrefPattern.FindAllStringSubmatch(rec.Body.String(), -1)
	if len(matches) == 0 {
		t.Fatal("rendered layout contains no constant /web/ links")
	}
	for _, match := range matches {
		href := match[1]
		if _, ok := registeredGETRoutes[href]; !ok {
			t.Errorf("navbar link %q has no matching GET route", href)
		}
	}
}

// TestLayoutRequiresYield guards the martini-contrib/render contract: layout.tmpl
// uses {{yield}} to inject page content. Parsing layout without that FuncMap must fail.
func TestLayoutRequiresYield(t *testing.T) {
	chdirToRepoRoot(t)

	layoutPath := filepath.Join("resources", "templates", "layout.tmpl")
	src, err := os.ReadFile(layoutPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "{{yield}}") {
		t.Fatal("layout.tmpl no longer uses {{yield}}; update renderHTML if composition changed")
	}

	_, err = template.New("layout").Parse(string(src))
	if err == nil {
		t.Fatal("expected layout parse without yield FuncMap to fail")
	}
	if !strings.Contains(err.Error(), `function "yield" not defined`) {
		t.Fatalf("unexpected parse error: %v", err)
	}

	_, err = template.New("layout").Funcs(template.FuncMap{
		"yield": func() template.HTML { return "" },
	}).Parse(string(src))
	if err != nil {
		t.Fatalf("layout should parse when yield is registered: %v", err)
	}
}

func TestRenderHTMLAllWebTemplates(t *testing.T) {
	chdirToRepoRoot(t)
	clearContentTemplateCache()

	data := sampleTemplateData()
	for _, name := range contentTemplateNames(t) {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			renderHTML(rec, http.StatusOK, name, data)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			if strings.Contains(body, "Internal Server Error") {
				t.Fatalf("error body: %s", body)
			}
			if !strings.Contains(body, "<!doctype html>") {
				t.Fatalf("missing layout for %s", name)
			}
		})
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

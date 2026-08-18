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
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestBundledJQueryMeetsSecurityFloor guards against regressing the bundled
// jQuery below 3.5.0 (CVE scanners / customer security requirements).
func TestBundledJQueryMeetsSecurityFloor(t *testing.T) {
	chdirToRepoRoot(t)

	f, err := os.Open(filepath.Join("resources", "public", "js", "jquery.min.js"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	line, err := bufio.NewReader(f).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	// e.g. /*! jQuery v3.7.1 | (c) OpenJS Foundation ...
	re := regexp.MustCompile(`jQuery v(\d+)\.(\d+)\.(\d+)`)
	m := re.FindStringSubmatch(line)
	if m == nil {
		t.Fatalf("could not parse jQuery version from header: %q", strings.TrimSpace(line))
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])

	// Floor: 3.5.0
	if major < 3 || (major == 3 && minor < 5) {
		t.Fatalf("bundled jQuery %d.%d.%d is below required 3.5.0", major, minor, patch)
	}
	t.Logf("bundled jQuery %d.%d.%d", major, minor, patch)
}

func TestBootstrapIconAssetsAreVendored(t *testing.T) {
	chdirToRepoRoot(t)

	iconDir := filepath.Join("resources", "public", "bootstrap-icons", "font")
	css, err := os.ReadFile(filepath.Join(iconDir, "bootstrap-icons.min.css"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"bootstrap-icons.woff", "bootstrap-icons.woff2"} {
		if !strings.Contains(string(css), want) {
			t.Errorf("Bootstrap Icons stylesheet is missing font reference %q", want)
		}
	}
	for _, name := range []string{"bootstrap-icons.min.css", "fonts/bootstrap-icons.woff", "fonts/bootstrap-icons.woff2"} {
		if _, err := os.Stat(filepath.Join(iconDir, name)); err != nil {
			t.Errorf("Bootstrap Icons asset %q is not vendored: %v", name, err)
		}
	}
}

func TestBootstrapLegacyBridgeIsShipped(t *testing.T) {
	chdirToRepoRoot(t)

	path := filepath.Join("resources", "public", "js", "bootstrap-legacy-bridge.js")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Bootstrap legacy bridge is not shipped: %v", err)
	}
	for _, want := range []string{
		"OrchestratorBootstrapBridge",
		"normalizeAttributes",
		"installJQueryAdapters",
	} {
		if !strings.Contains(string(source), want) {
			t.Errorf("Bootstrap legacy bridge is missing %q", want)
		}
	}
}

func TestActiveUIUsesBootstrapIcons(t *testing.T) {
	chdirToRepoRoot(t)

	for _, path := range []string{
		"resources/templates/layout.tmpl",
		"resources/templates/cluster.tmpl",
		"resources/templates/clusters.tmpl",
		"resources/public/js/orchestrator.js",
		"resources/public/js/cluster.js",
		"resources/public/js/clusters.js",
		"resources/public/js/audit-recovery.js",
		"resources/public/js/cluster-pools.js",
	} {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(source), "glyphicon") {
			t.Errorf("active UI asset %q still uses glyphicon", path)
		}
	}
}

func TestStaticClusterWorkspaceAssets(t *testing.T) {
	chdirToRepoRoot(t)

	cssPath := filepath.Join("resources", "public", "css", "cluster-workspace.css")
	css, err := os.ReadFile(cssPath)
	if err != nil {
		t.Fatalf("cluster workspace stylesheet is not shipped: %v", err)
	}
	selectors := workspaceCSSSelectors(string(css))
	if len(selectors) == 0 {
		t.Fatal("cluster workspace stylesheet does not contain any scoped rules")
	}
	for _, selector := range selectors {
		for _, part := range strings.Split(selector, ",") {
			if !isClusterWorkspaceSelector(part) {
				t.Errorf("cluster workspace selector must be scoped below #cluster_workspace: %q", strings.TrimSpace(part))
			}
		}
	}

	source, err := os.ReadFile(filepath.Join("resources", "public", "js", "orchestrator.js"))
	if err != nil {
		t.Fatal(err)
	}
	for _, hook := range []string{
		"instance-identity",
		"instance-role",
		"instance-health",
		"instance-replication",
		"instance-actions",
	} {
		if !strings.Contains(string(source), hook) {
			t.Errorf("orchestrator.js does not ship semantic card hook %q", hook)
		}
	}
}

func TestStaticClustersLandingAssets(t *testing.T) {
	chdirToRepoRoot(t)

	cssPath := filepath.Join("resources", "public", "css", "clusters-workspace.css")
	css, err := os.ReadFile(cssPath)
	if err != nil {
		t.Fatalf("clusters landing stylesheet is not shipped: %v", err)
	}
	selectors := workspaceCSSSelectors(string(css))
	if len(selectors) == 0 {
		t.Fatal("clusters landing stylesheet does not contain any scoped rules")
	}
	for _, selector := range selectors {
		for _, part := range strings.Split(selector, ",") {
			if !isWorkspaceSelector(part, "#clusters_workspace") {
				t.Errorf("clusters landing selector must be scoped below #clusters_workspace: %q", strings.TrimSpace(part))
			}
		}
	}
	for _, snippet := range []string{
		`var(--bs-gutter-x`,
		`.badge.label-info`,
		`.badge.label-warning`,
		`.badge.label-danger`,
		`.badge.label-stale`,
		`.badge.label-fatal`,
		`.badge.label-errant`,
	} {
		if !strings.Contains(string(css), snippet) {
			t.Errorf("clusters landing stylesheet is missing %q", snippet)
		}
	}

	source, err := os.ReadFile(filepath.Join("resources", "public", "js", "clusters.js"))
	if err != nil {
		t.Fatal(err)
	}
	for _, snippet := range []string{
		`resolveClusterHealthSummary(`,
		`data-health-state`,
		`cluster-health-label`,
	} {
		if !strings.Contains(string(source), snippet) {
			t.Errorf("clusters.js does not emit explicit landing health state %q", snippet)
		}
	}
}

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
	if unscoped := unscopedWorkspaceCSSSelectors(css, "#clusters_analysis_workspace"); len(unscoped) > 0 {
		t.Errorf("failure analysis stylesheet selectors must be scoped below #clusters_analysis_workspace: %q", unscoped)
	}

	intermediateBreakpointFound := false
	mediaPattern := regexp.MustCompile(`@media\s*\(max-width:\s*(\d+)px\)\s*\{`)
	columnPattern := regexp.MustCompile(`(?s)` + regexp.QuoteMeta("#clusters_analysis_workspace .clusters-analysis-columns") + `\s*\{([^}]*)\}`)
	clusterPattern := regexp.MustCompile(`(?s)` + regexp.QuoteMeta("#clusters_analysis_workspace .analysis-cluster") + `\s*\{([^}]*)\}`)
	flexibleGrid := "grid-template-columns: minmax(0, .9fr) minmax(0, 1.6fr) minmax(0, .7fr);"
	for _, match := range mediaPattern.FindAllStringSubmatchIndex(css, -1) {
		breakpoint, err := strconv.Atoi(css[match[2]:match[3]])
		if err != nil || breakpoint <= 760 {
			continue
		}
		openBrace := match[1] - 1
		closeBrace := matchingCSSBrace(css, openBrace)
		if closeBrace < 0 {
			continue
		}
		mediaCSS := css[openBrace+1 : closeBrace]
		columns := columnPattern.FindStringSubmatch(mediaCSS)
		cluster := clusterPattern.FindStringSubmatch(mediaCSS)
		if columns != nil && cluster != nil &&
			strings.Contains(columns[1], flexibleGrid) && strings.Contains(cluster[1], flexibleGrid) {
			intermediateBreakpointFound = true
			break
		}
	}
	if !intermediateBreakpointFound {
		t.Error("failure analysis stylesheet needs an intermediate breakpoint above 760px with flexible label and row grids")
	}

	braceDepth := 0
	for _, character := range css {
		switch character {
		case '{':
			braceDepth++
		case '}':
			braceDepth--
		}
		if braceDepth < 0 {
			break
		}
	}
	if braceDepth != 0 {
		t.Error("failure analysis stylesheet has unbalanced braces")
	}
}

func TestUnscopedWorkspaceCSSSelectorsRejectsArbitraryGlobalRule(t *testing.T) {
	css := `
#clusters_analysis_workspace .analysis-cluster { display: grid; }
@media (max-width: 760px) {
	#clusters_analysis_workspace .analysis-cluster { display: block; }
	.unexpected-global { display: none; }
}`

	got := unscopedWorkspaceCSSSelectors(css, "#clusters_analysis_workspace")
	want := []string{".unexpected-global"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unscopedWorkspaceCSSSelectors() = %q, want %q", got, want)
	}
}

func TestClusterTopologyRendererUsesWorkspaceCanvasViewport(t *testing.T) {
	chdirToRepoRoot(t)

	source, err := os.ReadFile(filepath.Join("resources", "public", "js", "cluster-tree.js"))
	if err != nil {
		t.Fatal(err)
	}
	for _, snippet := range []string{
		`var viewport = $("#cluster_canvas");`,
		`viewport = $("#cluster_container");`,
		`var svgWidth = Math.max(viewport.width() - margin.right - margin.left, topologyWidth);`,
		`var topologyWidth = (maxDepth + 1) * horizontalSpacing;`,
		`var svgHeight = viewport.height() - margin.top - margin.bottom;`,
	} {
		if !strings.Contains(string(source), snippet) {
			t.Errorf("cluster-tree.js must size the topology from the workspace viewport: missing %q", snippet)
		}
	}

	css, err := os.ReadFile(filepath.Join("resources", "public", "css", "cluster-workspace.css"))
	if err != nil {
		t.Fatal(err)
	}
	for _, snippet := range []string{
		`#cluster_workspace #cluster_canvas`,
		`overflow: auto;`,
		`#cluster_workspace #cluster_wrapper`,
		`min-width: 960px;`,
		`#cluster_workspace .instance.instance-diagram`,
	} {
		if !strings.Contains(string(css), snippet) {
			t.Errorf("cluster workspace must retain explorable topology cards on narrow screens: missing %q", snippet)
		}
	}
}

func TestClusterWorkspaceCommandsPreventAnchorNavigation(t *testing.T) {
	chdirToRepoRoot(t)

	source, err := os.ReadFile(filepath.Join("resources", "public", "js", "cluster.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), `$("#cluster_sidebar").on("click", "a[data-command]", function(event) {
      event.preventDefault();
    });`) {
		t.Fatal("cluster workspace command rail does not prevent href default navigation")
	}
}

func TestClusterNodeCardDragCancelKeepsSemanticTextDraggable(t *testing.T) {
	chdirToRepoRoot(t)

	source, err := os.ReadFile(filepath.Join("resources", "public", "js", "cluster.js"))
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`cancel:\s*"([^"]+)"`).FindStringSubmatch(string(source))
	if match == nil {
		t.Fatal("cluster instance draggable does not define a cancel selector")
	}
	cancelled := make(map[string]bool)
	for _, selector := range strings.Split(match[1], ",") {
		cancelled[strings.TrimSpace(selector)] = true
	}
	if cancelled["span"] {
		t.Fatal("semantic card text spans must remain draggable")
	}
	for _, selector := range []string{"button", "a", ".instance-glyphs", ".instance-trailer"} {
		if !cancelled[selector] {
			t.Errorf("cluster instance draggable must cancel actual control %q", selector)
		}
	}
}

func TestNonClusterDetailsTriggerOpensNodeModal(t *testing.T) {
	chdirToRepoRoot(t)

	source, err := os.ReadFile(filepath.Join("resources", "public", "js", "orchestrator.js"))
	if err != nil {
		t.Fatal(err)
	}
	want := regexp.MustCompile(`(?s)if \(renderType != "cluster"\) \{\s*popoverElement\.find\("\[data-node-details\]"\)\.click\(function\(e\) \{\s*e\.preventDefault\(\);\s*e\.stopPropagation\(\);\s*openNodeModal\(instance\);`)
	if !want.Match(source) {
		t.Fatal("non-cluster details trigger does not open the existing node modal")
	}
}

func TestOpenNodeModalExplicitlyShowsBootstrapModal(t *testing.T) {
	chdirToRepoRoot(t)

	source, err := os.ReadFile(filepath.Join("resources", "public", "js", "orchestrator.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), `$('#node_modal').modal('show');`) {
		t.Fatal("openNodeModal must explicitly show the Bootstrap modal")
	}
}

func TestClusterProblemsMoveIntoWorkspaceHeader(t *testing.T) {
	chdirToRepoRoot(t)

	source, err := os.ReadFile(filepath.Join("resources", "public", "js", "cluster.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), `$("#instance_problems").appendTo(".cluster-workspace-header").addClass("cluster-workspace-problems");`) {
		t.Fatal("cluster problems control must be placed in the workspace header")
	}
}

func TestErrantGTIDCardIsNotHealthy(t *testing.T) {
	chdirToRepoRoot(t)

	source, err := os.ReadFile(filepath.Join("resources", "public", "js", "orchestrator.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), `instance.errantGTIDProblem() ? "warning" : "healthy"`) {
		t.Fatal("errant-GTID-only instances must receive a non-healthy semantic state")
	}
}

func TestIsClusterWorkspaceSelector(t *testing.T) {
	for _, test := range []struct {
		selector string
		want     bool
	}{
		{selector: "#cluster_workspace", want: true},
		{selector: "#cluster_workspace .instance-card", want: true},
		{selector: "#cluster_workspace:hover", want: true},
		{selector: "#cluster_workspace-other", want: false},
		{selector: "#cluster_workspaceé", want: false},
		{selector: "[data-target=\"#cluster_workspace\"]", want: false},
		{selector: "/* #cluster_workspace */ .instance-card", want: false},
	} {
		t.Run(test.selector, func(t *testing.T) {
			if got := isClusterWorkspaceSelector(test.selector); got != test.want {
				t.Fatalf("isClusterWorkspaceSelector(%q) = %t, want %t", test.selector, got, test.want)
			}
		})
	}
}

func isClusterWorkspaceSelector(selector string) bool {
	return isWorkspaceSelector(selector, "#cluster_workspace")
}

func unscopedWorkspaceCSSSelectors(css string, workspaceID string) []string {
	var unscoped []string
	for _, selector := range workspaceCSSSelectors(css) {
		for _, part := range strings.Split(selector, ",") {
			part = strings.TrimSpace(part)
			if !isWorkspaceSelector(part, workspaceID) {
				unscoped = append(unscoped, part)
			}
		}
	}
	return unscoped
}

func isWorkspaceSelector(selector string, workspaceID string) bool {
	selector = stripCSSComments(selector)
	brackets := 0
	parentheses := 0
	var quote byte
	for i := 0; i < len(selector); i++ {
		c := selector[i]
		if quote != 0 {
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
		case '[':
			brackets++
		case ']':
			brackets--
		case '(':
			parentheses++
		case ')':
			parentheses--
		case '#':
			if brackets == 0 && parentheses == 0 && strings.HasPrefix(selector[i:], workspaceID) {
				end := i + len(workspaceID)
				if end == len(selector) || !isCSSIdentifierCharacter(selector[end]) {
					return true
				}
			}
		}
	}
	return false
}

func isCSSIdentifierCharacter(c byte) bool {
	return c >= 0x80 || c == '-' || c == '_' || c == '\\' || ('0' <= c && c <= '9') || ('A' <= c && c <= 'Z') || ('a' <= c && c <= 'z')
}

func stripCSSComments(css string) string {
	for {
		start := strings.Index(css, "/*")
		if start < 0 {
			return css
		}
		end := strings.Index(css[start+2:], "*/")
		if end < 0 {
			return css[:start]
		}
		css = css[:start] + css[start+end+4:]
	}
}

func workspaceCSSSelectors(css string) []string {
	css = stripCSSComments(css)
	var selectors []string
	for len(css) > 0 {
		open := strings.IndexByte(css, '{')
		if open < 0 {
			break
		}
		header := strings.TrimSpace(css[:open])
		close := matchingCSSBrace(css, open)
		if close < 0 {
			return append(selectors, header)
		}
		contents := css[open+1 : close]
		if strings.HasPrefix(header, "@media") || strings.HasPrefix(header, "@supports") || strings.HasPrefix(header, "@container") || strings.HasPrefix(header, "@layer") {
			selectors = append(selectors, workspaceCSSSelectors(contents)...)
		} else {
			selectors = append(selectors, header)
		}
		css = css[close+1:]
	}
	return selectors
}

func matchingCSSBrace(css string, open int) int {
	depth := 0
	for i := open; i < len(css); i++ {
		switch css[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func TestAuditFailoverHarnessSafetyContracts(t *testing.T) {
	chdirToRepoRoot(t)
	contents, err := os.ReadFile(filepath.Join("tests", "functional", "test-audit-ui-failover.sh"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(contents)
	for _, required := range []string{
		`deadline=$((SECONDS + 90))`,
		`--max-time 2`,
		`while (( SECONDS < deadline ))`,
		`remaining=$((deadline - SECONDS))`,
		`curl_args=(--max-time "$remaining")`,
		`if [ "$MYSQL1_STOPPED" != true ]; then`,
	} {
		if !strings.Contains(script, required) {
			t.Errorf("failover harness missing safety contract %q", required)
		}
	}
	if strings.Contains(script, `$COMPOSE start mysql2 mysql3`) {
		t.Error("restore_lab must never start mysql2 or mysql3")
	}
}

func TestSmokeEndsOnlyMaintenanceCreatedByItsBeginCall(t *testing.T) {
	chdirToRepoRoot(t)
	contents, err := os.ReadFile(filepath.Join("tests", "functional", "test-smoke.sh"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(contents)
	if !strings.Contains(script, `/api/end-maintenance/$MAINTENANCE_KEY`) {
		t.Error("smoke must end maintenance by the exact key returned by its begin call")
	}
	if !strings.Contains(script, `key = details.get("MaintenanceKey") if isinstance(details, dict) else None`) {
		t.Error("smoke must read the additive maintenance key without replacing instance details")
	}
	if strings.Contains(script, `/api/end-maintenance/mysql2/3306`) {
		t.Error("smoke must never end maintenance by instance")
	}
}

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
			if !strings.Contains(part, "#cluster_workspace") {
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

func workspaceCSSSelectors(css string) []string {
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

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
	"strings"
	"testing"
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

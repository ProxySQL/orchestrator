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
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRenderHTML_YieldInjectsInnerContent guards against regressions in the
// layout/inner template composition: layout.tmpl uses {{yield}} to inject the
// inner template's rendered output, and dropping the yield FuncMap (as
// happened when this fork rewrote rendering to use stdlib html/template)
// causes every /web/* page to 500.
func TestRenderHTML_YieldInjectsInnerContent(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	layout := `<!doctype html><html><body><header>CHROME</header>{{yield}}<footer>FOOT</footer></body></html>`
	inner := `<main>HELLO {{.Name}}</main>`
	if err := os.WriteFile(filepath.Join(dir, "templates", "layout.tmpl"), []byte(layout), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "templates", "page.tmpl"), []byte(inner), 0o644); err != nil {
		t.Fatal(err)
	}

	orig := templateDir
	templateDir = dir
	t.Cleanup(func() {
		templateDir = orig
		innerTemplateCache.Lock()
		innerTemplateCache.m = map[string]*template.Template{}
		innerTemplateCache.Unlock()
	})

	rec := httptest.NewRecorder()
	renderHTML(rec, 200, "templates/page", map[string]string{"Name": "world"})

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"CHROME", "HELLO world", "FOOT"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q\nbody: %s", want, body)
		}
	}
}

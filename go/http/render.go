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
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/proxysql/golib/log"
)

// assetVersion is appended as ?v= on static asset URLs in templates so
// browsers always pick up fresh JS/CSS after a server restart.
var assetVersion = computeAssetVersion()

func computeAssetVersion() string {
	if exe, err := os.Executable(); err == nil {
		if info, err := os.Stat(exe); err == nil {
			return fmt.Sprintf("%d", info.ModTime().UnixNano())
		}
	}
	return fmt.Sprintf("%d", os.Getpid())
}

// renderJSON writes a JSON response with the given status code.
func renderJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		_ = log.Errore(err)
	}
}

// contentTemplateCache caches parsed content templates (without layout).
var contentTemplateCache = struct {
	sync.RWMutex
	m map[string]*template.Template
}{m: make(map[string]*template.Template)}

var (
	layoutOnce    sync.Once
	layoutSource  string
	layoutLoadErr error
)

const templateDir = "resources"
const layoutFile = "templates/layout"

func loadLayoutSource() {
	layoutPath := filepath.Join(templateDir, layoutFile+".tmpl")
	b, err := os.ReadFile(layoutPath)
	if err != nil {
		layoutLoadErr = err
		return
	}
	layoutSource = string(b)
}

// getContentTemplate returns a cached content template or parses and caches it.
func getContentTemplate(name string) (*template.Template, error) {
	contentTemplateCache.RLock()
	if t, ok := contentTemplateCache.m[name]; ok {
		contentTemplateCache.RUnlock()
		return t, nil
	}
	contentTemplateCache.RUnlock()

	tmplPath := filepath.Join(templateDir, name+".tmpl")
	t, err := template.ParseFiles(tmplPath)
	if err != nil {
		return nil, err
	}

	contentTemplateCache.Lock()
	contentTemplateCache.m[name] = t
	contentTemplateCache.Unlock()
	return t, nil
}

// renderHTML renders an HTML template with the given data.
// The template name should be like "templates/clusters".
// Content is injected into layout.tmpl via {{yield}}, matching the
// martini-contrib/render convention used by the existing templates.
func renderHTML(w http.ResponseWriter, status int, name string, data interface{}) {
	content, err := getContentTemplate(name)
	if err != nil {
		_ = log.Errorf("Error parsing template %s: %+v", name, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var contentBuf bytes.Buffer
	if err := content.Execute(&contentBuf, data); err != nil {
		_ = log.Errorf("Error executing template %s: %+v", name, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	layoutOnce.Do(loadLayoutSource)
	if layoutLoadErr != nil {
		_ = log.Errorf("Error loading layout template: %+v", layoutLoadErr)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Per-request layout parse so the yield closure is concurrent-safe.
	layout, err := template.New("layout").Funcs(template.FuncMap{
		"yield": func() template.HTML {
			return template.HTML(contentBuf.String())
		},
	}).Parse(layoutSource)
	if err != nil {
		_ = log.Errorf("Error parsing layout template for %s: %+v", name, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Inject assetVersion so layout.tmpl can append ?v= on static asset URLs
	// for cache-busting after server upgrades.
	if m, ok := data.(map[string]interface{}); ok && m != nil {
		if _, present := m["assetVersion"]; !present {
			m["assetVersion"] = assetVersion
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(status)
	if err := layout.Execute(w, data); err != nil {
		_ = log.Errorf("Error executing layout for template %s: %+v", name, err)
	}
}

// renderRedirect sends an HTTP redirect.
func renderRedirect(w http.ResponseWriter, r *http.Request, url string) {
	http.Redirect(w, r, url, http.StatusFound)
}

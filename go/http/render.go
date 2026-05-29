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
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/proxysql/golib/log"
)

// assetVersion is computed once at process start and appended as a query
// parameter on static asset URLs in templates. Browsers cache assets by full
// URL, so bumping this string on every server restart guarantees clients pick
// up fresh JS/CSS without needing a hard reload.
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

// templateCache caches parsed templates.
var templateCache = struct {
	sync.RWMutex
	m map[string]*template.Template
}{m: make(map[string]*template.Template)}

const templateDir = "resources"
const layoutFile = "templates/layout"
const contentTemplateName = "content"

// getTemplate returns a cached template or parses and caches it.
func getTemplate(name string) (*template.Template, error) {
	templateCache.RLock()
	if t, ok := templateCache.m[name]; ok {
		templateCache.RUnlock()
		return t, nil
	}
	templateCache.RUnlock()

	layoutPath := filepath.Join(templateDir, layoutFile+".tmpl")
	tmplPath := filepath.Join(templateDir, name+".tmpl")

	t, err := template.ParseFiles(layoutPath)
	if err != nil {
		return nil, err
	}

	tmplBytes, err := os.ReadFile(tmplPath)
	if err != nil {
		return nil, err
	}

	if _, err := t.New(contentTemplateName).Parse(string(tmplBytes)); err != nil {
		return nil, err
	}

	templateCache.Lock()
	templateCache.m[name] = t
	templateCache.Unlock()
	return t, nil
}

// renderHTML renders an HTML template with the given data.
// The template name should be like "templates/clusters".
func renderHTML(w http.ResponseWriter, status int, name string, data interface{}) {
	t, err := getTemplate(name)
	if err != nil {
		_ = log.Errorf("Error parsing template %s: %+v", name, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	// Auto-inject assetVersion so layout.tmpl can cache-bust static asset URLs
	// without each call site having to remember to pass it. Guard against a
	// typed-nil map — `m[key] = v` would panic — even though no current caller
	// passes one.
	if m, ok := data.(map[string]interface{}); ok && m != nil {
		if _, present := m["assetVersion"]; !present {
			m["assetVersion"] = assetVersion
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(status)
	if err := t.Execute(w, data); err != nil {
		_ = log.Errorf("Error executing template %s: %+v", name, err)
	}
}

// renderRedirect sends an HTTP redirect.
func renderRedirect(w http.ResponseWriter, r *http.Request, url string) {
	http.Redirect(w, r, url, http.StatusFound)
}

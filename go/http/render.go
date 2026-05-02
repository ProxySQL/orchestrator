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
	"html/template"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/proxysql/golib/log"
)

// renderJSON writes a JSON response with the given status code.
func renderJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		_ = log.Errore(err)
	}
}

// templateDir is a var (not a const) so tests can point it at a temp dir.
var templateDir = "resources"

const layoutFile = "templates/layout"

// innerTemplateCache caches parsed inner (non-layout) templates. Inner
// templates have no per-request state so caching is safe. The layout, by
// contrast, is re-parsed per request because its FuncMap closes over a
// per-request buffer (see renderHTML).
var innerTemplateCache = struct {
	sync.RWMutex
	m map[string]*template.Template
}{m: make(map[string]*template.Template)}

// getInnerTemplate returns a cached inner template or parses and caches it.
func getInnerTemplate(name string) (*template.Template, error) {
	innerTemplateCache.RLock()
	if t, ok := innerTemplateCache.m[name]; ok {
		innerTemplateCache.RUnlock()
		return t, nil
	}
	innerTemplateCache.RUnlock()

	tmplPath := filepath.Join(templateDir, name+".tmpl")
	t, err := template.ParseFiles(tmplPath)
	if err != nil {
		return nil, err
	}

	innerTemplateCache.Lock()
	innerTemplateCache.m[name] = t
	innerTemplateCache.Unlock()
	return t, nil
}

// renderHTML renders an HTML template within the layout. The template name
// should be like "templates/clusters". The layout
// (resources/templates/layout.tmpl) uses {{yield}} to inject the inner
// template's rendered output, supplied by the FuncMap below.
func renderHTML(w http.ResponseWriter, status int, name string, data interface{}) {
	inner, err := getInnerTemplate(name)
	if err != nil {
		_ = log.Errorf("Error parsing template %s: %+v", name, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var innerBuf bytes.Buffer
	if err := inner.Execute(&innerBuf, data); err != nil {
		_ = log.Errorf("Error executing inner template %s: %+v", name, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	layoutPath := filepath.Join(templateDir, layoutFile+".tmpl")
	// The FuncMap must be registered on a template whose name matches the one
	// ParseFiles assigns (the file's basename), otherwise Execute fails with
	// "template ... is undefined".
	layout, err := template.New(filepath.Base(layoutPath)).Funcs(template.FuncMap{
		"yield": func() template.HTML { return template.HTML(innerBuf.String()) },
	}).ParseFiles(layoutPath)
	if err != nil {
		_ = log.Errorf("Error parsing layout for %s: %+v", name, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(status)
	if err := layout.Execute(w, data); err != nil {
		_ = log.Errorf("Error executing layout for %s: %+v", name, err)
	}
}

// renderRedirect sends an HTTP redirect.
func renderRedirect(w http.ResponseWriter, r *http.Request, url string) {
	http.Redirect(w, r, url, http.StatusFound)
}

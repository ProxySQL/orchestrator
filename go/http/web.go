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
	"expvar"
	"fmt"
	"net/http"
	"net/http/pprof"
	"strconv"
	"text/template"

	"github.com/go-chi/chi/v5"
	"github.com/rcrowley/go-metrics"
	"github.com/rcrowley/go-metrics/exp"

	"github.com/proxysql/orchestrator/go/config"
	"github.com/proxysql/orchestrator/go/inst"
)

// HttpWeb is the web requests server, mapping each request to a web page
type HttpWeb struct {
	URLPrefix string
}

var Web HttpWeb = HttpWeb{}

func (this *HttpWeb) getInstanceKey(host string, port string) (inst.InstanceKey, error) {
	instanceKey := inst.InstanceKey{Hostname: host}
	var err error

	if instanceKey.Port, err = strconv.Atoi(port); err != nil {
		return instanceKey, fmt.Errorf("Invalid port: %s", port)
	}
	return instanceKey, err
}

func (this *HttpWeb) AccessToken(w http.ResponseWriter, r *http.Request) {
	publicToken := template.JSEscapeString(r.URL.Query().Get("publicToken"))
	err := authenticateToken(publicToken, w)
	if err != nil {
		renderJSON(w, 200, &APIResponse{Code: ERROR, Message: fmt.Sprintf("%+v", err)})
		return
	}
	renderRedirect(w, r, this.URLPrefix+"/")
}

func (this *HttpWeb) Index(w http.ResponseWriter, r *http.Request) {
	renderRedirect(w, r, this.URLPrefix+"/web/clusters")
}

func (this *HttpWeb) Clusters(w http.ResponseWriter, r *http.Request) {
	renderHTML(w, 200, "templates/clusters", map[string]interface{}{
		"agentsHttpActive":              config.Config.ServeAgentsHttp,
		"title":                         "clusters",
		"autoshow_problems":             false,
		"authorizedForAction":           isAuthorizedForAction(r),
		"userId":                        getUserId(r),
		"removeTextFromHostnameDisplay": config.Config.RemoveTextFromHostnameDisplay,
		"prefix":                        this.URLPrefix,
		"webMessage":                    config.Config.WebMessage,
	})
}

func (this *HttpWeb) ClustersAnalysis(w http.ResponseWriter, r *http.Request) {
	renderHTML(w, 200, "templates/clusters_analysis", map[string]interface{}{
		"agentsHttpActive":              config.Config.ServeAgentsHttp,
		"title":                         "clusters",
		"autoshow_problems":             false,
		"authorizedForAction":           isAuthorizedForAction(r),
		"userId":                        getUserId(r),
		"removeTextFromHostnameDisplay": config.Config.RemoveTextFromHostnameDisplay,
		"prefix":                        this.URLPrefix,
		"webMessage":                    config.Config.WebMessage,
	})
}

func (this *HttpWeb) Cluster(w http.ResponseWriter, r *http.Request) {
	clusterName, _ := figureClusterName(chi.URLParam(r, "clusterName"))

	renderHTML(w, 200, "templates/cluster", map[string]interface{}{
		"agentsHttpActive":              config.Config.ServeAgentsHttp,
		"title":                         "cluster",
		"clusterName":                   clusterName,
		"autoshow_problems":             true,
		"contextMenuVisible":            true,
		"pseudoGTIDModeEnabled":         (config.Config.PseudoGTIDPattern != ""),
		"authorizedForAction":           isAuthorizedForAction(r),
		"userId":                        getUserId(r),
		"removeTextFromHostnameDisplay": config.Config.RemoveTextFromHostnameDisplay,
		"compactDisplay":                template.JSEscapeString(r.URL.Query().Get("compact")),
		"prefix":                        this.URLPrefix,
		"webMessage":                    config.Config.WebMessage,
	})
}

func (this *HttpWeb) ClusterByAlias(w http.ResponseWriter, r *http.Request) {
	clusterName, err := inst.GetClusterByAlias(chi.URLParam(r, "clusterAlias"))
	// Willing to accept the case of multiple clusters; we just present one
	if clusterName == "" && err != nil {
		renderJSON(w, 200, &APIResponse{Code: ERROR, Message: fmt.Sprintf("%+v", err)})
		return
	}

	this.clusterWithName(w, r, clusterName)
}

func (this *HttpWeb) ClusterByInstance(w http.ResponseWriter, r *http.Request) {
	instanceKey, err := this.getInstanceKey(chi.URLParam(r, "host"), chi.URLParam(r, "port"))
	if err != nil {
		renderJSON(w, 200, &APIResponse{Code: ERROR, Message: err.Error()})
		return
	}
	instance, found, err := inst.ReadInstance(&instanceKey)
	if (!found) || (err != nil) {
		renderJSON(w, 200, &APIResponse{Code: ERROR, Message: fmt.Sprintf("Cannot read instance: %+v", instanceKey)})
		return
	}

	// Willing to accept the case of multiple clusters; we just present one
	if instance.ClusterName == "" && err != nil {
		renderJSON(w, 200, &APIResponse{Code: ERROR, Message: fmt.Sprintf("%+v", err)})
		return
	}

	this.clusterWithName(w, r, instance.ClusterName)
}

// clusterWithName renders the cluster page for a given clusterName (shared by ClusterByAlias and ClusterByInstance).
func (this *HttpWeb) clusterWithName(w http.ResponseWriter, r *http.Request, clusterName string) {
	renderHTML(w, 200, "templates/cluster", map[string]interface{}{
		"agentsHttpActive":              config.Config.ServeAgentsHttp,
		"title":                         "cluster",
		"clusterName":                   clusterName,
		"autoshow_problems":             true,
		"contextMenuVisible":            true,
		"pseudoGTIDModeEnabled":         (config.Config.PseudoGTIDPattern != ""),
		"authorizedForAction":           isAuthorizedForAction(r),
		"userId":                        getUserId(r),
		"removeTextFromHostnameDisplay": config.Config.RemoveTextFromHostnameDisplay,
		"compactDisplay":                template.JSEscapeString(r.URL.Query().Get("compact")),
		"prefix":                        this.URLPrefix,
		"webMessage":                    config.Config.WebMessage,
	})
}

func (this *HttpWeb) ClusterPools(w http.ResponseWriter, r *http.Request) {
	clusterName, _ := figureClusterName(chi.URLParam(r, "clusterName"))
	renderHTML(w, 200, "templates/cluster_pools", map[string]interface{}{
		"agentsHttpActive":              config.Config.ServeAgentsHttp,
		"title":                         "cluster pools",
		"clusterName":                   clusterName,
		"autoshow_problems":             false, // because pool screen by default expands all hosts
		"contextMenuVisible":            true,
		"pseudoGTIDModeEnabled":         (config.Config.PseudoGTIDPattern != ""),
		"authorizedForAction":           isAuthorizedForAction(r),
		"userId":                        getUserId(r),
		"removeTextFromHostnameDisplay": config.Config.RemoveTextFromHostnameDisplay,
		"compactDisplay":                template.JSEscapeString(r.URL.Query().Get("compact")),
		"prefix":                        this.URLPrefix,
		"webMessage":                    config.Config.WebMessage,
	})
}

func (this *HttpWeb) Search(w http.ResponseWriter, r *http.Request) {
	searchString := chi.URLParam(r, "searchString")
	if searchString == "" {
		searchString = r.URL.Query().Get("s")
	}
	searchString = template.JSEscapeString(searchString)
	renderHTML(w, 200, "templates/search", map[string]interface{}{
		"agentsHttpActive":    config.Config.ServeAgentsHttp,
		"title":               "search",
		"searchString":        searchString,
		"authorizedForAction": isAuthorizedForAction(r),
		"userId":              getUserId(r),
		"autoshow_problems":   false,
		"prefix":              this.URLPrefix,
		"webMessage":          config.Config.WebMessage,
	})
}

func (this *HttpWeb) Discover(w http.ResponseWriter, r *http.Request) {

	renderHTML(w, 200, "templates/discover", map[string]interface{}{
		"agentsHttpActive":    config.Config.ServeAgentsHttp,
		"title":               "discover",
		"authorizedForAction": isAuthorizedForAction(r),
		"userId":              getUserId(r),
		"autoshow_problems":   false,
		"prefix":              this.URLPrefix,
		"webMessage":          config.Config.WebMessage,
	})
}

func (this *HttpWeb) Audit(w http.ResponseWriter, r *http.Request) {
	page, err := strconv.Atoi(chi.URLParam(r, "page"))
	if err != nil {
		page = 0
	}

	renderHTML(w, 200, "templates/audit", map[string]interface{}{
		"agentsHttpActive":    config.Config.ServeAgentsHttp,
		"title":               "audit",
		"authorizedForAction": isAuthorizedForAction(r),
		"userId":              getUserId(r),
		"autoshow_problems":   false,
		"page":                page,
		"auditHostname":       chi.URLParam(r, "host"),
		"auditPort":           chi.URLParam(r, "port"),
		"prefix":              this.URLPrefix,
		"webMessage":          config.Config.WebMessage,
	})
}

func (this *HttpWeb) AuditRecovery(w http.ResponseWriter, r *http.Request) {
	page, err := strconv.Atoi(chi.URLParam(r, "page"))
	if err != nil {
		page = 0
	}
	recoveryId, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 0)
	if err != nil {
		recoveryId = 0
	}
	recoveryUid := chi.URLParam(r, "uid")
	clusterAlias := chi.URLParam(r, "clusterAlias")

	clusterName, _ := figureClusterName(chi.URLParam(r, "clusterName"))
	renderHTML(w, 200, "templates/audit_recovery", map[string]interface{}{
		"agentsHttpActive":    config.Config.ServeAgentsHttp,
		"title":               "audit-recovery",
		"authorizedForAction": isAuthorizedForAction(r),
		"userId":              getUserId(r),
		"autoshow_problems":   false,
		"page":                page,
		"clusterName":         clusterName,
		"clusterAlias":        clusterAlias,
		"recoveryId":          recoveryId,
		"recoveryUid":         recoveryUid,
		"prefix":              this.URLPrefix,
		"webMessage":          config.Config.WebMessage,
	})
}

func (this *HttpWeb) AuditFailureDetection(w http.ResponseWriter, r *http.Request) {
	page, err := strconv.Atoi(chi.URLParam(r, "page"))
	if err != nil {
		page = 0
	}
	detectionId, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 0)
	if err != nil {
		detectionId = 0
	}
	clusterAlias := chi.URLParam(r, "clusterAlias")

	renderHTML(w, 200, "templates/audit_failure_detection", map[string]interface{}{
		"agentsHttpActive":    config.Config.ServeAgentsHttp,
		"title":               "audit-failure-detection",
		"authorizedForAction": isAuthorizedForAction(r),
		"userId":              getUserId(r),
		"autoshow_problems":   false,
		"page":                page,
		"detectionId":         detectionId,
		"clusterAlias":        clusterAlias,
		"prefix":              this.URLPrefix,
		"webMessage":          config.Config.WebMessage,
	})
}

func (this *HttpWeb) Agents(w http.ResponseWriter, r *http.Request) {
	renderHTML(w, 200, "templates/agents", map[string]interface{}{
		"agentsHttpActive":    config.Config.ServeAgentsHttp,
		"title":               "agents",
		"authorizedForAction": isAuthorizedForAction(r),
		"userId":              getUserId(r),
		"autoshow_problems":   false,
		"prefix":              this.URLPrefix,
		"webMessage":          config.Config.WebMessage,
	})
}

func (this *HttpWeb) Agent(w http.ResponseWriter, r *http.Request) {
	renderHTML(w, 200, "templates/agent", map[string]interface{}{
		"agentsHttpActive":    config.Config.ServeAgentsHttp,
		"title":               "agent",
		"authorizedForAction": isAuthorizedForAction(r),
		"userId":              getUserId(r),
		"autoshow_problems":   false,
		"agentHost":           chi.URLParam(r, "host"),
		"prefix":              this.URLPrefix,
		"webMessage":          config.Config.WebMessage,
	})
}

func (this *HttpWeb) AgentSeedDetails(w http.ResponseWriter, r *http.Request) {
	renderHTML(w, 200, "templates/agent_seed_details", map[string]interface{}{
		"agentsHttpActive":    config.Config.ServeAgentsHttp,
		"title":               "agent seed details",
		"authorizedForAction": isAuthorizedForAction(r),
		"userId":              getUserId(r),
		"autoshow_problems":   false,
		"seedId":              chi.URLParam(r, "seedId"),
		"prefix":              this.URLPrefix,
		"webMessage":          config.Config.WebMessage,
	})
}

func (this *HttpWeb) Seeds(w http.ResponseWriter, r *http.Request) {
	renderHTML(w, 200, "templates/seeds", map[string]interface{}{
		"agentsHttpActive":    config.Config.ServeAgentsHttp,
		"title":               "seeds",
		"authorizedForAction": isAuthorizedForAction(r),
		"userId":              getUserId(r),
		"autoshow_problems":   false,
		"prefix":              this.URLPrefix,
		"webMessage":          config.Config.WebMessage,
	})
}

func (this *HttpWeb) Home(w http.ResponseWriter, r *http.Request) {

	renderHTML(w, 200, "templates/home", map[string]interface{}{
		"agentsHttpActive":    config.Config.ServeAgentsHttp,
		"title":               "home",
		"authorizedForAction": isAuthorizedForAction(r),
		"userId":              getUserId(r),
		"autoshow_problems":   false,
		"prefix":              this.URLPrefix,
		"webMessage":          config.Config.WebMessage,
	})
}

func (this *HttpWeb) About(w http.ResponseWriter, r *http.Request) {

	renderHTML(w, 200, "templates/about", map[string]interface{}{
		"agentsHttpActive":    config.Config.ServeAgentsHttp,
		"title":               "about",
		"authorizedForAction": isAuthorizedForAction(r),
		"userId":              getUserId(r),
		"autoshow_problems":   false,
		"prefix":              this.URLPrefix,
		"webMessage":          config.Config.WebMessage,
	})
}

func (this *HttpWeb) KeepCalm(w http.ResponseWriter, r *http.Request) {

	renderHTML(w, 200, "templates/keep-calm", map[string]interface{}{
		"agentsHttpActive":    config.Config.ServeAgentsHttp,
		"title":               "Keep Calm",
		"authorizedForAction": isAuthorizedForAction(r),
		"userId":              getUserId(r),
		"autoshow_problems":   false,
		"prefix":              this.URLPrefix,
		"webMessage":          config.Config.WebMessage,
	})
}

func (this *HttpWeb) FAQ(w http.ResponseWriter, r *http.Request) {

	renderHTML(w, 200, "templates/faq", map[string]interface{}{
		"agentsHttpActive":    config.Config.ServeAgentsHttp,
		"title":               "FAQ",
		"authorizedForAction": isAuthorizedForAction(r),
		"userId":              getUserId(r),
		"autoshow_problems":   false,
		"prefix":              this.URLPrefix,
		"webMessage":          config.Config.WebMessage,
	})
}

func (this *HttpWeb) Status(w http.ResponseWriter, r *http.Request) {

	renderHTML(w, 200, "templates/status", map[string]interface{}{
		"agentsHttpActive":    config.Config.ServeAgentsHttp,
		"title":               "status",
		"authorizedForAction": isAuthorizedForAction(r),
		"userId":              getUserId(r),
		"autoshow_problems":   false,
		"prefix":              this.URLPrefix,
		"webMessage":          config.Config.WebMessage,
	})
}

func (this *HttpWeb) registerWebRequest(router chi.Router, path string, handler http.HandlerFunc) {
	fullPath := fmt.Sprintf("%s/web/%s", this.URLPrefix, path)
	if path == "/" {
		fullPath = fmt.Sprintf("%s/", this.URLPrefix)
	}

	if config.Config.RaftEnabled {
		router.With(raftReverseProxyMiddleware).Get(fullPath, handler)
	} else {
		router.Get(fullPath, handler)
	}
}

// RegisterRequests makes for the de-facto list of known Web calls
func (this *HttpWeb) RegisterRequests(router chi.Router) {
	this.registerWebRequest(router, "access-token", this.AccessToken)
	this.registerWebRequest(router, "", this.Index)
	this.registerWebRequest(router, "/", this.Index)
	this.registerWebRequest(router, "home", this.About)
	this.registerWebRequest(router, "about", this.About)
	this.registerWebRequest(router, "keep-calm", this.KeepCalm)
	this.registerWebRequest(router, "faq", this.FAQ)
	this.registerWebRequest(router, "status", this.Status)
	this.registerWebRequest(router, "clusters", this.Clusters)
	this.registerWebRequest(router, "clusters-analysis", this.ClustersAnalysis)
	this.registerWebRequest(router, "cluster/{clusterName}", this.Cluster)
	this.registerWebRequest(router, "cluster/alias/{clusterAlias}", this.ClusterByAlias)
	this.registerWebRequest(router, "cluster/instance/{host}/{port}", this.ClusterByInstance)
	this.registerWebRequest(router, "cluster-pools/{clusterName}", this.ClusterPools)
	this.registerWebRequest(router, "search/{searchString}", this.Search)
	this.registerWebRequest(router, "search", this.Search)
	this.registerWebRequest(router, "discover", this.Discover)
	this.registerWebRequest(router, "audit", this.Audit)
	this.registerWebRequest(router, "audit/{page}", this.Audit)
	this.registerWebRequest(router, "audit/instance/{host}/{port}", this.Audit)
	this.registerWebRequest(router, "audit/instance/{host}/{port}/{page}", this.Audit)
	this.registerWebRequest(router, "audit-recovery", this.AuditRecovery)
	this.registerWebRequest(router, "audit-recovery/{page}", this.AuditRecovery)
	this.registerWebRequest(router, "audit-recovery/id/{id}", this.AuditRecovery)
	this.registerWebRequest(router, "audit-recovery/uid/{uid}", this.AuditRecovery)
	this.registerWebRequest(router, "audit-recovery/cluster/{clusterName}", this.AuditRecovery)
	this.registerWebRequest(router, "audit-recovery/cluster/{clusterName}/{page}", this.AuditRecovery)
	this.registerWebRequest(router, "audit-recovery/alias/{clusterAlias}", this.AuditRecovery)
	this.registerWebRequest(router, "audit-recovery/alias/{clusterAlias}/{page}", this.AuditRecovery)
	this.registerWebRequest(router, "audit-failure-detection", this.AuditFailureDetection)
	this.registerWebRequest(router, "audit-failure-detection/{page}", this.AuditFailureDetection)
	this.registerWebRequest(router, "audit-failure-detection/id/{id}", this.AuditFailureDetection)
	this.registerWebRequest(router, "audit-failure-detection/alias/{clusterAlias}", this.AuditFailureDetection)
	this.registerWebRequest(router, "audit-failure-detection/alias/{clusterAlias}/{page}", this.AuditFailureDetection)
	this.registerWebRequest(router, "audit-recovery-steps/{uid}", this.AuditRecovery)
	this.registerWebRequest(router, "agents", this.Agents)
	this.registerWebRequest(router, "agent/{host}", this.Agent)
	this.registerWebRequest(router, "seed-details/{seedId}", this.AgentSeedDetails)
	this.registerWebRequest(router, "seeds", this.Seeds)

	this.RegisterDebug(router)
}

// RegisterDebug adds handlers for /debug/vars (expvar) and /debug/pprof (net/http/pprof) support
func (this *HttpWeb) RegisterDebug(router chi.Router) {
	router.Get(this.URLPrefix+"/debug/vars", func(w http.ResponseWriter, r *http.Request) {
		// from expvar.go, since the expvarHandler isn't exported :(
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = fmt.Fprintf(w, "{\n")
		first := true
		expvar.Do(func(kv expvar.KeyValue) {
			if !first {
				_, _ = fmt.Fprintf(w, ",\n")
			}
			first = false
			_, _ = fmt.Fprintf(w, "%q: %s", kv.Key, kv.Value)
		})
		_, _ = fmt.Fprintf(w, "\n}\n")
	})

	// list all the /debug/ endpoints we want
	router.Get(this.URLPrefix+"/debug/pprof", http.HandlerFunc(pprof.Index))
	router.Get(this.URLPrefix+"/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	router.Get(this.URLPrefix+"/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	router.Get(this.URLPrefix+"/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	router.Post(this.URLPrefix+"/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	router.Get(this.URLPrefix+"/debug/pprof/block", pprof.Handler("block").ServeHTTP)
	router.Get(this.URLPrefix+"/debug/pprof/heap", pprof.Handler("heap").ServeHTTP)
	router.Get(this.URLPrefix+"/debug/pprof/goroutine", pprof.Handler("goroutine").ServeHTTP)
	router.Get(this.URLPrefix+"/debug/pprof/threadcreate", pprof.Handler("threadcreate").ServeHTTP)

	// go-metrics
	router.Get(this.URLPrefix+"/debug/metrics", exp.ExpHandler(metrics.DefaultRegistry).(http.HandlerFunc))
}

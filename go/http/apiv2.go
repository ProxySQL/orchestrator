/*
   Copyright 2026 Percona LLC

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
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/proxysql/orchestrator/go/inst"
	"github.com/proxysql/orchestrator/go/logic"
	"github.com/proxysql/orchestrator/go/process"
	"github.com/proxysql/orchestrator/go/proxysql"
)

// V2APIResponse is the standard v2 response envelope
type V2APIResponse struct {
	Status  string      `json:"status"`           // "ok" or "error"
	Data    interface{} `json:"data,omitempty"`    // response payload
	Error   *V2APIError `json:"error,omitempty"`   // error details
	Message string      `json:"message,omitempty"` // human-readable message
}

// V2APIError contains machine-readable error details
type V2APIError struct {
	Code    string `json:"code"`    // machine-readable error code
	Message string `json:"message"` // human-readable error description
}

// respondOK writes a 200 JSON response with the standard v2 envelope.
func respondOK(w http.ResponseWriter, data interface{}) {
	writeV2JSON(w, http.StatusOK, V2APIResponse{
		Status: "ok",
		Data:   data,
	})
}

// respondOKWithMessage writes a 200 JSON response with data and a message.
func respondOKWithMessage(w http.ResponseWriter, data interface{}, message string) {
	writeV2JSON(w, http.StatusOK, V2APIResponse{
		Status:  "ok",
		Data:    data,
		Message: message,
	})
}

// respondError writes an error JSON response with the given HTTP status code.
func respondError(w http.ResponseWriter, status int, code, message string) {
	writeV2JSON(w, status, V2APIResponse{
		Status: "error",
		Error: &V2APIError{
			Code:    code,
			Message: message,
		},
	})
}

// respondNotFound writes a 404 JSON response.
func respondNotFound(w http.ResponseWriter, message string) {
	respondError(w, http.StatusNotFound, "NOT_FOUND", message)
}

// writeV2JSON marshals the response and writes it with the given status code.
func writeV2JSON(w http.ResponseWriter, status int, resp V2APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

// RegisterV2Routes returns a chi.Router with all v2 API routes.
func RegisterV2Routes() chi.Router {
	r := chi.NewRouter()

	// Cluster endpoints
	r.Get("/clusters", V2Clusters)
	r.Get("/clusters/{alias}", V2ClusterInfo)
	r.Get("/clusters/{alias}/instances", V2ClusterInstances)

	// Instance endpoints
	r.Get("/instances/{host}/{port}", V2Instance)

	// Topology endpoints
	r.Get("/topology/{clusterAlias}", V2Topology)

	// Recovery endpoints
	r.Get("/recoveries", V2Recoveries)
	r.Get("/recoveries/active", V2ActiveRecoveries)

	// Health/status
	r.Get("/status", V2Status)

	// ProxySQL
	r.Get("/proxysql/servers", V2ProxySQLServers)

	return r
}

// V2Clusters returns a list of all known clusters with metadata.
func V2Clusters(w http.ResponseWriter, r *http.Request) {
	clustersInfo, err := inst.ReadClustersInfo("")
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR",
			fmt.Sprintf("Failed to read clusters: %v", err))
		return
	}
	respondOK(w, clustersInfo)
}

// V2ClusterInfo returns detailed info for one cluster, looked up by alias.
func V2ClusterInfo(w http.ResponseWriter, r *http.Request) {
	alias := chi.URLParam(r, "alias")
	clusterName, err := inst.GetClusterByAlias(alias)
	if err != nil {
		// Try treating the alias as a cluster name directly
		clusterName = alias
	}
	clusterInfo, err := inst.ReadClusterInfo(clusterName)
	if err != nil {
		respondNotFound(w, fmt.Sprintf("Cluster not found: %s", alias))
		return
	}
	respondOK(w, clusterInfo)
}

// V2ClusterInstances returns all instances belonging to a cluster.
func V2ClusterInstances(w http.ResponseWriter, r *http.Request) {
	alias := chi.URLParam(r, "alias")
	clusterName, err := inst.GetClusterByAlias(alias)
	if err != nil {
		clusterName = alias
	}
	instances, err := inst.ReadClusterInstances(clusterName)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR",
			fmt.Sprintf("Failed to read cluster instances: %v", err))
		return
	}
	if len(instances) == 0 {
		respondNotFound(w, fmt.Sprintf("No instances found for cluster: %s", alias))
		return
	}
	respondOK(w, instances)
}

// V2Instance returns detailed information about a single instance.
func V2Instance(w http.ResponseWriter, r *http.Request) {
	host := chi.URLParam(r, "host")
	port := chi.URLParam(r, "port")

	instanceKey, err := inst.NewResolveInstanceKeyStrings(host, port)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAMS",
			fmt.Sprintf("Invalid instance key: %s:%s", host, port))
		return
	}
	instance, found, err := inst.ReadInstance(instanceKey)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR",
			fmt.Sprintf("Failed to read instance: %v", err))
		return
	}
	if !found {
		respondNotFound(w, fmt.Sprintf("Instance not found: %s:%s", host, port))
		return
	}
	respondOK(w, instance)
}

// V2Topology returns the topology (instances with replication relationships) for a cluster.
func V2Topology(w http.ResponseWriter, r *http.Request) {
	clusterAlias := chi.URLParam(r, "clusterAlias")
	clusterName, err := inst.GetClusterByAlias(clusterAlias)
	if err != nil {
		clusterName = clusterAlias
	}
	instances, err := inst.ReadClusterInstances(clusterName)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR",
			fmt.Sprintf("Failed to read topology: %v", err))
		return
	}
	if len(instances) == 0 {
		respondNotFound(w, fmt.Sprintf("No topology found for cluster: %s", clusterAlias))
		return
	}
	respondOK(w, instances)
}

// V2Recoveries returns recent recovery entries. Supports optional query parameters:
// ?cluster=<name>&page=<n>&unacknowledged=true
func V2Recoveries(w http.ResponseWriter, r *http.Request) {
	clusterName := r.URL.Query().Get("cluster")
	clusterAlias := r.URL.Query().Get("alias")
	unacknowledgedOnly := r.URL.Query().Get("unacknowledged") == "true"
	page := 0
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p >= 0 {
			page = p
		}
	}

	recoveries, err := logic.ReadRecentRecoveries(clusterName, clusterAlias, unacknowledgedOnly, page)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR",
			fmt.Sprintf("Failed to read recoveries: %v", err))
		return
	}
	respondOK(w, recoveries)
}

// V2ActiveRecoveries returns currently active (in-progress) recoveries.
func V2ActiveRecoveries(w http.ResponseWriter, r *http.Request) {
	recoveries, err := logic.ReadActiveRecoveries()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR",
			fmt.Sprintf("Failed to read active recoveries: %v", err))
		return
	}
	respondOK(w, recoveries)
}

// V2Status returns orchestrator health and status information.
func V2Status(w http.ResponseWriter, r *http.Request) {
	health, err := process.HealthTest()
	if err != nil {
		respondError(w, http.StatusServiceUnavailable, "UNHEALTHY",
			fmt.Sprintf("Application node is unhealthy: %v", err))
		return
	}
	respondOKWithMessage(w, health, "Application node is healthy")
}

// V2ProxySQLServers returns ProxySQL server entries.
func V2ProxySQLServers(w http.ResponseWriter, r *http.Request) {
	hook := proxysql.GetHook()
	if hook == nil || !hook.IsConfigured() {
		respondError(w, http.StatusNotFound, "NOT_CONFIGURED",
			"ProxySQL is not configured")
		return
	}
	servers, err := hook.GetClient().GetServers()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR",
			fmt.Sprintf("Failed to query ProxySQL servers: %v", err))
		return
	}
	respondOKWithMessage(w, servers, fmt.Sprintf("Found %d servers", len(servers)))
}

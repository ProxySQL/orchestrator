/*
   Copyright 2024 Orchestrator Authors

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
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/proxysql/orchestrator/go/config"
	"github.com/proxysql/orchestrator/go/inst"
	"github.com/proxysql/orchestrator/go/logic"
	"github.com/proxysql/orchestrator/go/process"
	"github.com/proxysql/orchestrator/go/proxysql"
)

// V2APIResponse is the standard response envelope for API v2 endpoints.
type V2APIResponse struct {
	Status  string      `json:"status"`
	Data    interface{} `json:"data,omitempty"`
	Error   *V2APIError `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
}

// V2APIError represents an error in the API v2 response.
type V2APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// respondOK writes a successful JSON response with the given data.
func respondOK(w http.ResponseWriter, data interface{}) {
	renderJSON(w, http.StatusOK, &V2APIResponse{
		Status: "ok",
		Data:   data,
	})
}

// respondError writes an error JSON response with the given HTTP status, error code, and message.
func respondError(w http.ResponseWriter, status int, code string, message string) {
	renderJSON(w, status, &V2APIResponse{
		Status: "error",
		Error: &V2APIError{
			Code:    code,
			Message: message,
		},
	})
}

// respondNotFound writes a 404 JSON response with the given message.
func respondNotFound(w http.ResponseWriter, message string) {
	respondError(w, http.StatusNotFound, "NOT_FOUND", message)
}

// RegisterV2Routes mounts all v2 API routes under /api/v2 on the given router.
func RegisterV2Routes(r chi.Router) {
	prefix := config.Config.URLPrefix
	r.Route(fmt.Sprintf("%s/api/v2", prefix), func(r chi.Router) {
		// Cluster endpoints
		r.Get("/clusters", V2Clusters)
		r.Get("/clusters/{name}", V2ClusterInfo)
		r.Get("/clusters/{name}/instances", V2ClusterInstances)
		r.Get("/clusters/{name}/topology", V2Topology)

		// Instance endpoints
		r.Get("/instances/{host}/{port}", V2Instance)
		r.Get("/instances/{host}/{port}/channels", V2InstanceChannels)

		// Recovery endpoints
		r.Get("/recoveries", V2Recoveries)
		r.Get("/recoveries/active", V2ActiveRecoveries)

		// Status endpoints
		r.Get("/status", V2Status)

		// ProxySQL endpoints
		r.Get("/proxysql/servers", V2ProxySQLServers)
	})
}

// V2Clusters returns a list of all known clusters with metadata.
func V2Clusters(w http.ResponseWriter, r *http.Request) {
	clustersInfo, err := inst.ReadClustersInfo("")
	if err != nil {
		respondError(w, http.StatusInternalServerError, "CLUSTER_LIST_ERROR", fmt.Sprintf("Failed to read clusters: %v", err))
		return
	}
	respondOK(w, clustersInfo)
}

// V2ClusterInfo returns detailed information about a specific cluster.
func V2ClusterInfo(w http.ResponseWriter, r *http.Request) {
	clusterName := chi.URLParam(r, "name")
	clusterName, err := figureClusterName(clusterName)
	if err != nil {
		respondNotFound(w, fmt.Sprintf("Cluster not found: %v", err))
		return
	}
	clusterInfo, err := inst.ReadClusterInfo(clusterName)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "CLUSTER_INFO_ERROR", fmt.Sprintf("Failed to read cluster info: %v", err))
		return
	}
	respondOK(w, clusterInfo)
}

// V2ClusterInstances returns all instances belonging to a given cluster.
func V2ClusterInstances(w http.ResponseWriter, r *http.Request) {
	clusterName := chi.URLParam(r, "name")
	clusterName, err := figureClusterName(clusterName)
	if err != nil {
		respondNotFound(w, fmt.Sprintf("Cluster not found: %v", err))
		return
	}
	instances, err := inst.ReadClusterInstances(clusterName)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "CLUSTER_INSTANCES_ERROR", fmt.Sprintf("Failed to read cluster instances: %v", err))
		return
	}
	respondOK(w, instances)
}

// V2Instance returns detailed information about a specific MySQL instance.
func V2Instance(w http.ResponseWriter, r *http.Request) {
	host := chi.URLParam(r, "host")
	port := chi.URLParam(r, "port")

	instanceKey, err := inst.NewResolveInstanceKeyStrings(host, port)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_INSTANCE", fmt.Sprintf("Invalid instance key: %v", err))
		return
	}
	instanceKey, err = inst.FigureInstanceKey(instanceKey, nil)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_INSTANCE", fmt.Sprintf("Cannot resolve instance: %v", err))
		return
	}
	instance, found, err := inst.ReadInstance(instanceKey)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INSTANCE_READ_ERROR", fmt.Sprintf("Failed to read instance: %v", err))
		return
	}
	if !found {
		respondNotFound(w, fmt.Sprintf("Instance not found: %s:%s", host, port))
		return
	}
	respondOK(w, instance)
}

// V2Topology returns the ASCII topology representation for a given cluster.
func V2Topology(w http.ResponseWriter, r *http.Request) {
	clusterName := chi.URLParam(r, "name")
	clusterName, err := figureClusterName(clusterName)
	if err != nil {
		respondNotFound(w, fmt.Sprintf("Cluster not found: %v", err))
		return
	}
	asciiOutput, err := inst.ASCIITopology(clusterName, "", false, false)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "TOPOLOGY_ERROR", fmt.Sprintf("Failed to generate topology: %v", err))
		return
	}
	respondOK(w, map[string]interface{}{
		"clusterName": clusterName,
		"topology":    asciiOutput,
	})
}

// V2Recoveries returns recent recovery entries, optionally filtered by cluster.
func V2Recoveries(w http.ResponseWriter, r *http.Request) {
	clusterName := r.URL.Query().Get("cluster")
	clusterAlias := r.URL.Query().Get("alias")
	page := 0
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil {
			page = p
		}
	}
	recoveries, err := logic.ReadRecentRecoveries(clusterName, clusterAlias, false, page)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "RECOVERIES_ERROR", fmt.Sprintf("Failed to read recoveries: %v", err))
		return
	}
	respondOK(w, recoveries)
}

// V2ActiveRecoveries returns currently active (in-progress) recoveries.
func V2ActiveRecoveries(w http.ResponseWriter, r *http.Request) {
	recoveries, err := logic.ReadActiveRecoveries()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "ACTIVE_RECOVERIES_ERROR", fmt.Sprintf("Failed to read active recoveries: %v", err))
		return
	}
	respondOK(w, recoveries)
}

// V2Status returns the health status of the orchestrator node.
func V2Status(w http.ResponseWriter, r *http.Request) {
	health, err := process.HealthTest()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "UNHEALTHY", fmt.Sprintf("Application node is unhealthy: %v", err))
		return
	}
	respondOK(w, health)
}

// V2InstanceChannels returns the replication channels for a specific MySQL instance.
// For multi-source replicas, this returns all named replication channels and their status.
func V2InstanceChannels(w http.ResponseWriter, r *http.Request) {
	host := chi.URLParam(r, "host")
	port := chi.URLParam(r, "port")

	instanceKey, err := inst.NewResolveInstanceKeyStrings(host, port)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_INSTANCE", fmt.Sprintf("Invalid instance key: %v", err))
		return
	}
	instanceKey, err = inst.FigureInstanceKey(instanceKey, nil)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_INSTANCE", fmt.Sprintf("Cannot resolve instance: %v", err))
		return
	}
	instance, found, err := inst.ReadInstance(instanceKey)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INSTANCE_READ_ERROR", fmt.Sprintf("Failed to read instance: %v", err))
		return
	}
	if !found {
		respondNotFound(w, fmt.Sprintf("Instance not found: %s:%s", host, port))
		return
	}
	respondOK(w, instance.ReplicationChannels)
}

// V2ProxySQLServers returns all servers from ProxySQL's runtime_mysql_servers table.
func V2ProxySQLServers(w http.ResponseWriter, r *http.Request) {
	hook := proxysql.GetHook()
	if hook == nil || !hook.IsConfigured() {
		respondError(w, http.StatusServiceUnavailable, "PROXYSQL_NOT_CONFIGURED", "ProxySQL is not configured")
		return
	}
	servers, err := hook.GetClient().GetServers()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "PROXYSQL_ERROR", fmt.Sprintf("Failed to query ProxySQL servers: %v", err))
		return
	}
	respondOK(w, servers)
}

/*
   Copyright 2024 Percona LLC

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
	"net/http"
	"sync/atomic"

	"github.com/proxysql/orchestrator/go/process"
	orcraft "github.com/proxysql/orchestrator/go/raft"
)

// HealthLive is a lightweight liveness probe. It always returns 200 if the
// process is running.
func HealthLive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
}

// HealthReady checks whether the backend DB is connected and the health check
// loop is running successfully. Returns 200 when ready, 503 otherwise.
func HealthReady(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	healthy := atomic.LoadInt64(&process.LastContinousCheckHealthy) == 1

	if healthy {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "not ready"})
	}
}

// HealthLeader returns 200 if this node is the active leader (raft leader or
// elected node), or if raft is disabled and the node is the active node.
// Returns 503 otherwise.
func HealthLeader(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	isLeader := false
	if orcraft.IsRaftEnabled() {
		isLeader = orcraft.IsLeader()
	} else {
		// When raft is not enabled, if health checks pass, treat the node as leader
		isLeader = atomic.LoadInt64(&process.LastContinousCheckHealthy) == 1
	}

	if isLeader {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "leader"})
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"status": "not leader"})
	}
}

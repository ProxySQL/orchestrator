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

package metrics

import (
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/proxysql/golib/log"
	"github.com/proxysql/orchestrator/go/config"
	"github.com/proxysql/orchestrator/go/process"
	orcraft "github.com/proxysql/orchestrator/go/raft"
)

// Prometheus metric variables, exported so other packages can update them.
var (
	DiscoveriesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "orchestrator_discoveries_total",
		Help: "Total number of discovery attempts.",
	})

	DiscoveryErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "orchestrator_discovery_errors_total",
		Help: "Total number of failed discoveries.",
	})

	InstancesTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "orchestrator_instances_total",
		Help: "Total number of known instances.",
	})

	ClustersTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "orchestrator_clusters_total",
		Help: "Total number of known clusters.",
	})

	RecoveriesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "orchestrator_recoveries_total",
		Help: "Total number of recovery attempts.",
	}, []string{"type", "result"})

	RecoveryDurationSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "orchestrator_recovery_duration_seconds",
		Help:    "Duration of recovery operations in seconds.",
		Buckets: prometheus.ExponentialBuckets(0.5, 2, 12), // 0.5s to ~1024s
	})

	HealthIsReady = prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "orchestrator_health_is_ready",
		Help: "1 if the node is ready, 0 otherwise.",
	}, func() float64 {
		if atomic.LoadInt64(&process.LastContinousCheckHealthy) == 1 {
			return 1
		}
		return 0
	})

	HealthIsLive = prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "orchestrator_health_is_live",
		Help: "1 if the node is live, 0 otherwise.",
	}, func() float64 {
		return 1
	})

	RaftIsHealthy = prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "orchestrator_raft_is_healthy",
		Help: "1 if the node is a healthy raft node, 0 otherwise.",
	}, func() float64 {
		if orcraft.IsRaftEnabled() && orcraft.IsHealthy() {
			return 1
		}
		return 0
	})

	RaftIsLeader = prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "orchestrator_raft_is_leader",
		Help: "1 if the node is the raft leader, 0 otherwise.",
	}, func() float64 {
		if orcraft.IsRaftEnabled() && orcraft.IsLeader() {
			return 1
		}
		return 0
	})
)

// InitPrometheus registers all Prometheus metrics. Should be called once at
// startup when PrometheusEnabled is true.
func InitPrometheus() {
	if !config.Config.PrometheusEnabled {
		return
	}
	log.Info("Initializing Prometheus metrics")
	prometheus.MustRegister(
		DiscoveriesTotal,
		DiscoveryErrorsTotal,
		InstancesTotal,
		ClustersTotal,
		RecoveriesTotal,
		RecoveryDurationSeconds,
		HealthIsReady,
		HealthIsLive,
		RaftIsHealthy,
		RaftIsLeader,
	)
}

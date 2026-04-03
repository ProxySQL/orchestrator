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

package inst

import (
	"fmt"

	"github.com/proxysql/golib/log"
	"github.com/proxysql/golib/sqlutils"
	"github.com/proxysql/orchestrator/go/db"
)

// GetPostgreSQLReplicationAnalysis analyzes PostgreSQL streaming replication
// topology for failures. It reads instance data from the orchestrator backend
// database (not from PostgreSQL directly) and produces analysis entries that
// the recovery logic can act upon.
//
// This mirrors the MySQL GetReplicationAnalysis but with PostgreSQL-appropriate
// failure scenarios (no intermediate masters, no co-masters, no binlog servers).
func GetPostgreSQLReplicationAnalysis(clusterName string, hints *ReplicationAnalysisHints) ([]ReplicationAnalysis, error) {
	result := []ReplicationAnalysis{}

	// Query the orchestrator backend for primary instances and their standbys.
	// A "primary" in PG is an instance with no master_host (or master_host='')
	// and replication_depth=0. Standbys have replication_depth=1.
	query := `
		SELECT
			master_instance.hostname,
			master_instance.port,
			master_instance.read_only,
			master_instance.last_checked <= master_instance.last_seen AS is_last_check_valid,
			master_instance.last_check_partial_success,
			master_instance.master_host,
			master_instance.master_port,
			master_instance.cluster_name,
			master_instance.replication_depth,
			master_instance.data_center,
			master_instance.region,
			master_instance.physical_environment,
			master_instance.binary_log_file,
			master_instance.binary_log_pos,
			0 AS is_downtimed,
			'' AS downtime_end_timestamp,
			COUNT(replica_instance.server_id) AS count_replicas,
			IFNULL(
				SUM(
					replica_instance.last_checked <= replica_instance.last_seen
				),
				0
			) AS count_valid_replicas,
			IFNULL(
				SUM(
					replica_instance.last_checked <= replica_instance.last_seen
					AND replica_instance.slave_io_running != 0
					AND replica_instance.slave_sql_running != 0
				),
				0
			) AS count_valid_replicating_replicas,
			IFNULL(
				SUM(
					replica_instance.last_checked <= replica_instance.last_seen
					AND replica_instance.slave_io_running = 0
					AND replica_instance.slave_sql_running = 1
				),
				0
			) AS count_replicas_failing_to_connect,
			IFNULL(
				SUM(0),
				0
			) AS count_downtimed_replicas,
			master_instance.slave_hosts
		FROM
			database_instance master_instance
			LEFT JOIN database_instance replica_instance ON (
				replica_instance.master_host = master_instance.hostname
				AND replica_instance.master_port = master_instance.port
			)
		WHERE
			? IN ('', master_instance.cluster_name)
			AND master_instance.replication_depth = 0
		GROUP BY
			master_instance.hostname,
			master_instance.port
	`
	args := sqlutils.Args(clusterName)

	err := db.QueryOrchestrator(query, args, func(m sqlutils.RowMap) error {
		a := ReplicationAnalysis{
			Analysis:   NoProblem,
			IsMaster:   true,
			IsCoMaster: false,
		}

		a.AnalyzedInstanceKey = InstanceKey{
			Hostname: m.GetString("hostname"),
			Port:     m.GetInt("port"),
		}
		a.ClusterDetails.ClusterName = m.GetString("cluster_name")
		a.AnalyzedInstanceDataCenter = m.GetString("data_center")
		a.AnalyzedInstanceRegion = m.GetString("region")
		a.AnalyzedInstancePhysicalEnvironment = m.GetString("physical_environment")
		a.AnalyzedInstanceBinlogCoordinates = BinlogCoordinates{
			LogFile: m.GetString("binary_log_file"),
			LogPos:  m.GetInt64("binary_log_pos"),
		}
		a.IsReadOnly = m.GetBool("read_only")
		a.LastCheckValid = m.GetBool("is_last_check_valid")
		a.LastCheckPartialSuccess = m.GetBool("last_check_partial_success")
		a.CountReplicas = m.GetUint("count_replicas")
		a.CountValidReplicas = m.GetUint("count_valid_replicas")
		a.CountValidReplicatingReplicas = m.GetUint("count_valid_replicating_replicas")
		a.CountReplicasFailingToConnectToMaster = m.GetUint("count_replicas_failing_to_connect")
		a.CountDowntimedReplicas = m.GetUint("count_downtimed_replicas")
		a.IsDowntimed = m.GetBool("is_downtimed")
		a.GTIDMode = "ON" // PG always uses WAL
		a.Replicas = *NewInstanceKeyMap()
		_ = a.ReadReplicaHostsFromString(m.GetString("slave_hosts"))

		masterHost := m.GetString("master_host")
		masterPort := m.GetInt("master_port")
		a.AnalyzedInstanceMasterKey = InstanceKey{
			Hostname: masterHost,
			Port:     masterPort,
		}

		// Determine analysis for this primary
		if !a.LastCheckValid {
			// Primary is unreachable
			if a.CountReplicas == 0 {
				// No standbys at all - nothing to fail over to
				a.Analysis = DeadMasterWithoutReplicas
				a.Description = "Primary is unreachable and has no standbys"
			} else if a.CountValidReplicas == 0 {
				// All standbys are also unreachable
				a.Analysis = DeadPrimaryAndSomeStandbys
				a.Description = "Primary is unreachable and none of its standbys are reachable"
			} else if a.CountValidReplicatingReplicas == 0 {
				// Primary down, standbys exist but none replicating
				a.Analysis = DeadPrimary
				a.Description = "Primary is unreachable but has reachable standbys"
			} else if a.CountValidReplicatingReplicas < a.CountReplicas {
				a.Analysis = DeadPrimaryAndSomeStandbys
				a.Description = "Primary is unreachable, some standbys are replicating"
			} else {
				// All replicas seem to be replicating fine, but primary check is invalid.
				// Might be a transient issue.
				a.Analysis = UnreachablePrimary
				a.Description = "Primary is unreachable but all standbys are replicating"
			}
		} else {
			// Primary is reachable
			if a.CountReplicas > 0 && a.CountValidReplicatingReplicas == 0 {
				a.Analysis = AllStandbyNotReplicating
				a.Description = "Primary is reachable but none of its standbys are replicating"
			} else if a.CountReplicas > 0 && a.CountValidReplicatingReplicas < a.CountReplicas {
				nonReplicating := a.CountReplicas - a.CountValidReplicatingReplicas
				if nonReplicating > 0 {
					a.Analysis = StandbyNotReplicating
					a.Description = fmt.Sprintf("Primary has %d standbys not replicating", nonReplicating)
				}
			}
			// If primary is reachable and all standbys are fine, Analysis remains NoProblem
		}

		if !hints.IncludeNoProblem && a.Analysis == NoProblem {
			// Skip non-problems unless requested
			return nil
		}
		if a.IsDowntimed && !hints.IncludeDowntimed {
			return nil
		}
		result = append(result, a)

		if a.CountReplicas > 0 && hints.AuditAnalysis {
			go func() { _ = auditInstanceAnalysisInChangelog(&a.AnalyzedInstanceKey, a.Analysis) }()
		}
		return nil
	})

	if err != nil {
		return result, log.Errore(err)
	}
	return result, nil
}

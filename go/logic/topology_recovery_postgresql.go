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

package logic

import (
	"fmt"

	"github.com/proxysql/orchestrator/go/config"
	"github.com/proxysql/orchestrator/go/inst"
)

// checkAndRecoverDeadPrimary is the PostgreSQL equivalent of checkAndRecoverDeadMaster.
// It handles promotion of a standby when a PostgreSQL primary is detected as dead.
func checkAndRecoverDeadPrimary(analysisEntry inst.ReplicationAnalysis, candidateInstanceKey *inst.InstanceKey, forceInstanceRecovery bool, skipProcesses bool) (recoveryAttempted bool, topologyRecovery *TopologyRecovery, err error) {
	if !forceInstanceRecovery && !analysisEntry.ClusterDetails.HasAutomatedMasterRecovery {
		return false, nil, nil
	}

	topologyRecovery, err = AttemptRecoveryRegistration(&analysisEntry, !forceInstanceRecovery, !forceInstanceRecovery)
	if topologyRecovery == nil {
		AuditTopologyRecovery(topologyRecovery, fmt.Sprintf("found an active or recent recovery on %+v. Will not issue another RecoverDeadPrimary.", analysisEntry.AnalyzedInstanceKey))
		return false, nil, err
	}

	AuditTopologyRecovery(topologyRecovery, fmt.Sprintf("will handle DeadPrimary event on %+v", analysisEntry.ClusterDetails.ClusterName))

	// Execute pre-failover hooks
	if !skipProcesses {
		if err := executeProcesses(config.Config.PreFailoverProcesses, "PreFailoverProcesses", topologyRecovery, true); err != nil {
			AuditTopologyRecovery(topologyRecovery, fmt.Sprintf("PreFailoverProcesses error: %+v", err))
			return false, topologyRecovery, err
		}
	}

	// Read replicas of the dead primary
	replicas, err := inst.ReadReplicaInstances(&analysisEntry.AnalyzedInstanceKey)
	if err != nil {
		AuditTopologyRecovery(topologyRecovery, fmt.Sprintf("error reading replicas of dead primary: %+v", err))
		return true, topologyRecovery, err
	}

	if len(replicas) == 0 {
		AuditTopologyRecovery(topologyRecovery, "no replicas found for dead primary; nothing to promote")
		resolveRecovery(topologyRecovery, nil)
		return true, topologyRecovery, fmt.Errorf("no replicas found for dead primary %+v", analysisEntry.AnalyzedInstanceKey)
	}

	// Pick the best standby for promotion
	bestStandby, err := inst.PostgreSQLGetBestStandbyForPromotion(replicas, candidateInstanceKey)
	if err != nil {
		AuditTopologyRecovery(topologyRecovery, fmt.Sprintf("error selecting best standby for promotion: %+v", err))
		resolveRecovery(topologyRecovery, nil)
		return true, topologyRecovery, err
	}

	AuditTopologyRecovery(topologyRecovery, fmt.Sprintf("selected %+v as promotion candidate", bestStandby.Key))

	// Promote the best standby
	promotedInstance, err := inst.PostgreSQLPromoteStandby(&bestStandby.Key)
	if err != nil {
		AuditTopologyRecovery(topologyRecovery, fmt.Sprintf("error promoting standby %+v: %+v", bestStandby.Key, err))
		resolveRecovery(topologyRecovery, nil)
		return true, topologyRecovery, err
	}

	AuditTopologyRecovery(topologyRecovery, fmt.Sprintf("successfully promoted %+v to primary", promotedInstance.Key))

	// Reconfigure remaining standbys to replicate from the new primary
	for _, replica := range replicas {
		if replica.Key.Equals(&promotedInstance.Key) {
			continue
		}
		AuditTopologyRecovery(topologyRecovery, fmt.Sprintf("reconfiguring standby %+v to replicate from new primary %+v", replica.Key, promotedInstance.Key))
		if err := inst.PostgreSQLReconfigureStandby(&replica.Key, &promotedInstance.Key); err != nil {
			AuditTopologyRecovery(topologyRecovery, fmt.Sprintf("error reconfiguring standby %+v: %+v (continuing with others)", replica.Key, err))
			topologyRecovery.LostReplicas.AddKey(replica.Key)
		}
	}

	// Resolve and execute post-failover hooks
	resolveRecovery(topologyRecovery, promotedInstance)
	if promotedInstance != nil {
		AuditTopologyRecovery(topologyRecovery, fmt.Sprintf("RecoverDeadPrimary: successfully promoted %+v", promotedInstance.Key))
		if !skipProcesses {
			topologyRecovery.SuccessorKey = &promotedInstance.Key
			executeProcesses(config.Config.PostMasterFailoverProcesses, "PostMasterFailoverProcesses", topologyRecovery, false)
			executeProcesses(config.Config.PostFailoverProcesses, "PostFailoverProcesses", topologyRecovery, false)
		}
	} else {
		if !skipProcesses {
			executeProcesses(config.Config.PostUnsuccessfulFailoverProcesses, "PostUnsuccessfulFailoverProcesses", topologyRecovery, false)
		}
	}

	return true, topologyRecovery, err
}

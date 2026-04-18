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
	"time"

	"github.com/proxysql/golib/log"
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

// PostgreSQLGracefulPrimarySwitchover performs a planned switchover of a
// PostgreSQL primary to a designated standby. It sets the primary read-only,
// waits for the standby to catch up, promotes the standby, reconfigures
// remaining standbys, and configures the demoted primary for repositioning.
func PostgreSQLGracefulPrimarySwitchover(clusterName string, designatedKey *inst.InstanceKey, auto bool) (topologyRecovery *TopologyRecovery, err error) {
	// --- Validate: read primary and standbys ---
	clusterMasters, err := inst.ReadClusterMaster(clusterName)
	if err != nil {
		return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: cannot deduce cluster primary for %+v: %v", clusterName, err)
	}
	if len(clusterMasters) != 1 {
		return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: found %d potential primaries for %+v, expected 1", len(clusterMasters), clusterName)
	}
	clusterPrimary := clusterMasters[0]

	standbys, err := inst.ReadReplicaInstances(&clusterPrimary.Key)
	if err != nil {
		return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: error reading standbys: %v", err)
	}
	if len(standbys) == 0 {
		return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: primary %+v has no standbys", clusterPrimary.Key)
	}

	// --- Determine designated standby ---
	var designatedStandby *inst.Instance
	if designatedKey != nil && designatedKey.IsValid() {
		// User specified a standby — verify it's a direct replica
		for _, s := range standbys {
			if s.Key.Equals(designatedKey) {
				designatedStandby = s
				break
			}
		}
		if designatedStandby == nil {
			return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: designated instance %+v is not a standby of %+v", *designatedKey, clusterPrimary.Key)
		}
	} else if len(standbys) == 1 {
		designatedStandby = standbys[0]
	} else if auto {
		designatedStandby, err = inst.PostgreSQLGetBestStandbyForPromotion(standbys, nil)
		if err != nil {
			return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: cannot auto-select standby: %v", err)
		}
	} else {
		return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: multiple standbys and no designated instance specified (use auto or specify -d)")
	}

	if designatedStandby.IsDowntimed {
		return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: designated standby %+v is downtimed", designatedStandby.Key)
	}
	if !designatedStandby.IsLastCheckValid {
		return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: designated standby %+v has invalid last check", designatedStandby.Key)
	}
	if !designatedStandby.HasReasonableMaintenanceReplicationLag() {
		return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: designated standby %+v has excessive replication lag", designatedStandby.Key)
	}

	log.Infof("PostgreSQLGracefulPrimarySwitchover: will demote %+v and promote %+v", clusterPrimary.Key, designatedStandby.Key)

	// --- Register recovery and build analysis entry ---
	analysisEntry, err := forceAnalysisEntry(clusterName, inst.DeadPrimary, inst.GracefulMasterTakeoverCommandHint, &clusterPrimary.Key)
	if err != nil {
		return nil, err
	}

	// --- Register recovery to prevent concurrent switchover attempts ---
	topologyRecovery, err = AttemptRecoveryRegistration(&analysisEntry, false, false)
	if topologyRecovery == nil {
		AuditTopologyRecovery(topologyRecovery, fmt.Sprintf("found an active or recent recovery on %+v. Will not issue another graceful switchover.", clusterName))
		return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: active or recent recovery exists for %+v, aborting", clusterName)
	}
	topologyRecovery.SuccessorKey = &designatedStandby.Key

	// --- Execute PreGracefulTakeoverProcesses ---
	if err := executeProcesses(config.Config.PreGracefulTakeoverProcesses, "PreGracefulTakeoverProcesses", topologyRecovery, true); err != nil {
		return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: PreGracefulTakeoverProcesses failed: %v", err)
	}

	// --- Set primary read-only ---
	log.Infof("PostgreSQLGracefulPrimarySwitchover: setting %+v read-only", clusterPrimary.Key)
	if _, err := inst.PostgreSQLSetReadOnly(&clusterPrimary.Key, true); err != nil {
		return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: failed to set read-only on %+v: %v", clusterPrimary.Key, err)
	}

	// --- Capture target LSN ---
	targetLSN, err := inst.PostgreSQLGetCurrentWALLSN(&clusterPrimary.Key)
	if err != nil {
		// Undo read-only before aborting
		_, _ = inst.PostgreSQLSetReadOnly(&clusterPrimary.Key, false)
		return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: failed to get WAL LSN from %+v: %v", clusterPrimary.Key, err)
	}
	log.Infof("PostgreSQLGracefulPrimarySwitchover: target LSN is %s", targetLSN)

	// --- Wait for standby to catch up ---
	// Use 3x the maintenance lag threshold since the primary is already read-only
	// and waiting longer only extends the maintenance window, not data risk.
	catchUpTimeout := 3 * time.Duration(config.Config.ReasonableMaintenanceReplicationLagSeconds) * time.Second
	if err := inst.PostgreSQLWaitForStandbyLSN(&designatedStandby.Key, targetLSN, catchUpTimeout); err != nil {
		// Undo read-only before aborting
		_, _ = inst.PostgreSQLSetReadOnly(&clusterPrimary.Key, false)
		return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: standby catch-up failed: %v", err)
	}

	// --- Promote designated standby ---
	promotedInstance, err := inst.PostgreSQLPromoteStandby(&designatedStandby.Key)
	if err != nil {
		// Undo read-only before aborting
		_, _ = inst.PostgreSQLSetReadOnly(&clusterPrimary.Key, false)
		return nil, fmt.Errorf("PostgreSQLGracefulPrimarySwitchover: promotion of %+v failed: %v", designatedStandby.Key, err)
	}
	log.Infof("PostgreSQLGracefulPrimarySwitchover: promoted %+v to primary", promotedInstance.Key)
	topologyRecovery.SuccessorKey = &promotedInstance.Key

	// --- Reconfigure remaining standbys ---
	for _, standby := range standbys {
		if standby.Key.Equals(&promotedInstance.Key) {
			continue
		}
		AuditTopologyRecovery(topologyRecovery, fmt.Sprintf("reconfiguring standby %+v to replicate from new primary %+v", standby.Key, promotedInstance.Key))
		if err := inst.PostgreSQLReconfigureStandby(&standby.Key, &promotedInstance.Key); err != nil {
			AuditTopologyRecovery(topologyRecovery, fmt.Sprintf("error reconfiguring standby %+v: %v (continuing)", standby.Key, err))
			topologyRecovery.LostReplicas.AddKey(standby.Key)
		}
	}

	// --- Configure demoted primary for repositioning ---
	AuditTopologyRecovery(topologyRecovery, fmt.Sprintf("configuring demoted primary %+v to replicate from %+v", clusterPrimary.Key, promotedInstance.Key))
	if err := inst.PostgreSQLRepositionAsStandby(&clusterPrimary.Key, &promotedInstance.Key); err != nil {
		AuditTopologyRecovery(topologyRecovery, fmt.Sprintf("error configuring demoted primary %+v: %v", clusterPrimary.Key, err))
	}

	// --- Resolve recovery ---
	resolveRecovery(topologyRecovery, promotedInstance)

	// --- Execute PostGracefulTakeoverProcesses ---
	executeProcesses(config.Config.PostGracefulTakeoverProcesses, "PostGracefulTakeoverProcesses", topologyRecovery, false)

	AuditTopologyRecovery(topologyRecovery, fmt.Sprintf("PostgreSQLGracefulPrimarySwitchover: completed. New primary: %+v", promotedInstance.Key))
	return topologyRecovery, nil
}

/*
   Copyright 2026 ProxySQL LLC

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

// shouldAvoidDeadMasterDueToIORunning reports whether any valid replica still
// has a running IO thread. That proves the master is accepting replica
// connections, so orchestrator must not classify the master as DeadMaster
// merely because SQL threads are stopped (see issue #106).
func shouldAvoidDeadMasterDueToIORunning(countValidIORunningReplicas uint) bool {
	return countValidIORunningReplicas > 0
}

// classifyUnreachableMasterWhenIORunning returns UnreachableMaster when the
// master check failed but at least one replica still has IO running.
// Returns NoProblem when this rule does not apply (caller continues other branches).
func classifyUnreachableMasterWhenIORunning(isMaster bool, lastCheckValid bool, countValidIORunningReplicas uint) AnalysisCode {
	if isMaster && !lastCheckValid && shouldAvoidDeadMasterDueToIORunning(countValidIORunningReplicas) {
		return UnreachableMaster
	}
	return NoProblem
}

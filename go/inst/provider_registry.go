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

import "sync"

var (
	providerMu   sync.RWMutex
	providerInst DatabaseProvider
)

// SetProvider sets the global database provider. This should be called
// during initialization to configure which database engine orchestrator
// manages. The default provider is MySQL.
func SetProvider(p DatabaseProvider) {
	providerMu.Lock()
	defer providerMu.Unlock()
	providerInst = p
}

// GetProvider returns the currently configured database provider.
func GetProvider() DatabaseProvider {
	providerMu.RLock()
	defer providerMu.RUnlock()
	return providerInst
}

func init() {
	// MySQL is the default provider, preserving backward compatibility.
	SetProvider(NewMySQLProvider())
}

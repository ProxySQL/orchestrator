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
	"testing"
)

func TestMySQLProviderName(t *testing.T) {
	p := NewMySQLProvider()
	if p.ProviderName() != "mysql" {
		t.Errorf("expected 'mysql', got %q", p.ProviderName())
	}
}

func TestDefaultProviderIsMySQL(t *testing.T) {
	p := GetProvider()
	if p == nil {
		t.Fatal("expected non-nil default provider")
	}
	if p.ProviderName() != "mysql" {
		t.Errorf("expected default provider 'mysql', got %q", p.ProviderName())
	}
}

func TestMySQLProviderImplementsInterface(t *testing.T) {
	// Compile-time interface satisfaction check
	var _ DatabaseProvider = (*MySQLProvider)(nil)
}

func TestSetAndGetProvider(t *testing.T) {
	original := GetProvider()
	defer SetProvider(original)

	custom := NewMySQLProvider()
	SetProvider(custom)

	got := GetProvider()
	if got != custom {
		t.Error("GetProvider did not return the provider set by SetProvider")
	}
}

func TestReplicationStatusDefaults(t *testing.T) {
	status := ReplicationStatus{}
	if status.ReplicaRunning {
		t.Error("expected ReplicaRunning to be false by default")
	}
	if status.Lag != 0 {
		t.Error("expected Lag to be 0 by default")
	}
	if status.Position != "" {
		t.Error("expected Position to be empty by default")
	}
}

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

import "testing"

func TestPostgreSQLProviderName(t *testing.T) {
	p := NewPostgreSQLProvider()
	if p.ProviderName() != "postgresql" {
		t.Errorf("expected 'postgresql', got %q", p.ProviderName())
	}
}

func TestPostgreSQLProviderImplementsInterface(t *testing.T) {
	var _ DatabaseProvider = (*PostgreSQLProvider)(nil)
}

func TestPostgreSQLProviderNewReturnsNonNil(t *testing.T) {
	p := NewPostgreSQLProvider()
	if p == nil {
		t.Error("expected non-nil provider")
	}
}

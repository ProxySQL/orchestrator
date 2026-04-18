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
	"time"
)

func TestPostgreSQLSetReadOnlyNilKey(t *testing.T) {
	_, err := PostgreSQLSetReadOnly(nil, true)
	if err == nil {
		t.Fatal("expected error for nil instanceKey")
	}
}

func TestPostgreSQLGetCurrentWALLSNNilKey(t *testing.T) {
	_, err := PostgreSQLGetCurrentWALLSN(nil)
	if err == nil {
		t.Fatal("expected error for nil instanceKey")
	}
}

func TestPostgreSQLWaitForStandbyLSNNilKey(t *testing.T) {
	err := PostgreSQLWaitForStandbyLSN(nil, "0/0", 1*time.Second)
	if err == nil {
		t.Fatal("expected error for nil instanceKey")
	}
}

func TestPostgreSQLWaitForStandbyLSNEmptyLSN(t *testing.T) {
	key := &InstanceKey{Hostname: "localhost", Port: 5432}
	err := PostgreSQLWaitForStandbyLSN(key, "", 1*time.Second)
	if err == nil {
		t.Fatal("expected error for empty targetLSN")
	}
}

func TestPostgreSQLRepositionAsStandbyNilKeys(t *testing.T) {
	key := &InstanceKey{Hostname: "localhost", Port: 5432}
	if err := PostgreSQLRepositionAsStandby(nil, key); err == nil {
		t.Fatal("expected error for nil instanceKey")
	}
	if err := PostgreSQLRepositionAsStandby(key, nil); err == nil {
		t.Fatal("expected error for nil newPrimaryKey")
	}
}

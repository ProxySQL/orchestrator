/*
   Copyright 2026 ProxySQL Authors

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
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	test "github.com/proxysql/golib/tests"
	"github.com/proxysql/orchestrator/go/config"
	"github.com/proxysql/orchestrator/go/db"
	"github.com/proxysql/orchestrator/go/inst"
)

func TestMain(m *testing.M) {
	config.MarkConfigurationLoaded()
	// keep hostname resolution local-only for the whole test binary, so that
	// ResolveHostname never spawns asynchronous backend writes
	config.Config.HostnameResolveMethod = "none"
	os.Exit(m.Run())
}

// waitForInstanceDaoInit blocks until the background initializeInstanceDao()
// goroutine has created the package-level caches, which are required by
// WriteInstance/ForgetInstance.
func waitForInstanceDaoInit(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		ready := false
		func() {
			defer func() {
				_ = recover()
			}()
			inst.InstanceIsForgotten(&inst.InstanceKey{})
			ready = true
		}()
		if ready {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("instance DAO caches were not initialized in time")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func setupSQLiteBackend(t *testing.T) {
	t.Helper()
	waitForInstanceDaoInit(t)
	origBackendDB := config.Config.BackendDB
	origSQLiteDataFile := config.Config.SQLite3DataFile
	origHostnameResolveMethod := config.Config.HostnameResolveMethod
	config.Config.BackendDB = "sqlite"
	config.Config.SQLite3DataFile = filepath.Join(t.TempDir(), "orchestrator.sqlite3")
	// avoid asynchronous hostname-resolve writes racing with the backend switch
	config.Config.HostnameResolveMethod = "none"
	t.Cleanup(func() {
		config.Config.BackendDB = origBackendDB
		config.Config.SQLite3DataFile = origSQLiteDataFile
		config.Config.HostnameResolveMethod = origHostnameResolveMethod
	})
	_, err := db.OpenOrchestrator()
	test.S(t).ExpectNil(err)
}

func writeTestInstance(t *testing.T, hostname string, port int) {
	t.Helper()
	instance := &inst.Instance{Key: inst.InstanceKey{Hostname: hostname, Port: port}}
	test.S(t).ExpectNil(inst.WriteInstance(instance, false, nil))
}

func setInstanceLastSeen(t *testing.T, hostname string, port int, hoursAgo int) {
	t.Helper()
	_, err := db.ExecOrchestrator(`
		update database_instance
		set last_seen = NOW() - interval ? hour
		where hostname = ? and port = ?`,
		hoursAgo, hostname, port,
	)
	test.S(t).ExpectNil(err)
}

func readInstanceKeyMap(t *testing.T) *inst.InstanceKeyMap {
	t.Helper()
	keys, err := inst.ReadAllInstanceKeys()
	test.S(t).ExpectNil(err)
	keyMap := inst.NewInstanceKeyMap()
	keyMap.AddKeys(keys)
	return keyMap
}

func mkSnapshotReader(t *testing.T, snapshotData *SnapshotData) io.ReadCloser {
	t.Helper()
	b, err := json.Marshal(snapshotData)
	test.S(t).ExpectNil(err)
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err = zw.Write(b)
	test.S(t).ExpectNil(err)
	test.S(t).ExpectNil(zw.Close())
	return io.NopCloser(bytes.NewReader(buf.Bytes()))
}

func TestReadRecentlySeenInstanceKeyMap(t *testing.T) {
	setupSQLiteBackend(t)

	writeTestInstance(t, "fresh", 3306)
	writeTestInstance(t, "borderline", 3306)
	writeTestInstance(t, "stale", 3306)
	writeTestInstance(t, "never-seen", 3306)
	setInstanceLastSeen(t, "borderline", 3306, 239)
	setInstanceLastSeen(t, "stale", 3306, 241)
	_, err := db.ExecOrchestrator(`update database_instance set last_seen = NULL where hostname = 'never-seen'`)
	test.S(t).ExpectNil(err)

	keys, err := inst.ReadRecentlySeenInstanceKeyMap(240)
	test.S(t).ExpectNil(err)

	test.S(t).ExpectTrue(keys.HasKey(inst.InstanceKey{Hostname: "fresh", Port: 3306}))
	test.S(t).ExpectTrue(keys.HasKey(inst.InstanceKey{Hostname: "borderline", Port: 3306}))
	test.S(t).ExpectFalse(keys.HasKey(inst.InstanceKey{Hostname: "stale", Port: 3306}))
	test.S(t).ExpectFalse(keys.HasKey(inst.InstanceKey{Hostname: "never-seen", Port: 3306}))
}

// TestSnapshotRestoreRetainsRecentlySeenInstances verifies the fix for issue #123:
// instances that exist in the local backend but are absent from a (stale) raft
// snapshot must not be purged at restore time while they are still within the
// UnseenInstanceForgetHours recency window. Genuinely stale instances must
// still be forgotten, and instances carried by the snapshot must be added.
func TestSnapshotRestoreRetainsRecentlySeenInstances(t *testing.T) {
	setupSQLiteBackend(t)

	origUnseenHours := config.Config.UnseenInstanceForgetHours
	config.Config.UnseenInstanceForgetHours = 240
	t.Cleanup(func() { config.Config.UnseenInstanceForgetHours = origUnseenHours })

	writeTestInstance(t, "fresh-a", 3306)
	writeTestInstance(t, "fresh-b", 3307)
	writeTestInstance(t, "stale", 3306)
	setInstanceLastSeen(t, "stale", 3306, 300)

	snapshotData := NewSnapshotData()
	snapshotData.MinimalInstances = []inst.MinimalInstance{
		{Key: inst.InstanceKey{Hostname: "in-snapshot", Port: 3306}},
	}

	applier := NewSnapshotDataCreatorApplier()
	test.S(t).ExpectNil(applier.Restore(mkSnapshotReader(t, snapshotData)))

	keyMap := readInstanceKeyMap(t)
	test.S(t).ExpectTrue(keyMap.HasKey(inst.InstanceKey{Hostname: "fresh-a", Port: 3306}))
	test.S(t).ExpectTrue(keyMap.HasKey(inst.InstanceKey{Hostname: "fresh-b", Port: 3307}))
	test.S(t).ExpectTrue(keyMap.HasKey(inst.InstanceKey{Hostname: "in-snapshot", Port: 3306}))
	test.S(t).ExpectFalse(keyMap.HasKey(inst.InstanceKey{Hostname: "stale", Port: 3306}))
}

// TestSnapshotRestoreForgetsUnseenStaleInstances ensures the recency guard does
// not prevent restore from forgetting instances that are both absent from the
// snapshot and already stale locally (the pre-existing behavior for genuine
// decommissions).
func TestSnapshotRestoreForgetsUnseenStaleInstances(t *testing.T) {
	setupSQLiteBackend(t)

	origUnseenHours := config.Config.UnseenInstanceForgetHours
	config.Config.UnseenInstanceForgetHours = 1
	t.Cleanup(func() { config.Config.UnseenInstanceForgetHours = origUnseenHours })

	writeTestInstance(t, "stale-a", 3306)
	writeTestInstance(t, "stale-b", 3307)
	setInstanceLastSeen(t, "stale-a", 3306, 2)
	setInstanceLastSeen(t, "stale-b", 3307, 2)

	snapshotData := NewSnapshotData()

	applier := NewSnapshotDataCreatorApplier()
	test.S(t).ExpectNil(applier.Restore(mkSnapshotReader(t, snapshotData)))

	keyMap := readInstanceKeyMap(t)
	test.S(t).ExpectFalse(keyMap.HasKey(inst.InstanceKey{Hostname: "stale-a", Port: 3306}))
	test.S(t).ExpectFalse(keyMap.HasKey(inst.InstanceKey{Hostname: "stale-b", Port: 3307}))
}

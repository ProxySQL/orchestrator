package inst

import (
	"database/sql"
	"testing"

	test "github.com/proxysql/golib/tests"
)

func TestSelectCanonicalChannelIndexEmpty(t *testing.T) {
	channels := []ChannelStatus{}
	idx := selectCanonicalChannelIndex(channels)
	test.S(t).ExpectEquals(idx, -1)
}

func TestSelectCanonicalChannelIndexSingleDefault(t *testing.T) {
	channels := []ChannelStatus{
		{ChannelName: "", MasterKey: InstanceKey{Hostname: "master1", Port: 3306}},
	}
	idx := selectCanonicalChannelIndex(channels)
	test.S(t).ExpectEquals(idx, 0)
}

func TestSelectCanonicalChannelIndexPrefersDefault(t *testing.T) {
	channels := []ChannelStatus{
		{ChannelName: "channel_a", MasterKey: InstanceKey{Hostname: "master1", Port: 3306}},
		{ChannelName: "", MasterKey: InstanceKey{Hostname: "master2", Port: 3306}},
		{ChannelName: "channel_b", MasterKey: InstanceKey{Hostname: "master3", Port: 3306}},
	}
	idx := selectCanonicalChannelIndex(channels)
	test.S(t).ExpectEquals(idx, 1)
}

func TestSelectCanonicalChannelIndexSkipsGRChannels(t *testing.T) {
	channels := []ChannelStatus{
		{ChannelName: "group_replication_applier", MasterKey: InstanceKey{Hostname: "master1", Port: 3306}},
		{ChannelName: "group_replication_recovery", MasterKey: InstanceKey{Hostname: "master1", Port: 3306}},
		{ChannelName: "my_channel", MasterKey: InstanceKey{Hostname: "master2", Port: 3306}},
	}
	idx := selectCanonicalChannelIndex(channels)
	test.S(t).ExpectEquals(idx, 2)
}

func TestSelectCanonicalChannelIndexAllGR(t *testing.T) {
	channels := []ChannelStatus{
		{ChannelName: "group_replication_applier", MasterKey: InstanceKey{Hostname: "master1", Port: 3306}},
		{ChannelName: "group_replication_recovery", MasterKey: InstanceKey{Hostname: "master1", Port: 3306}},
	}
	// When only GR channels exist, falls back to index 0
	idx := selectCanonicalChannelIndex(channels)
	test.S(t).ExpectEquals(idx, 0)
}

func TestSelectCanonicalChannelIndexMultipleNamed(t *testing.T) {
	channels := []ChannelStatus{
		{ChannelName: "channel_a", MasterKey: InstanceKey{Hostname: "master1", Port: 3306}},
		{ChannelName: "channel_b", MasterKey: InstanceKey{Hostname: "master2", Port: 3306}},
	}
	// No default channel, no GR channels: picks the first one
	idx := selectCanonicalChannelIndex(channels)
	test.S(t).ExpectEquals(idx, 0)
}

func TestIsGRInternalChannel(t *testing.T) {
	cs := ChannelStatus{ChannelName: "group_replication_applier"}
	test.S(t).ExpectTrue(cs.IsGRInternalChannel())

	cs2 := ChannelStatus{ChannelName: "group_replication_recovery"}
	test.S(t).ExpectTrue(cs2.IsGRInternalChannel())

	cs3 := ChannelStatus{ChannelName: "my_channel"}
	test.S(t).ExpectFalse(cs3.IsGRInternalChannel())

	cs4 := ChannelStatus{ChannelName: ""}
	test.S(t).ExpectFalse(cs4.IsGRInternalChannel())
}

func TestForChannelClause(t *testing.T) {
	test.S(t).ExpectEquals(forChannelClause(""), "")
	test.S(t).ExpectEquals(forChannelClause("my_channel"), " FOR CHANNEL 'my_channel'")
	test.S(t).ExpectEquals(forChannelClause("group_replication_applier"), " FOR CHANNEL 'group_replication_applier'")
}

func TestSingleSourceBehaviorUnchanged(t *testing.T) {
	// Verify that an instance with no replication channels behaves identically
	// to pre-channel-support behavior
	instance := NewInstance()
	instance.Key = InstanceKey{Hostname: "replica1", Port: 3306}
	instance.MasterKey = InstanceKey{Hostname: "master1", Port: 3306}
	instance.ReadBinlogCoordinates = BinlogCoordinates{LogFile: "mysql-bin.000001", LogPos: 100}
	instance.Version = "5.7.35"

	// No channels set
	test.S(t).ExpectEquals(len(instance.ReplicationChannels), 0)
	test.S(t).ExpectEquals(instance.ManagedChannelName, "")
	test.S(t).ExpectTrue(instance.IsReplica())
}

func TestMultiSourceInstanceManagedChannel(t *testing.T) {
	// Verify that when ReplicationChannels has multiple entries and we pick canonical,
	// the ManagedChannelName reflects the chosen channel
	channels := []ChannelStatus{
		{
			ChannelName:                 "channel_a",
			MasterKey:                   InstanceKey{Hostname: "master1", Port: 3306},
			ReplicationIOThreadRunning:  true,
			ReplicationSQLThreadRunning: true,
			SecondsBehindMaster:         sql.NullInt64{Int64: 0, Valid: true},
		},
		{
			ChannelName:                 "channel_b",
			MasterKey:                   InstanceKey{Hostname: "master2", Port: 3306},
			ReplicationIOThreadRunning:  true,
			ReplicationSQLThreadRunning: true,
			SecondsBehindMaster:         sql.NullInt64{Int64: 5, Valid: true},
		},
	}

	idx := selectCanonicalChannelIndex(channels)
	test.S(t).ExpectEquals(idx, 0)

	// The canonical channel should be channel_a (first non-GR channel)
	ch := channels[idx]
	test.S(t).ExpectEquals(ch.ChannelName, "channel_a")
	test.S(t).ExpectEquals(ch.MasterKey.Hostname, "master1")
}

func TestChannelAwareSQLGeneration(t *testing.T) {
	qsp := GetQueryStringProvider("5.7.35")

	// Without channel name -- no FOR CHANNEL clause
	stopSQL := qsp.StopReplicaForChannel("")
	test.S(t).ExpectTrue(len(stopSQL) > 0)
	test.S(t).ExpectEquals(stopSQL, qsp.stop_slave())

	startSQL := qsp.StartReplicaForChannel("")
	test.S(t).ExpectEquals(startSQL, qsp.start_slave())

	// With channel name -- should include FOR CHANNEL clause
	stopSQLCh := qsp.StopReplicaForChannel("my_channel")
	test.S(t).ExpectTrue(len(stopSQLCh) > len(stopSQL))
	expectedStop := qsp.stop_slave() + " FOR CHANNEL 'my_channel'"
	test.S(t).ExpectEquals(stopSQLCh, expectedStop)

	startSQLCh := qsp.StartReplicaForChannel("my_channel")
	expectedStart := qsp.start_slave() + " FOR CHANNEL 'my_channel'"
	test.S(t).ExpectEquals(startSQLCh, expectedStart)

	resetSQLCh := qsp.ResetReplicaForChannel("my_channel")
	expectedReset := qsp.reset_slave() + " FOR CHANNEL 'my_channel'"
	test.S(t).ExpectEquals(resetSQLCh, expectedReset)
}

func TestChannelAwareSQLGeneration84(t *testing.T) {
	// Test with MySQL 8.4+ which uses "stop replica" / "start replica" syntax
	qsp := GetQueryStringProvider("8.4.0")

	stopSQL := qsp.StopReplicaForChannel("my_channel")
	test.S(t).ExpectEquals(stopSQL, "stop replica FOR CHANNEL 'my_channel'")

	startSQL := qsp.StartReplicaForChannel("my_channel")
	test.S(t).ExpectEquals(startSQL, "start replica FOR CHANNEL 'my_channel'")

	resetSQL := qsp.ResetReplicaForChannel("my_channel")
	test.S(t).ExpectEquals(resetSQL, "reset replica FOR CHANNEL 'my_channel'")
}

func TestChannelAwareIOSQLThreadOperations(t *testing.T) {
	qsp := GetQueryStringProvider("5.7.35")

	// IO thread operations
	stopIO := qsp.StopReplicaIOThreadForChannel("ch1")
	test.S(t).ExpectEquals(stopIO, "stop slave io_thread FOR CHANNEL 'ch1'")

	startIO := qsp.StartReplicaIOThreadForChannel("ch1")
	test.S(t).ExpectEquals(startIO, "start slave io_thread FOR CHANNEL 'ch1'")

	// SQL thread operations
	stopSQLThread := qsp.StopReplicaSQLThreadForChannel("ch1")
	test.S(t).ExpectEquals(stopSQLThread, "stop slave sql_thread FOR CHANNEL 'ch1'")

	startSQLThread := qsp.StartReplicaSQLThreadForChannel("ch1")
	test.S(t).ExpectEquals(startSQLThread, "start slave sql_thread FOR CHANNEL 'ch1'")

	// Without channel name -- no FOR CHANNEL
	stopIODefault := qsp.StopReplicaIOThreadForChannel("")
	test.S(t).ExpectEquals(stopIODefault, "stop slave io_thread")
}

func TestSelectCanonicalChannelWithDefaultAndGR(t *testing.T) {
	// When default channel exists alongside GR channels, prefer default
	channels := []ChannelStatus{
		{ChannelName: "group_replication_applier"},
		{ChannelName: ""},
		{ChannelName: "group_replication_recovery"},
	}
	idx := selectCanonicalChannelIndex(channels)
	test.S(t).ExpectEquals(idx, 1)
}

func TestSelectCanonicalChannelWithNamedAndGR(t *testing.T) {
	// When no default channel but named + GR channels exist, prefer the named one
	channels := []ChannelStatus{
		{ChannelName: "group_replication_applier"},
		{ChannelName: "group_replication_recovery"},
		{ChannelName: "custom_repl"},
		{ChannelName: "another_repl"},
	}
	idx := selectCanonicalChannelIndex(channels)
	test.S(t).ExpectEquals(idx, 2)
}

package orcraft

import (
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/raft"
)

// freeLocalPort asks the OS for a free TCP port on 127.0.0.1.
func freeLocalPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to allocate a free port: %+v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		t.Fatalf("failed to release listener: %+v", err)
	}
	return port
}

// assertSingleNodeIdentity bootstraps a single-node store advertising as
// advertiseAddr and checks that raft's persisted configuration keys the node
// by that exact address, rather than some resolved variant of it.
func assertSingleNodeIdentity(t *testing.T, advertiseAddr string) {
	t.Helper()
	raftDir := t.TempDir()

	raftBind, err := normalizeRaftNode(advertiseAddr)
	if err != nil {
		t.Fatalf("normalizeRaftNode failed: %+v", err)
	}
	raftAdvertise, err := normalizeRaftNode(advertiseAddr)
	if err != nil {
		t.Fatalf("normalizeRaftNode failed: %+v", err)
	}

	s := NewStore(raftDir, raftBind, raftAdvertise, nil, nil)
	if err := s.Open(nil); err != nil {
		t.Fatalf("Store.Open failed: %+v", err)
	}
	defer s.raft.Shutdown()

	// Wait for the single-node cluster to elect itself leader.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if s.raft.State() == raft.Leader {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if s.raft.State() != raft.Leader {
		t.Fatalf("single-node raft store never became leader (state=%v)", s.raft.State())
	}

	future := s.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		t.Fatalf("GetConfiguration failed: %+v", err)
	}
	servers := future.Configuration().Servers
	if len(servers) != 1 {
		t.Fatalf("expected exactly 1 server in configuration, got %d: %+v", len(servers), servers)
	}

	server := servers[0]
	if string(server.ID) != advertiseAddr {
		t.Errorf("expected raft ServerID to be %q, got %q (identity must not be a resolved, point-in-time address)", advertiseAddr, string(server.ID))
	}
	if string(server.Address) != advertiseAddr {
		t.Errorf("expected raft ServerAddress to be %q, got %q", advertiseAddr, string(server.Address))
	}
}

func TestStoreOpenKeepsHostnameAsServerIdentity(t *testing.T) {
	assertSingleNodeIdentity(t, "localhost:"+strconv.Itoa(freeLocalPort(t)))
}

func TestStoreOpenKeepsIPv6AsServerIdentity(t *testing.T) {
	assertSingleNodeIdentity(t, net.JoinHostPort("::1", strconv.Itoa(freeLocalPort(t))))
}

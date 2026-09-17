package orcraft

import (
	"testing"

	"github.com/proxysql/orchestrator/go/config"
)

func TestNormalizeRaftNodePreservesHostnameWithPort(t *testing.T) {
	node, err := normalizeRaftNode("orchestrator-0.orchestrator.default.svc.cluster.local:10008")
	if err != nil {
		t.Fatalf("normalizeRaftNode returned error: %+v", err)
	}
	if node != "orchestrator-0.orchestrator.default.svc.cluster.local:10008" {
		t.Errorf("expected hostname:port to be preserved unresolved, got %q", node)
	}
}

func TestNormalizeRaftNodePassesThroughLiteralIP(t *testing.T) {
	node, err := normalizeRaftNode("192.168.1.10:10008")
	if err != nil {
		t.Fatalf("normalizeRaftNode returned error: %+v", err)
	}
	if node != "192.168.1.10:10008" {
		t.Errorf("expected literal IP to pass through unchanged, got %q", node)
	}
}

func withConfig(t *testing.T, httpAdvertise, listenAddress string, useSSL bool, fn func()) {
	t.Helper()
	origHTTPAdvertise := config.Config.HTTPAdvertise
	origListenAddress := config.Config.ListenAddress
	origUseSSL := config.Config.UseSSL
	config.Config.HTTPAdvertise = httpAdvertise
	config.Config.ListenAddress = listenAddress
	config.Config.UseSSL = useSSL
	defer func() {
		config.Config.HTTPAdvertise = origHTTPAdvertise
		config.Config.ListenAddress = origListenAddress
		config.Config.UseSSL = origUseSSL
	}()
	fn()
}

func TestComputeLeaderURIWithIPv6Advertise(t *testing.T) {
	withConfig(t, "", "0.0.0.0:3000", false, func() {
		uri, err := computeLeaderURI("[::1]:10008")
		if err != nil {
			t.Fatalf("computeLeaderURI returned error: %+v", err)
		}
		if uri != "http://[::1]:3000" {
			t.Errorf("expected %q, got %q", "http://[::1]:3000", uri)
		}
	})
}

func TestComputeLeaderURIWithHostname(t *testing.T) {
	withConfig(t, "", "0.0.0.0:3000", false, func() {
		uri, err := computeLeaderURI("orchestrator-0.svc.cluster.local:10008")
		if err != nil {
			t.Fatalf("computeLeaderURI returned error: %+v", err)
		}
		if uri != "http://orchestrator-0.svc.cluster.local:3000" {
			t.Errorf("expected %q, got %q", "http://orchestrator-0.svc.cluster.local:3000", uri)
		}
	})
}

func TestComputeLeaderURIPrefersExplicitHTTPAdvertise(t *testing.T) {
	withConfig(t, "https://explicit:9999", "0.0.0.0:3000", false, func() {
		uri, err := computeLeaderURI("[::1]:10008")
		if err != nil {
			t.Fatalf("computeLeaderURI returned error: %+v", err)
		}
		if uri != "https://explicit:9999" {
			t.Errorf("expected %q, got %q", "https://explicit:9999", uri)
		}
	})
}

func TestNormalizeRaftNodePassesThroughLiteralIPv6(t *testing.T) {
	node, err := normalizeRaftNode("[::1]:10008")
	if err != nil {
		t.Fatalf("normalizeRaftNode returned error: %+v", err)
	}
	if node != "[::1]:10008" {
		t.Errorf("expected literal IPv6 to pass through unchanged, got %q", node)
	}
}

func TestNormalizeRaftNodeAddsDefaultPortToIPv6(t *testing.T) {
	originalPort := config.Config.DefaultRaftPort
	config.Config.DefaultRaftPort = 10008
	defer func() { config.Config.DefaultRaftPort = originalPort }()

	node, err := normalizeRaftNode("::1")
	if err != nil {
		t.Fatalf("normalizeRaftNode returned error: %+v", err)
	}
	if node != "[::1]:10008" {
		t.Errorf("expected default port to be appended in bracketed form, got %q", node)
	}
}

func TestNormalizeRaftNodeAddsDefaultPort(t *testing.T) {
	originalPort := config.Config.DefaultRaftPort
	config.Config.DefaultRaftPort = 10008
	defer func() { config.Config.DefaultRaftPort = originalPort }()

	node, err := normalizeRaftNode("orchestrator-0.orchestrator.default.svc.cluster.local")
	if err != nil {
		t.Fatalf("normalizeRaftNode returned error: %+v", err)
	}
	if node != "orchestrator-0.orchestrator.default.svc.cluster.local:10008" {
		t.Errorf("expected default port to be appended, got %q", node)
	}
}

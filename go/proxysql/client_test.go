package proxysql

import (
	"testing"
)

func TestNewClientNilWhenUnconfigured(t *testing.T) {
	client := NewClient("", 6032, "admin", "admin", false)
	if client != nil {
		t.Error("expected nil client when address is empty")
	}
}

func TestNewClientNonNilWhenConfigured(t *testing.T) {
	client := NewClient("127.0.0.1", 6032, "admin", "admin", false)
	if client == nil {
		t.Error("expected non-nil client when address is provided")
	}
}

func TestClientDSN(t *testing.T) {
	client := NewClient("127.0.0.1", 6032, "admin", "secret", false)
	expected := "admin:secret@tcp(127.0.0.1:6032)/?interpolateParams=true&timeout=1s&readTimeout=1s&writeTimeout=1s"
	if client.dsn != expected {
		t.Errorf("expected DSN %q, got %q", expected, client.dsn)
	}
}

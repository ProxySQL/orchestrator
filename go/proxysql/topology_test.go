package proxysql

import (
	"testing"
)

func TestServerEntryStruct(t *testing.T) {
	s := ServerEntry{
		HostgroupID: 10,
		Hostname:    "db1.example.com",
		Port:        3306,
		Status:      "ONLINE",
		Weight:      1000,
		MaxConns:    100,
		Comment:     "primary writer",
	}

	if s.HostgroupID != 10 {
		t.Errorf("expected HostgroupID=10, got %d", s.HostgroupID)
	}
	if s.Hostname != "db1.example.com" {
		t.Errorf("expected Hostname=db1.example.com, got %s", s.Hostname)
	}
	if s.Port != 3306 {
		t.Errorf("expected Port=3306, got %d", s.Port)
	}
	if s.Status != "ONLINE" {
		t.Errorf("expected Status=ONLINE, got %s", s.Status)
	}
	if s.Weight != 1000 {
		t.Errorf("expected Weight=1000, got %d", s.Weight)
	}
	if s.MaxConns != 100 {
		t.Errorf("expected MaxConns=100, got %d", s.MaxConns)
	}
	if s.Comment != "primary writer" {
		t.Errorf("expected Comment='primary writer', got '%s'", s.Comment)
	}
}

func TestGetServersNilClient(t *testing.T) {
	var c *Client
	servers, err := c.GetServers()
	if err != nil {
		t.Errorf("expected no error for nil client, got %v", err)
	}
	if servers == nil {
		t.Error("expected non-nil empty slice, got nil")
	}
	if len(servers) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(servers))
	}
}

func TestGetServersByHostgroupNilClient(t *testing.T) {
	var c *Client
	servers, err := c.GetServersByHostgroup(10)
	if err != nil {
		t.Errorf("expected no error for nil client, got %v", err)
	}
	if servers == nil {
		t.Error("expected non-nil empty slice, got nil")
	}
	if len(servers) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(servers))
	}
}

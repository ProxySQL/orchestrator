package proxysql

import (
	"testing"
)

func TestHookNilClientIsNoop(t *testing.T) {
	hook := NewHook(nil, 10, 20, "offline_soft")
	err := hook.PreFailover("old-master", 3306)
	if err != nil {
		t.Errorf("expected nil error for PreFailover with nil client, got %v", err)
	}
	err = hook.PostFailover("new-master", 3306, "old-master", 3306)
	if err != nil {
		t.Errorf("expected nil error for PostFailover with nil client, got %v", err)
	}
}

func TestHookUnconfiguredHostgroupIsNoop(t *testing.T) {
	client := NewClient("127.0.0.1", 6032, "admin", "admin", false)
	hook := NewHook(client, 0, 0, "offline_soft")
	err := hook.PreFailover("old-master", 3306)
	if err != nil {
		t.Errorf("expected nil error for unconfigured hostgroups, got %v", err)
	}
}

func TestPreFailoverSQLGeneration(t *testing.T) {
	tests := []struct {
		action       string
		host         string
		port         int
		expectedSQL  string
		expectedArgs int
	}{
		{
			action:       "offline_soft",
			host:         "db1.example.com",
			port:         3306,
			expectedSQL:  "UPDATE mysql_servers SET status='OFFLINE_SOFT' WHERE hostname=? AND port=? AND hostgroup_id=?",
			expectedArgs: 3,
		},
		{
			action:       "weight_zero",
			host:         "db1.example.com",
			port:         3306,
			expectedSQL:  "UPDATE mysql_servers SET weight=0 WHERE hostname=? AND port=? AND hostgroup_id=?",
			expectedArgs: 3,
		},
		{
			action:       "none",
			host:         "db1.example.com",
			port:         3306,
			expectedSQL:  "",
			expectedArgs: 0,
		},
	}
	for _, tt := range tests {
		sql, args := buildPreFailoverSQL(tt.action, tt.host, tt.port, 10)
		if sql != tt.expectedSQL {
			t.Errorf("action=%s: expected SQL %q, got %q", tt.action, tt.expectedSQL, sql)
		}
		if len(args) != tt.expectedArgs {
			t.Errorf("action=%s: expected %d args, got %d", tt.action, tt.expectedArgs, len(args))
		}
	}
}

func TestPostFailoverSQLGeneration(t *testing.T) {
	sqls, args := buildPostFailoverSQL("new-master", 3306, "old-master", 3306, 10, 20)
	if len(sqls) < 3 {
		t.Errorf("expected at least 3 SQL statements for post-failover, got %d", len(sqls))
	}
	if len(args) != len(sqls) {
		t.Errorf("expected args slice length to match sqls length, got %d vs %d", len(args), len(sqls))
	}
}

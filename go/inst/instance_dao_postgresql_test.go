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

	"github.com/proxysql/orchestrator/go/config"
)

func TestParsePostgreSQLVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"PostgreSQL 16.2 (Debian 16.2-1.pgdg120+2) on x86_64-pc-linux-gnu", "16.2"},
		{"PostgreSQL 15.4 on x86_64-pc-linux-gnu, compiled by gcc", "15.4"},
		{"PostgreSQL 14.0", "14.0"},
		{"something unexpected", "something unexpected"},
	}
	for _, tt := range tests {
		result := parsePostgreSQLVersion(tt.input)
		if result != tt.expected {
			t.Errorf("parsePostgreSQLVersion(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestParseConnInfo(t *testing.T) {
	tests := []struct {
		conninfo     string
		expectedHost string
		expectedPort int
	}{
		{"host=primary.example.com port=5432 user=replicator", "primary.example.com", 5432},
		{"host=10.0.0.1 port=5433 user=rep", "10.0.0.1", 5433},
		{"host=myhost user=rep", "myhost", 0},
		{"user=rep password=secret", "", 0},
	}
	for _, tt := range tests {
		host, port := parseConnInfo(tt.conninfo)
		if host != tt.expectedHost || port != tt.expectedPort {
			t.Errorf("parseConnInfo(%q) = (%q, %d), want (%q, %d)", tt.conninfo, host, port, tt.expectedHost, tt.expectedPort)
		}
	}
}

func TestLsnToInt64(t *testing.T) {
	tests := []struct {
		lsn      string
		expected int64
	}{
		{"0/16B3748", 0x16B3748},
		{"1/16B3748", (1 << 32) | 0x16B3748},
		{"0/0", 0},
		{"invalid", 0},
	}
	for _, tt := range tests {
		result := lsnToInt64(tt.lsn)
		if result != tt.expected {
			t.Errorf("lsnToInt64(%q) = %d, want %d", tt.lsn, result, tt.expected)
		}
	}
}

func TestBoolToString(t *testing.T) {
	if boolToString(true, "Yes", "No") != "Yes" {
		t.Error("expected Yes for true")
	}
	if boolToString(false, "Yes", "No") != "No" {
		t.Error("expected No for false")
	}
}

func TestProviderTypeDefaultIsMySQL(t *testing.T) {
	// Verify that the default config has ProviderType set to "mysql"
	cfg := config.Config
	if cfg.ProviderType != "mysql" {
		t.Errorf("expected default ProviderType to be 'mysql', got %q", cfg.ProviderType)
	}
}

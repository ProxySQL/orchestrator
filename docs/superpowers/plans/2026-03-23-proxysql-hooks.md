# ProxySQL Failover Hooks — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Built-in pre/post failover hooks that update ProxySQL hostgroups via its Admin API — no custom scripts needed.

**Architecture:** New `go/proxysql/` package provides a ProxySQL Admin client. A `ProxySQLHook` integrates into the existing failover flow in `topology_recovery.go`, called alongside KV store updates after master promotion. Configuration follows the flat-field pattern in `config.go` (same as Consul). The hook is nil-safe: if ProxySQL is not configured, all operations are no-ops.

**Tech Stack:** Go, MySQL driver (ProxySQL Admin speaks MySQL protocol), existing orchestrator config/recovery infrastructure.

**Spec:** GitHub issue #30

---

## File Structure

| File | Responsibility |
|------|---------------|
| `go/proxysql/client.go` | ProxySQL Admin connection management, query execution |
| `go/proxysql/client_test.go` | Unit tests for client (mocked DB) |
| `go/proxysql/hook.go` | Pre/post failover hook logic: drain master, promote new master in hostgroups |
| `go/proxysql/hook_test.go` | Unit tests for hook logic |
| `go/config/config.go` | New `ProxySQL*` configuration fields |
| `go/config/config_test.go` | Tests for ProxySQL config defaults and validation |
| `go/logic/topology_recovery.go` | Call ProxySQL hooks during failover (alongside KV writes) |
| `go/app/cli.go` | New `proxysql-test` CLI command |
| `go/logic/orchestrator.go` | Initialize ProxySQL hook at startup (alongside KV store init) |
| `docs/proxysql-hooks.md` | User documentation |

---

### Task 1: ProxySQL configuration fields

**GitHub Issue Title:** Add ProxySQL configuration fields

**Files:**
- Modify: `go/config/config.go`
- Modify: `go/config/config_test.go`

- [ ] **Step 1: Write test for ProxySQL config defaults**

In `go/config/config_test.go`, add:

```go
func TestProxySQLConfigDefaults(t *testing.T) {
	cfg := newConfiguration()
	if cfg.ProxySQLAdminPort != 6032 {
		t.Errorf("expected default ProxySQLAdminPort=6032, got %d", cfg.ProxySQLAdminPort)
	}
	if cfg.ProxySQLAdminUser != "admin" {
		t.Errorf("expected default ProxySQLAdminUser=admin, got %s", cfg.ProxySQLAdminUser)
	}
	if cfg.ProxySQLWriterHostgroup != 0 {
		t.Errorf("expected default ProxySQLWriterHostgroup=0, got %d", cfg.ProxySQLWriterHostgroup)
	}
	if cfg.ProxySQLReaderHostgroup != 0 {
		t.Errorf("expected default ProxySQLReaderHostgroup=0, got %d", cfg.ProxySQLReaderHostgroup)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./go/config/... -run TestProxySQLConfigDefaults -v
```

Expected: FAIL — `ProxySQLAdminPort` field does not exist.

- [ ] **Step 3: Add ProxySQL config fields**

In `go/config/config.go`, add these fields to the `Configuration` struct after the ZK/Consul section (after line ~284):

```go
	ProxySQLAdminAddress       string            // Address of ProxySQL Admin interface. Example: 127.0.0.1
	ProxySQLAdminPort          int               // Port of ProxySQL Admin interface. Default: 6032
	ProxySQLAdminUser          string            // Username for ProxySQL Admin. Default: admin
	ProxySQLAdminPassword      string            // Password for ProxySQL Admin
	ProxySQLAdminUseTLS        bool              // Use TLS for ProxySQL Admin connection
	ProxySQLWriterHostgroup    int               // ProxySQL hostgroup ID for the writer (master). 0 means unconfigured.
	ProxySQLReaderHostgroup    int               // ProxySQL hostgroup ID for readers (replicas). 0 means unconfigured.
	ProxySQLPreFailoverAction  string            // Action on old master before failover: "offline_soft" (default), "weight_zero", "none"
```

In `newConfiguration()`, add defaults:

```go
	config.ProxySQLAdminPort = 6032
	config.ProxySQLAdminUser = "admin"
	config.ProxySQLPreFailoverAction = "offline_soft"
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./go/config/... -run TestProxySQLConfigDefaults -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go/config/config.go go/config/config_test.go
git commit -m "Add ProxySQL configuration fields to Config struct"
```

---

### Task 2: ProxySQL Admin client

**GitHub Issue Title:** Implement ProxySQL Admin client library

**Files:**
- Create: `go/proxysql/client.go`
- Create: `go/proxysql/client_test.go`

- [ ] **Step 1: Write test for client creation**

Create `go/proxysql/client_test.go`:

```go
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
	// This creates the struct but doesn't connect — no ProxySQL needed
	client := NewClient("127.0.0.1", 6032, "admin", "admin", false)
	if client == nil {
		t.Error("expected non-nil client when address is provided")
	}
}

func TestClientDSN(t *testing.T) {
	client := NewClient("127.0.0.1", 6032, "admin", "secret", false)
	expected := "admin:secret@tcp(127.0.0.1:6032)/"
	if client.dsn != expected {
		t.Errorf("expected DSN %q, got %q", expected, client.dsn)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./go/proxysql/... -run TestNewClient -v
```

Expected: FAIL — package doesn't exist yet.

- [ ] **Step 3: Implement client**

Create `go/proxysql/client.go`:

```go
package proxysql

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/proxysql/golib/log"
)

// Client manages a connection to ProxySQL's Admin interface.
// The Admin interface speaks the MySQL protocol, so we use a standard MySQL driver.
type Client struct {
	dsn     string
	address string
	port    int
}

// NewClient creates a new ProxySQL Admin client.
// Returns nil if address is empty (unconfigured — all operations become no-ops).
func NewClient(address string, port int, user, password string, useTLS bool) *Client {
	if address == "" {
		return nil
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/", user, password, address, port)
	if useTLS {
		dsn += "?tls=true"
	}
	return &Client{
		dsn:     dsn,
		address: address,
		port:    port,
	}
}

// openDB opens a fresh connection to ProxySQL Admin.
// Callers must close the returned *sql.DB.
func (c *Client) openDB() (*sql.DB, error) {
	db, err := sql.Open("mysql", c.dsn)
	if err != nil {
		return nil, fmt.Errorf("proxysql: failed to open connection to %s:%d: %v", c.address, c.port, err)
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

// Exec executes an admin command against ProxySQL.
func (c *Client) Exec(query string, args ...interface{}) error {
	db, err := c.openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("proxysql: exec %q failed: %v", query, err)
	}
	return nil
}

// Query executes a query and returns rows. Caller must close rows.
func (c *Client) Query(query string, args ...interface{}) (*sql.Rows, *sql.DB, error) {
	db, err := c.openDB()
	if err != nil {
		return nil, nil, err
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("proxysql: query %q failed: %v", query, err)
	}
	// Caller must close both rows and db
	return rows, db, nil
}

// Ping verifies the connection to ProxySQL Admin.
func (c *Client) Ping() error {
	db, err := c.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return fmt.Errorf("proxysql: ping failed on %s:%d: %v", c.address, c.port, err)
	}
	log.Infof("proxysql: successfully connected to Admin at %s:%d", c.address, c.port)
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./go/proxysql/... -v
```

Expected: PASS (all 3 tests)

- [ ] **Step 5: Commit**

```bash
git add go/proxysql/
git commit -m "Add ProxySQL Admin client library"
```

---

### Task 3: ProxySQL hook logic

**GitHub Issue Title:** Implement ProxySQL failover hook

**Files:**
- Create: `go/proxysql/hook.go`
- Create: `go/proxysql/hook_test.go`

- [ ] **Step 1: Write tests for hook operations**

Create `go/proxysql/hook_test.go`:

```go
package proxysql

import (
	"testing"
)

func TestHookNilClientIsNoop(t *testing.T) {
	hook := NewHook(nil, 10, 20, "offline_soft")
	// All operations should succeed silently with nil client
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
	// Hostgroup 0 means unconfigured — should be no-op
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./go/proxysql/... -run TestHook -v
go test ./go/proxysql/... -run TestPreFailover -v
go test ./go/proxysql/... -run TestPostFailover -v
```

Expected: FAIL — `NewHook`, `buildPreFailoverSQL`, `buildPostFailoverSQL` not defined.

- [ ] **Step 3: Implement hook**

Create `go/proxysql/hook.go`:

```go
package proxysql

import (
	"fmt"

	"github.com/proxysql/golib/log"
)

// Hook manages ProxySQL hostgroup updates during orchestrator failovers.
type Hook struct {
	client           *Client
	writerHostgroup  int
	readerHostgroup  int
	preFailoverAction string
}

// NewHook creates a ProxySQL failover hook.
// If client is nil or hostgroups are 0 (unconfigured), all operations are no-ops.
func NewHook(client *Client, writerHostgroup, readerHostgroup int, preFailoverAction string) *Hook {
	if preFailoverAction == "" {
		preFailoverAction = "offline_soft"
	}
	return &Hook{
		client:           client,
		writerHostgroup:  writerHostgroup,
		readerHostgroup:  readerHostgroup,
		preFailoverAction: preFailoverAction,
	}
}

// IsConfigured returns true if ProxySQL hooks are active.
func (h *Hook) IsConfigured() bool {
	return h.client != nil && h.writerHostgroup > 0
}

// PreFailover drains the old master in ProxySQL before failover begins.
func (h *Hook) PreFailover(oldMasterHost string, oldMasterPort int) error {
	if !h.IsConfigured() {
		return nil
	}

	log.Infof("proxysql: pre-failover: draining old master %s:%d (action=%s)", oldMasterHost, oldMasterPort, h.preFailoverAction)

	query, args := buildPreFailoverSQL(h.preFailoverAction, oldMasterHost, oldMasterPort, h.writerHostgroup)
	if query == "" {
		return nil
	}
	if err := h.client.Exec(query, args...); err != nil {
		return fmt.Errorf("proxysql: pre-failover drain failed: %v", err)
	}
	if err := h.client.Exec("LOAD MYSQL SERVERS TO RUNTIME"); err != nil {
		return fmt.Errorf("proxysql: pre-failover LOAD TO RUNTIME failed: %v", err)
	}

	log.Infof("proxysql: pre-failover: drained old master %s:%d", oldMasterHost, oldMasterPort)
	return nil
}

// PostFailover updates ProxySQL to route traffic to the new master.
func (h *Hook) PostFailover(newMasterHost string, newMasterPort int, oldMasterHost string, oldMasterPort int) error {
	if !h.IsConfigured() {
		return nil
	}

	log.Infof("proxysql: post-failover: promoting %s:%d as writer in hostgroup %d", newMasterHost, newMasterPort, h.writerHostgroup)

	sqls, sqlArgs := buildPostFailoverSQL(newMasterHost, newMasterPort, oldMasterHost, oldMasterPort, h.writerHostgroup, h.readerHostgroup)
	for i, query := range sqls {
		if err := h.client.Exec(query, sqlArgs[i]...); err != nil {
			return fmt.Errorf("proxysql: post-failover exec failed: %v", err)
		}
	}
	if err := h.client.Exec("LOAD MYSQL SERVERS TO RUNTIME"); err != nil {
		return fmt.Errorf("proxysql: post-failover LOAD TO RUNTIME failed: %v", err)
	}
	if err := h.client.Exec("SAVE MYSQL SERVERS TO DISK"); err != nil {
		log.Errorf("proxysql: post-failover SAVE TO DISK failed (non-fatal): %v", err)
		// Non-fatal: runtime is already updated
	}

	log.Infof("proxysql: post-failover: promoted %s:%d as writer", newMasterHost, newMasterPort)
	return nil
}

// buildPreFailoverSQL returns the parameterized SQL and args to drain the old master.
func buildPreFailoverSQL(action, host string, port, writerHostgroup int) (string, []interface{}) {
	args := []interface{}{host, port, writerHostgroup}
	switch action {
	case "offline_soft":
		return "UPDATE mysql_servers SET status='OFFLINE_SOFT' WHERE hostname=? AND port=? AND hostgroup_id=?", args
	case "weight_zero":
		return "UPDATE mysql_servers SET weight=0 WHERE hostname=? AND port=? AND hostgroup_id=?", args
	case "none":
		return "", nil
	default:
		return "UPDATE mysql_servers SET status='OFFLINE_SOFT' WHERE hostname=? AND port=? AND hostgroup_id=?", args
	}
}

// buildPostFailoverSQL returns parameterized SQL statements and args to promote the new master.
func buildPostFailoverSQL(newHost string, newPort int, oldHost string, oldPort int, writerHostgroup, readerHostgroup int) ([]string, [][]interface{}) {
	sqls := []string{
		// Remove old master from writer hostgroup
		"DELETE FROM mysql_servers WHERE hostname=? AND port=? AND hostgroup_id=?",
		// Insert new master into writer hostgroup (REPLACE handles case where it's already there)
		"REPLACE INTO mysql_servers (hostgroup_id, hostname, port) VALUES (?, ?, ?)",
	}
	args := [][]interface{}{
		{oldHost, oldPort, writerHostgroup},
		{writerHostgroup, newHost, newPort},
	}

	// If reader hostgroup is configured, remove new master from readers (it's now the writer)
	if readerHostgroup > 0 {
		sqls = append(sqls,
			"DELETE FROM mysql_servers WHERE hostname=? AND port=? AND hostgroup_id=?",
		)
		args = append(args, []interface{}{newHost, newPort, readerHostgroup})
		// Optionally add old master to reader hostgroup (it may come back as a replica)
		sqls = append(sqls,
			"REPLACE INTO mysql_servers (hostgroup_id, hostname, port, status) VALUES (?, ?, ?, 'OFFLINE_SOFT')",
		)
		args = append(args, []interface{}{readerHostgroup, oldHost, oldPort})
	}

	return sqls, args
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./go/proxysql/... -v
```

Expected: PASS (all tests)

- [ ] **Step 5: Commit**

```bash
git add go/proxysql/hook.go go/proxysql/hook_test.go
git commit -m "Implement ProxySQL failover hook logic"
```

---

### Task 4: Hook initialization and singleton

**GitHub Issue Title:** Initialize ProxySQL hook at startup

**Files:**
- Create: `go/proxysql/init.go`
- Modify: `go/logic/orchestrator.go`
- Modify: `go/app/cli.go`

- [ ] **Step 1: Create hook singleton with initialization**

Create `go/proxysql/init.go`:

```go
package proxysql

import (
	"sync"

	"github.com/proxysql/orchestrator/go/config"
	"github.com/proxysql/golib/log"
)

var (
	hookOnce    sync.Once
	// defaultHook is initialized to a no-op hook to avoid data races.
	// GetHook() is safe to call from any goroutine at any time.
	defaultHook = NewHook(nil, 0, 0, "")
)

// InitHook initializes the global ProxySQL hook from configuration.
// Safe to call multiple times — only the first call has effect.
func InitHook() {
	hookOnce.Do(func() {
		client := NewClient(
			config.Config.ProxySQLAdminAddress,
			config.Config.ProxySQLAdminPort,
			config.Config.ProxySQLAdminUser,
			config.Config.ProxySQLAdminPassword,
			config.Config.ProxySQLAdminUseTLS,
		)
		hook := NewHook(
			client,
			config.Config.ProxySQLWriterHostgroup,
			config.Config.ProxySQLReaderHostgroup,
			config.Config.ProxySQLPreFailoverAction,
		)
		if hook.IsConfigured() {
			log.Infof("ProxySQL hooks enabled: admin=%s:%d writer_hg=%d reader_hg=%d",
				config.Config.ProxySQLAdminAddress,
				config.Config.ProxySQLAdminPort,
				config.Config.ProxySQLWriterHostgroup,
				config.Config.ProxySQLReaderHostgroup,
			)
		} else if config.Config.ProxySQLAdminAddress != "" && config.Config.ProxySQLWriterHostgroup == 0 {
			log.Warningf("ProxySQL: ProxySQLAdminAddress is set but ProxySQLWriterHostgroup is 0 (unconfigured). ProxySQL hooks will be inactive.")
		}
		defaultHook = hook
	})
}

// GetHook returns the global ProxySQL hook.
// Always returns a valid hook — returns a no-op hook if InitHook() has not been called.
func GetHook() *Hook {
	return defaultHook
}
```

- [ ] **Step 2: Add `InitHook()` call to app startup**

In `go/logic/orchestrator.go`, find `go kv.InitKVStores()` (line 588). Add immediately after it:

```go
	go proxysql.InitHook()
```

Add the import: `"github.com/proxysql/orchestrator/go/proxysql"`

In `go/app/cli.go`, find `kv.InitKVStores()` (line 215). Add immediately after it:

```go
	proxysql.InitHook()
```

Add the import: `"github.com/proxysql/orchestrator/go/proxysql"`

- [ ] **Step 3: Verify build succeeds**

```bash
go build -o /dev/null ./go/cmd/orchestrator
```

Expected: BUILD SUCCESS

- [ ] **Step 4: Commit**

```bash
git add go/proxysql/init.go go/app/http.go go/app/cli.go
git commit -m "Initialize ProxySQL hook at application startup"
```

---

### Task 5: Integrate hook into failover flow

**GitHub Issue Title:** Call ProxySQL hooks during master failover

**Files:**
- Modify: `go/logic/topology_recovery.go`

- [ ] **Step 1: Add ProxySQL pre-failover call**

In `go/logic/topology_recovery.go`, in `recoverDeadMaster` (around line 527, after `PreFailoverProcesses` executes successfully), add:

```go
	if err := proxysql.GetHook().PreFailover(failedInstanceKey.Hostname, failedInstanceKey.Port); err != nil {
		log.Errorf("ProxySQL pre-failover failed (non-blocking): %v", err)
		AuditTopologyRecovery(topologyRecovery, fmt.Sprintf("ProxySQL pre-failover failed: %v", err))
		// Non-blocking: we don't abort failover if ProxySQL is unreachable
	}
```

Add the import: `"github.com/proxysql/orchestrator/go/proxysql"`

- [ ] **Step 2: Add ProxySQL post-failover call**

In `recoverDeadMaster`, after the KV pairs are written (around line 948, after `kv.DistributePairs`), add:

```go
		if err := proxysql.GetHook().PostFailover(
			promotedReplica.Key.Hostname, promotedReplica.Key.Port,
			failedInstanceKey.Hostname, failedInstanceKey.Port,
		); err != nil {
			log.Errorf("ProxySQL post-failover failed: %v", err)
			AuditTopologyRecovery(topologyRecovery, fmt.Sprintf("ProxySQL post-failover failed: %v", err))
		}
```

- [ ] **Step 3: Also add to graceful takeover path**

In `go/logic/topology_recovery.go`, find the `GracefulMasterTakeover` function (line ~2089). At line ~2233, just before `executeProcesses(config.Config.PostGracefulTakeoverProcesses, ...)`, add:

```go
	if topologyRecovery.SuccessorKey != nil {
		if err := proxysql.GetHook().PostFailover(
			topologyRecovery.SuccessorKey.Hostname, topologyRecovery.SuccessorKey.Port,
			clusterMaster.Key.Hostname, clusterMaster.Key.Port,
		); err != nil {
			log.Errorf("ProxySQL post-graceful-takeover failed: %v", err)
			AuditTopologyRecovery(topologyRecovery, fmt.Sprintf("ProxySQL post-graceful-takeover failed: %v", err))
		}
	}
```

Here `clusterMaster` is the demoted master and `topologyRecovery.SuccessorKey` is the promoted replica.

- [ ] **Step 4: Verify build succeeds**

```bash
go build -o /dev/null ./go/cmd/orchestrator
```

Expected: BUILD SUCCESS

- [ ] **Step 5: Run existing tests to verify no regression**

```bash
go test ./go/logic/... -v -count=1 2>&1 | tail -20
```

Expected: existing tests still pass (ProxySQL hook is a no-op when unconfigured).

- [ ] **Step 6: Commit**

```bash
git add go/logic/topology_recovery.go
git commit -m "Integrate ProxySQL hooks into master failover and graceful takeover"
```

---

### Task 6: CLI command for ProxySQL testing

**GitHub Issue Title:** Add proxysql-test CLI command

**Files:**
- Modify: `go/app/cli.go`

- [ ] **Step 1: Add CLI command**

In `go/app/cli.go`, in the `Cli` function's switch statement, add a new case in an appropriate section:

```go
	case registerCliCommand("proxysql-test", "ProxySQL", `Test connectivity to ProxySQL Admin interface`):
		{
			proxysql.InitHook()
			hook := proxysql.GetHook()
			if !hook.IsConfigured() {
				log.Fatal("ProxySQL is not configured. Set ProxySQLAdminAddress and ProxySQLWriterHostgroup in config.")
			}
			client := proxysql.NewClient(
				config.Config.ProxySQLAdminAddress,
				config.Config.ProxySQLAdminPort,
				config.Config.ProxySQLAdminUser,
				config.Config.ProxySQLAdminPassword,
				config.Config.ProxySQLAdminUseTLS,
			)
			if err := client.Ping(); err != nil {
				log.Fatale(err)
			}
			fmt.Println("ProxySQL Admin connection: OK")
			fmt.Printf("Writer hostgroup: %d\n", config.Config.ProxySQLWriterHostgroup)
			fmt.Printf("Reader hostgroup: %d\n", config.Config.ProxySQLReaderHostgroup)
		}
	case registerCliCommand("proxysql-servers", "ProxySQL", `Show mysql_servers from ProxySQL`):
		{
			proxysql.InitHook()
			client := proxysql.NewClient(
				config.Config.ProxySQLAdminAddress,
				config.Config.ProxySQLAdminPort,
				config.Config.ProxySQLAdminUser,
				config.Config.ProxySQLAdminPassword,
				config.Config.ProxySQLAdminUseTLS,
			)
			if client == nil {
				log.Fatal("ProxySQL is not configured.")
			}
			rows, db, err := client.Query("SELECT hostgroup_id, hostname, port, status, weight FROM runtime_mysql_servers ORDER BY hostgroup_id, hostname, port")
			if err != nil {
				log.Fatale(err)
			}
			defer db.Close()
			defer rows.Close()
			fmt.Printf("%-12s %-30s %-6s %-15s %-6s\n", "HOSTGROUP", "HOSTNAME", "PORT", "STATUS", "WEIGHT")
			for rows.Next() {
				var hg, port, weight int
				var hostname, status string
				if err := rows.Scan(&hg, &hostname, &port, &status, &weight); err != nil {
					log.Errorf("Error scanning row: %v", err)
					continue
				}
				fmt.Printf("%-12d %-30s %-6d %-15s %-6d\n", hg, hostname, port, status, weight)
			}
		}
```

- [ ] **Step 2: Verify build succeeds**

```bash
go build -o /dev/null ./go/cmd/orchestrator
```

Expected: BUILD SUCCESS

- [ ] **Step 3: Verify command appears in help**

```bash
go run go/cmd/orchestrator/main.go -c help | grep -i proxysql
```

Expected: `proxysql-test` and `proxysql-servers` appear in output.

- [ ] **Step 4: Commit**

```bash
git add go/app/cli.go
git commit -m "Add proxysql-test and proxysql-servers CLI commands"
```

---

### Task 7: Vendor and build verification

**Files:**
- Modify: `vendor/` (if needed)

- [ ] **Step 1: Tidy and vendor**

```bash
go mod tidy
go mod vendor
```

No new dependencies should be needed — `go-sql-driver/mysql` is already vendored.

- [ ] **Step 2: Full build**

```bash
go build -o bin/orchestrator ./go/cmd/orchestrator
```

Expected: BUILD SUCCESS

- [ ] **Step 3: Run full test suite**

```bash
go test ./go/... 2>&1 | tail -30
```

Expected: All tests pass (including new proxysql tests).

- [ ] **Step 4: Commit any vendor changes**

```bash
git add vendor/ go.mod go.sum
git commit -m "Update vendor for ProxySQL hooks" || echo "nothing to commit"
```

---

### Task 8: Documentation

**Files:**
- Create: `docs/proxysql-hooks.md`
- Modify: `docs/configuration.md` (add ProxySQL section reference)

- [ ] **Step 1: Create documentation**

Create `docs/proxysql-hooks.md`:

```markdown
# ProxySQL Failover Hooks

Orchestrator has built-in support for updating [ProxySQL](https://proxysql.com) hostgroups during failover. When configured, orchestrator will automatically:

1. **Before failover:** Drain the old master in ProxySQL (set `OFFLINE_SOFT` or `weight=0`)
2. **After failover:** Update ProxySQL hostgroups to route traffic to the new master

No custom scripts needed — orchestrator + ProxySQL works out of the box.

## Configuration

Add these settings to your `orchestrator.conf.json`:

```json
{
  "ProxySQLAdminAddress": "127.0.0.1",
  "ProxySQLAdminPort": 6032,
  "ProxySQLAdminUser": "admin",
  "ProxySQLAdminPassword": "admin",
  "ProxySQLWriterHostgroup": 10,
  "ProxySQLReaderHostgroup": 20,
  "ProxySQLPreFailoverAction": "offline_soft"
}
```

### Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `ProxySQLAdminAddress` | (empty) | ProxySQL Admin host. Leave empty to disable hooks. |
| `ProxySQLAdminPort` | 6032 | ProxySQL Admin port |
| `ProxySQLAdminUser` | admin | Admin interface username |
| `ProxySQLAdminPassword` | (empty) | Admin interface password |
| `ProxySQLAdminUseTLS` | false | Use TLS for Admin connection |
| `ProxySQLWriterHostgroup` | 0 | Writer hostgroup ID. Must be > 0 to enable hooks. |
| `ProxySQLReaderHostgroup` | 0 | Reader hostgroup ID. Optional. |
| `ProxySQLPreFailoverAction` | offline_soft | Pre-failover action: `offline_soft`, `weight_zero`, or `none` |

## How It Works

### Pre-Failover

When orchestrator detects a dead master and begins recovery:

- **`offline_soft`**: Sets the old master's status to `OFFLINE_SOFT` in ProxySQL. Existing connections are allowed to complete, but no new connections are routed to it.
- **`weight_zero`**: Sets the old master's weight to 0. Similar effect but preserves the server entry's status.
- **`none`**: Skips pre-failover ProxySQL update.

### Post-Failover

After a new master is promoted:

1. Old master is removed from the writer hostgroup
2. New master is added to the writer hostgroup
3. If reader hostgroup is configured: new master is removed from readers
4. `LOAD MYSQL SERVERS TO RUNTIME` is executed to apply changes immediately
5. `SAVE MYSQL SERVERS TO DISK` is executed to persist changes

### Failover Timeline

```
Dead master detected
  → OnFailureDetectionProcesses (scripts)
    → PreFailoverProcesses (scripts)
      → ProxySQL pre-failover: drain old master     ← NEW
        → [topology manipulation: elect new master]
          → KV store updates (Consul/ZK)
            → ProxySQL post-failover: promote new master  ← NEW
              → PostMasterFailoverProcesses (scripts)
                → PostFailoverProcesses (scripts)
```

ProxySQL hooks run **alongside** existing script-based hooks — they don't replace `PreFailoverProcesses` or `PostFailoverProcesses`.

## CLI Commands

### Test connectivity

```bash
orchestrator-client -c proxysql-test
```

### Show ProxySQL server list

```bash
orchestrator-client -c proxysql-servers
```

## Multiple ProxySQL Instances

For ProxySQL Cluster deployments, configure orchestrator to connect to **one** ProxySQL node. Changes propagate automatically across the cluster via ProxySQL's built-in cluster synchronization (`proxysql_servers` table).

If not using ProxySQL Cluster, you can run multiple orchestrator hook configurations by setting up a ProxySQL load balancer, or by using the existing `PostMasterFailoverProcesses` script hooks for additional ProxySQL instances.

## Interaction with Existing Hooks

ProxySQL hooks are **non-blocking** during pre-failover: if ProxySQL is unreachable, the failover proceeds normally. Post-failover errors are logged but do not mark the recovery as failed.

This ensures that a ProxySQL outage never prevents MySQL failover from completing.
```

- [ ] **Step 2: Commit**

```bash
git add docs/proxysql-hooks.md
git commit -m "Add ProxySQL hooks documentation"
```

---

### Task 9: Sample configuration

**Files:**
- Modify: `conf/orchestrator-sample.conf.json` or create `conf/orchestrator-proxysql-sample.conf.json`

- [ ] **Step 1: Create sample config**

Check what exists in `conf/` and create a sample config that includes ProxySQL settings alongside the standard orchestrator settings. Include comments explaining each ProxySQL setting.

- [ ] **Step 2: Commit**

```bash
git add conf/
git commit -m "Add ProxySQL sample configuration"
```

---

### Task 10: Final PR

- [ ] **Step 1: Push and create PR**

```bash
git push -u origin issue30-proxysql-hooks
gh pr create --title "Add built-in ProxySQL failover hooks" --body "Closes #30

## Summary
- New \`go/proxysql/\` package with Admin client and failover hook
- Pre-failover: drain old master in ProxySQL (offline_soft/weight_zero)
- Post-failover: update hostgroups to route to new master
- Configuration: \`ProxySQLAdminAddress\`, \`ProxySQLWriterHostgroup\`, etc.
- CLI: \`proxysql-test\`, \`proxysql-servers\` commands
- Documentation: \`docs/proxysql-hooks.md\`
- Nil-safe: all no-ops when ProxySQL is unconfigured
- Non-blocking: ProxySQL failures don't prevent MySQL failover

## Test plan
- [ ] Unit tests pass: \`go test ./go/proxysql/...\`
- [ ] Full test suite passes: \`go test ./go/...\`
- [ ] Build succeeds: \`go build -o bin/orchestrator ./go/cmd/orchestrator\`
- [ ] CLI commands appear in help output
- [ ] With ProxySQL configured: verify connectivity via \`proxysql-test\`
"
```

---

## Execution Order

```
Task 1 (config) → Task 2 (client) → Task 3 (hook logic) → Task 4 (init) → Task 5 (integration) → Task 6 (CLI) → Task 7 (vendor/build) → Task 8 (docs) → Task 9 (sample config) → Task 10 (PR)
```

All tasks are sequential — each builds on the previous.

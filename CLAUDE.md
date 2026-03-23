# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Orchestrator is a MySQL high availability and replication management tool written in Go. It discovers MySQL replication topologies, enables refactoring (moving replicas between masters), and performs automated or manual failover/recovery. It runs as a service with CLI, HTTP API, and web UI interfaces.

This is Percona's fork of the archived openark/orchestrator, maintained primarily for use in Percona's Kubernetes Operators.

## Build & Test Commands

**Build the binary:**
```bash
./script/build           # builds to bin/orchestrator
# or directly:
go build -o bin/orchestrator go/cmd/orchestrator/main.go
```
Note: requires `gcc` (for SQLite via cgo).

**Run unit tests:**
```bash
go test ./go/...
```

**Run a single package's tests:**
```bash
go test ./go/inst/...
go test ./go/config/...
```

**Run a specific test:**
```bash
go test ./go/inst/... -run TestBinlogCoordinates
```

**Docker-based workflows (via `script/dock`):**
- `script/dock alpine` — build and run orchestrator service
- `script/dock test` — build, run unit tests, integration tests, doc tests
- `script/dock pkg` — build and create distribution packages (.deb/.rpm/.tgz)
- `script/dock system` — full CI environment with MySQL topology, HAProxy, Consul

**Package for release:**
```bash
./build.sh -b            # build only (no packages)
./build.sh               # build + package (requires fpm)
```

## Architecture

All Go source lives under `go/`. The module path is `github.com/openark/orchestrator`.

### Key packages

- **`go/cmd/orchestrator/`** — Entry point (`main.go`). Parses CLI flags and delegates to `go/app`.
- **`go/app/`** — Application bootstrap. `http.go` starts the HTTP service; `cli.go` handles CLI commands.
- **`go/inst/`** — Core domain model. MySQL instance representation, replication analysis, binlog coordinates, GTID sets, topology operations, and their DAO (database access) counterparts (`*_dao.go` files). This is the largest and most critical package.
- **`go/logic/`** — Orchestration logic. `orchestrator.go` is the main loop. `topology_recovery.go` and `topology_recovery_dao.go` handle failover/recovery. `command_applier.go` applies topology commands.
- **`go/config/`** — Configuration loading from JSON. Single `Config` struct with extensive options.
- **`go/http/`** — HTTP API (`api.go`) and web UI (`web.go`). Uses the Martini framework. `agents_api.go` for agent endpoints.
- **`go/db/`** — Database abstraction layer. Supports both MySQL and SQLite as orchestrator's backend store. `generate_base.go` and `generate_patches.go` handle schema migrations.
- **`go/discovery/`** — Instance discovery queue and aggregation logic.
- **`go/process/`** — Health checks, leader election, access tokens for the orchestrator process itself.
- **`go/raft/`** — Raft consensus implementation for multi-node orchestrator HA (uses a fork of hashicorp/raft).
- **`go/kv/`** — Key-value store integrations (Consul, ZooKeeper) used for master discovery publishing.
- **`go/ssl/`** — TLS configuration helpers.
- **`go/os/`** — OS-level process execution utilities.
- **`go/collection/`** — Generic collection utilities (e.g., expiring collections).
- **`go/golib/`** — Local fork of openark/golib (replaced via `go.mod`), provides logging and SQL utilities.

### Patterns

- **DAO pattern**: Domain types live alongside their database access code in the same package. Files named `*_dao.go` contain SQL queries and database interaction. The domain structs and logic are in the corresponding non-DAO files.
- **Backend flexibility**: Orchestrator can use either MySQL or SQLite as its backend database. The `go/db/` package abstracts this.
- **Raft consensus**: For HA deployments, orchestrator nodes form a Raft cluster. The `go/raft/` package wraps the hashicorp/raft library with custom FSM and snapshot logic.
- **Configuration**: A single large `Config` struct in `go/config/config.go` holds all settings, loaded from a JSON config file.

### Web resources

- **`resources/`** — Static web assets (HTML templates, JS, CSS, images) served by the web UI.
- **`conf/`** — Sample configuration files (`orchestrator-sample*.conf.json`).

# Raft Upstream Migration Evaluation

**Date:** 2026-03-24
**Issue:** #56
**Status:** Complete

## Executive Summary

Orchestrator depends on `openark/raft`, a 2017-era fork of `hashicorp/raft` pinned at commit `fba9f909f7fe` (September 2017). The fork diverges significantly from upstream `hashicorp/raft` v1.7+, which has undergone nine years of active development including security fixes, performance improvements, and major API changes.

**Recommendation:** Migrate to upstream `hashicorp/raft` v1.7.x. The migration is moderate effort (estimated 3-5 days of focused work) and eliminates ongoing security and maintenance risk from running unmaintained consensus code.

---

## 1. Current State

### Replace Directive (go.mod)

```
replace github.com/hashicorp/raft => github.com/openark/raft v0.0.0-20170918052300-fba9f909f7fe
```

This redirects all `github.com/hashicorp/raft` imports to a fork from September 18, 2017. The `go.mod` declares `github.com/hashicorp/raft v1.7.3` as the desired version, but the replace directive overrides it entirely.

### Files Using the Raft API

| File | Purpose |
|------|---------|
| `go/raft/store.go` | Raft initialization, peer management, command application |
| `go/raft/raft.go` | High-level orchestrator raft operations (leader checks, snapshots, yield) |
| `go/raft/fsm.go` | Finite state machine (Apply, Snapshot, Restore) |
| `go/raft/fsm_snapshot.go` | Snapshot persistence |
| `go/raft/rel_store.go` | SQLite-backed LogStore and StableStore |
| `go/raft/file_snapshot.go` | Custom file-based SnapshotStore |
| `go/raft/http_client.go` | HTTP client for leader communication (not raft API) |
| `go/raft/applier.go` | CommandApplier interface (no raft API) |
| `go/raft/snapshot.go` | SnapshotCreatorApplier interface (no raft API) |

---

## 2. Divergent API Catalog

### 2.1 Removed APIs (do not exist in upstream v1.7+)

#### `raft.PeerStore` interface and `raft.StaticPeers`

- **Location:** `store.go:22` (field type), `store.go:74` (instantiation), `store.go:75` (`SetPeers`)
- **What it does:** The old API used a `PeerStore` interface to track cluster membership. `StaticPeers` was a simple in-memory implementation.
- **Upstream replacement:** Upstream replaced `PeerStore` with the `Configuration` / `ConfigurationStore` system. Peer management is now handled through `raft.Configuration` containing `Server` entries. For bootstrap, use `raft.BootstrapCluster()`. At runtime, query `raft.GetConfiguration()`.
- **Migration effort:** **Moderate.** Must replace `PeerStore` field with configuration-based peer tracking. The `GetPeers()` function (called from `raft.go:299`) must be reimplemented using `raft.GetConfiguration()`.

#### `raft.AddUniquePeer()`

- **Location:** `store.go:69`
- **What it does:** Helper to add a peer to a string slice if not already present.
- **Upstream replacement:** No direct replacement needed. This is a trivial utility; replace with a local helper or inline dedup logic.
- **Migration effort:** **Trivial.**

#### `config.EnableSingleNode`

- **Location:** `store.go:83`
- **What it does:** Allows a single node to self-elect as leader without any peers.
- **Upstream replacement:** Use `raft.BootstrapCluster()` to initialize a single-node cluster. This is a one-time operation checked against existing state.
- **Migration effort:** **Moderate.** Need to add bootstrap logic that runs conditionally on first startup.

#### `config.DisableBootstrapAfterElect`

- **Location:** `store.go:84`
- **What it does:** Controls whether bootstrap info is retained after initial election.
- **Upstream replacement:** Not needed. Upstream `BootstrapCluster()` is inherently a one-time operation.
- **Migration effort:** **Trivial.** Simply remove.

#### `raft.NewRaft()` 7-argument constructor

- **Location:** `store.go:111`
- **Signature used:** `NewRaft(config, fsm, logStore, stableStore, snapshotStore, peerStore, transport)`
- **Upstream signature:** `NewRaft(config, fsm, logStore, stableStore, snapshotStore, transport)` (6 arguments, no peerStore)
- **What changed:** The `peerStore` argument was removed. Peer/membership information is now managed through `Configuration`.
- **Migration effort:** **Trivial** once PeerStore is removed. Drop the `peerStore` argument.

#### `raft.Raft.AddPeer()` / `raft.Raft.RemovePeer()`

- **Location:** `store.go:125` (AddPeer), `store.go:137` (RemovePeer)
- **What they do:** Add or remove a peer from the cluster using the old string-based peer identity.
- **Upstream replacement:** `raft.Raft.AddVoter(id, address, prevIndex, timeout)` and `raft.Raft.RemoveServer(id, prevIndex, timeout)`. The new API uses `ServerID` + `ServerAddress` instead of a single string, and requires an index parameter for consistency.
- **Migration effort:** **Moderate.** Must decide on a ServerID scheme (e.g., use address as ID, or introduce a separate ID). Update both `AddPeer` and `RemovePeer` in `store.go`, plus all callers.

#### `raft.Raft.Yield()`

- **Location:** `raft.go:284`
- **What it does:** Causes the leader to voluntarily step down in favor of a specific peer. This is a custom addition in the openark fork, not present in standard hashicorp/raft.
- **Upstream replacement:** `raft.Raft.LeadershipTransfer()` (added in upstream) transfers leadership but does not target a specific peer. Upstream also has `LeadershipTransferToServer(id, address)` for targeted transfer.
- **Migration effort:** **Moderate.** `Yield()` is called from `raft.go:284` and from `fsm.go:65` (via the `yield` and `yieldByHint` FSM commands). Must map the yield/yield-hint logic to `LeadershipTransfer()` or `LeadershipTransferToServer()`. The semantics are slightly different: `Yield()` was a local step-down, while `LeadershipTransfer` is a coordinated handoff.

#### `raft.Raft.StepDown()`

- **Location:** `raft.go:277`
- **What it does:** Forces this node to step down from leadership.
- **Upstream replacement:** Not present as a public method in upstream. However, `LeadershipTransfer()` achieves a similar result. Alternatively, since this is only called in one place, it may be replaceable with `LeadershipTransfer()`.
- **Migration effort:** **Moderate.** Need to evaluate whether `LeadershipTransfer()` is an acceptable substitute or if a different approach is needed.

#### `raft.Raft.Leader()` returning `string`

- **Location:** `raft.go:235`
- **What it does:** Returns the address of the current leader as a plain string.
- **Upstream replacement:** `raft.Raft.Leader()` in upstream returns `raft.ServerAddress` (a typed string). Additionally, upstream provides `LeaderWithID()` returning both `ServerAddress` and `ServerID`.
- **Migration effort:** **Trivial.** `ServerAddress` is a `string` typedef; a simple type conversion suffices.

#### `raft.Raft.LeaderCh()`

- **Location:** `raft.go:154`
- **What it does:** Returns a channel that signals leadership changes.
- **Upstream status:** `LeaderCh()` still exists in upstream v1.7.x.
- **Migration effort:** **None.** API is compatible.

### 2.2 Compatible APIs (exist in both fork and upstream)

| API Call | Location | Status |
|----------|----------|--------|
| `raft.DefaultConfig()` | `store.go:48` | Compatible |
| `raft.NewTCPTransport()` | `store.go:60` | Compatible |
| `raft.Raft.Apply()` | `store.go:157` | Compatible |
| `raft.Raft.State()` | `raft.go:248`, `store.go:148` | Compatible |
| `raft.Raft.Snapshot()` | `raft.go:264` | Compatible |
| `raft.Raft.LeaderCh()` | `raft.go:154` | Compatible |
| `raft.Leader` / `raft.Follower` / `raft.Candidate` (state constants) | `raft.go:222,227,249,260` | Compatible |
| `raft.RaftState` type | `raft.go:247` | Compatible |
| `raft.Log` struct | `fsm.go:33`, `rel_store.go:179,196,201` | Compatible |
| `raft.FSMSnapshot` interface | `fsm.go:84` | Compatible |
| `raft.SnapshotSink` interface | `fsm_snapshot.go:35` | Compatible |
| `raft.SnapshotMeta` struct | `file_snapshot.go:55,137,154,191,199,273` | Compatible (but `Peers` field changed to `Configuration`) |
| `raft.ErrLogNotFound` | `rel_store.go:190` | Compatible |
| `config.SnapshotThreshold` | `store.go:49` | Compatible |
| `config.SnapshotInterval` | `store.go:50` | Compatible |
| `config.ShutdownOnRemove` | `store.go:51` | Compatible |

### 2.3 Custom Implementations (orchestrator code, not raft API)

| Component | Assessment |
|-----------|------------|
| `RelationalStore` (LogStore + StableStore) | Implements standard `raft.LogStore` and `raft.StableStore` interfaces. **Fully compatible** with upstream -- these interfaces have not changed. |
| `FileSnapshotStore` | Implements `raft.SnapshotStore` interface. **Mostly compatible** but the `Create()` method signature changed. The old `Create(index, term uint64, peers []byte)` now expects `Create(version raft.SnapshotVersion, index, term uint64, configuration raft.Configuration, configurationIndex uint64, trans raft.Transport)` in upstream. This is a **significant** change. |
| `fsm` (FSM implementation) | Implements `raft.FSM` interface. The `Apply(*raft.Log) interface{}` signature is compatible. |
| `fsmSnapshot` | Implements `raft.FSMSnapshot` interface. Compatible. |

### 2.4 SnapshotStore.Create() Signature Change (Critical)

The most impactful API change is in the `SnapshotStore` interface:

**Old (openark fork):**
```go
Create(index, term uint64, peers []byte) (SnapshotSink, error)
```

**Upstream v1.7+:**
```go
Create(version SnapshotVersion, index, term uint64, configuration Configuration, configurationIndex uint64, trans Transport) (SnapshotSink, error)
```

The custom `FileSnapshotStore` in `go/raft/file_snapshot.go` must be updated to match this new signature. The `peers []byte` parameter has been replaced by a structured `Configuration` object. The snapshot metadata (`SnapshotMeta.Peers`) field has similarly been replaced by `SnapshotMeta.Configuration` and `SnapshotMeta.ConfigurationIndex`.

**Migration effort:** **Significant.** The entire `FileSnapshotStore` needs to be updated, including `Create()`, metadata serialization, and the `SnapshotMeta` embedding. Alternatively, upstream provides `raft.NewFileSnapshotStore()` which could replace the custom implementation entirely.

---

## 3. Security Risk Assessment

### Staying on the Fork

| Risk | Severity | Details |
|------|----------|---------|
| No upstream security patches | **High** | Nine years of hashicorp/raft security fixes are missing. Any CVEs discovered in raft consensus logic, transport, or snapshot handling are unpatched. |
| No upstream bug fixes | **Medium** | Known correctness issues fixed upstream (e.g., split-brain edge cases, snapshot race conditions) remain present. |
| Dependency chain risk | **Medium** | The fork depends on old versions of `hashicorp/go-msgpack` and other libraries that may have their own vulnerabilities. |
| Go compatibility | **Low-Medium** | The 2017 code may develop subtle incompatibilities with newer Go versions over time (e.g., changes in `sync`, `net`, or `crypto` packages). |
| Supply chain audit | **Medium** | Security auditors reviewing the project will flag an unmaintained, nine-year-old fork of critical consensus infrastructure. |

### Known Upstream Improvements Since 2017

- Pre-vote protocol (reduces disruption from network-partitioned nodes)
- Improved snapshot handling and transfer
- Better leadership transfer support
- Non-voter (staging) server support
- Batch Apply support for FSM
- Configurable trailing logs
- Numerous race condition and deadlock fixes
- Modern Go module support

---

## 4. Migration Effort Estimate

### Task Breakdown

| Task | Effort | Complexity |
|------|--------|------------|
| Remove `PeerStore` / `StaticPeers`, implement `Configuration`-based peer tracking | 4h | Moderate |
| Replace `EnableSingleNode` with `BootstrapCluster()` | 2h | Moderate |
| Update `NewRaft()` call (drop peerStore arg) | 0.5h | Trivial |
| Replace `AddPeer()`/`RemovePeer()` with `AddVoter()`/`RemoveServer()` | 3h | Moderate |
| Replace `Yield()`/`StepDown()` with `LeadershipTransfer()` | 3h | Moderate |
| Update `Leader()` return type handling | 0.5h | Trivial |
| Update `FileSnapshotStore.Create()` signature or replace with upstream snapshot store | 4h | Significant |
| Update `SnapshotMeta.Peers` to `SnapshotMeta.Configuration` | 2h | Moderate |
| Remove `raft.AddUniquePeer()` usage | 0.5h | Trivial |
| Remove `replace` directive in `go.mod`, resolve dependency changes | 1h | Trivial |
| Testing: unit tests for raft package | 4h | Moderate |
| Testing: integration test with multi-node raft cluster | 6h | Significant |
| Snapshot migration: handle existing snapshot files from old format | 2h | Moderate |

### Total Estimate

**~32 hours (3-5 working days)** for an engineer familiar with the codebase.

### Risk Factors

- **Snapshot format migration:** Existing deployments have snapshots in the old format. A migration path or clean-start procedure is needed.
- **Behavioral differences:** Upstream raft may have subtly different timing, election, and snapshot behavior. Thorough integration testing is essential.
- **ServerID scheme:** Upstream requires both a `ServerID` and `ServerAddress` for each node. The simplest approach is to use the address as the ID (which upstream supports), but this should be a deliberate design choice.

---

## 5. Options Analysis

### Option A: Stay on openark/raft Fork (Status Quo)

| Pros | Cons |
|------|------|
| Zero effort | No security patches for 9 years |
| No risk of regression | No bug fixes from upstream |
| Known working behavior | Growing API incompatibility with Go ecosystem |
| | Blocks adoption of upstream raft features |
| | Audit/compliance risk |

### Option B: Migrate to Upstream hashicorp/raft v1.7+

| Pros | Cons |
|------|------|
| Security patches and ongoing maintenance | 3-5 days migration effort |
| Modern features (pre-vote, leadership transfer, batching) | Risk of behavioral regression |
| Standard API compatible with ecosystem | Snapshot format migration needed |
| Reduced maintenance burden long-term | |
| Better documentation and community support | |

### Option C: Fork to proxysql/raft

| Pros | Cons |
|------|------|
| Can selectively backport security fixes | Ongoing maintenance burden |
| Minimal code changes needed | Must track upstream CVEs manually |
| Preserves current API compatibility | Still divergent from ecosystem |
| | Two custom forks to maintain (golib + raft) |

---

## 6. Recommendation

**Migrate to upstream `hashicorp/raft` v1.7.x (Option B).**

Rationale:

1. **Security is the primary driver.** Running consensus infrastructure from a nine-year-old unmaintained fork is an unacceptable risk for production MySQL HA deployments, especially in Percona's Kubernetes Operators context.

2. **The effort is bounded.** The divergent APIs are well-cataloged and the migration path for each is clear. The custom `RelationalStore` (LogStore/StableStore) is fully compatible and requires no changes.

3. **Option C (own fork) creates ongoing burden** without addressing the root problem -- we would need to continuously monitor and backport upstream fixes, which over time costs more than a one-time migration.

4. **The custom `FileSnapshotStore` could potentially be replaced** with upstream's built-in `FileSnapshotStore`, further reducing custom code. The custom version was likely created because of minor behavioral differences in 2017 that have since been addressed upstream.

### Suggested Migration Order

1. Remove the `replace` directive, fix compilation errors
2. Update `NewRaft()` constructor and `Config` fields
3. Replace `PeerStore` with `Configuration`-based approach
4. Replace `AddPeer`/`RemovePeer` with `AddVoter`/`RemoveServer`
5. Replace `Yield`/`StepDown` with `LeadershipTransfer`
6. Update or replace `FileSnapshotStore`
7. Integration testing with multi-node cluster
8. Document snapshot migration procedure for existing deployments

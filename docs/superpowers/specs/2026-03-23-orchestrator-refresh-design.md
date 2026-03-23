# Orchestrator Refresh — ProxySQL LLC Takeover

**Date:** 2026-03-23
**Author:** René (ProxySQL founder/author)
**Status:** Approved

## Context

Orchestrator is a MySQL HA and replication management tool originally authored by Shlomi Noach, maintained at various points by Outbrain, GitHub, and Percona. The project has stagnated — Percona maintains it narrowly for their Kubernetes Operators and is not entertaining external enhancements.

ProxySQL heavily relies on orchestrator for MySQL failover and topology management. ProxySQL LLC is taking over maintainership to:

1. **Signal life:** Orchestrator is not dead. ProxySQL is committed to supporting it.
2. **Modernize:** Update dependencies, fix CI, improve code quality and contributor experience.
3. **Evolve:** Build native ProxySQL integration, and eventually support PostgreSQL.

This repo (`proxysql/orchestrator`) is a fork from `percona/orchestrator`. We will periodically pull fixes from Percona (track upstream — cherry-pick relevant fixes as needed, reviewed by maintainers) but ProxySQL drives direction.

The target community is open-source contributors. Governance, contributor experience, and visibility are first-class concerns from day one.

**Breaking changes in 4.0:** The Go module path changes from `github.com/openark/orchestrator` to `github.com/proxysql/orchestrator`. Anyone importing orchestrator as a Go library must update imports. The CLI, HTTP API, and configuration format remain backward-compatible — existing deployments can upgrade by changing the binary only.

## Approach: Phased Rollout

### Phase 1 — Identity & Trust (days)

**Goal:** Anyone landing on the repo immediately sees "ProxySQL LLC maintains this. It's alive. Come contribute."

**Deliverable:** Tag `4.0.0-rc1`.

#### 1.1 README overhaul

- Replace Percona fork notice with ProxySQL LLC maintainer statement
- Brief lineage: Outbrain -> GitHub -> openark -> Percona -> ProxySQL LLC
- Value proposition: orchestrator + ProxySQL for MySQL HA
- Mention future vision (ProxySQL-native integration, PostgreSQL exploration)
- Update all badge URLs to `proxysql/orchestrator`
- Link to new governance files

#### 1.2 LICENSE and copyright

- Add "Copyright ProxySQL LLC" to the LICENSE file alongside existing copyright notices (Apache 2.0 permits this)
- Do not remove existing copyright attributions (Outbrain, GitHub, openark, Percona)

#### 1.3 Governance files (repo root)

- **CONTRIBUTING.md** — How to file issues, submit PRs, coding standards (gofmt, tests required), DCO sign-off (lightweight, same as Linux kernel)
- **CODE_OF_CONDUCT.md** — Contributor Covenant (industry standard)
- **SECURITY.md** — Vulnerability reporting process (security@proxysql.com or similar)
- **MAINTAINERS.md** — René + designated maintainers with roles

#### 1.4 GitHub templates refresh

- Convert `.github/ISSUE_TEMPLATE.md` to multiple templates (bug report, feature request, question)
- Update `.github/PULL_REQUEST_TEMPLATE.md` — reference CONTRIBUTING.md, add checklist
- Add `.github/FUNDING.yml` if applicable

#### 1.5 Fix CI (critical)

GitHub Actions workflows reference Go 1.16, but `go.mod` requires Go 1.25.7. All three workflows must be updated:

- `.github/workflows/main.yml` — CI (build, unit tests, integration tests, doc tests)
- `.github/workflows/system.yml` — system tests (MySQL topology, HAProxy, Consul)
- `.github/workflows/upgrade.yml` — upgrade path testing

Also update deprecated GitHub Actions (`actions/checkout@v2` → `v4`, `actions/setup-go@v1` → `v5`).

**Minimum bar for Phase 1:** all three workflows run green with Go 1.25.7. New CI capabilities (linting, caching, matrix testing) are deferred to Phase 2.4.

CI must pass on the current codebase before tagging anything.

#### 1.6 Docs references update

- Bulk-update `docs/` links from `openark/orchestrator` and `percona/orchestrator` to `proxysql/orchestrator`
- Update table of contents and developer docs

#### 1.7 Tag 4.0.0-rc1

- Update `RELEASE_VERSION` to `4.0.0-rc1`
- Create GitHub Release with notes explaining new maintainership

---

### Phase 2 — Technical Foundation (weeks)

**Goal:** Modern, secure, contributor-ready codebase. **Deliverable:** Tag `4.0.0` stable.

#### 2.1 Go module path migration

- `github.com/openark/orchestrator` -> `github.com/proxysql/orchestrator` across all Go files, `go.mod`, and imports
- Touches nearly every `.go` file — one large, mechanical PR
- The local `go/golib` directory (currently `replace github.com/openark/golib => ./go/golib`) becomes `replace github.com/proxysql/orchestrator/go/golib => ./go/golib` — it stays as a local subdirectory, no separate repo needed
- Full test suite verification
- **Pre-requisite:** Confirm GitHub repo settings at `github.com/proxysql/orchestrator` support Go module resolution (`go get`). If the repo is still marked as a GitHub fork, the Go module proxy may not resolve correctly — may need to detach fork relationship.

#### 2.2 Dependency updates

- Audit all dependencies for CVEs (`govulncheck`)
- Update direct dependencies to latest stable versions
- Evaluate the `openark/raft` fork — this is a 2017-era fork with ~9 years of divergence from upstream `hashicorp/raft`. This is a significant investigation: determine if upstream can be used, or if the fork needs to move to `proxysql/raft`. If the evaluation proves too complex, defer to Phase 3.
- Re-vendor after updates

#### 2.3 Code quality tooling

- Add `staticcheck` and `golangci-lint` to CI
- Fix issues they surface
- Add root-level `Makefile` (build, test, lint, fmt) — contributors shouldn't need to read scripts

#### 2.4 CI modernization

- Add dependency caching for faster builds
- Add CVE scanning as required CI check
- Add golangci-lint as CI step
- Consider matrix testing across Go versions (current + previous)

#### 2.5 Go-martini assessment

Decision: **keep for 4.0.0**. Martini is unmaintained since 2017 but functional. The module rename is disruptive enough — defer framework replacement to Phase 3 when the API is being reworked anyway.

#### 2.6 Tag 4.0.0 stable

- Update `RELEASE_VERSION` to `4.0.0`
- Full release notes covering everything since 3.2.6
- Build and publish packages:
  - Docker images to GitHub Container Registry (`ghcr.io/proxysql/orchestrator`)
  - deb/rpm/tar as GitHub Release assets
  - Consider Docker Hub mirror for discoverability

---

### Phase 3 — Feature Direction (ongoing)

**Goal:** Evolve orchestrator toward ProxySQL-native integration and lay groundwork for multi-database support. Community-driven — each item is independently contributable.

#### 3.1 Official ProxySQL hooks

- Built-in pre/post failover hooks that notify ProxySQL via its Admin API
- No custom scripts needed — "orchestrator + ProxySQL works out of the box"
- Clear documentation with examples

#### 3.2 ProxySQL-aware topology

- Orchestrator queries ProxySQL for hostgroup configuration
- During failover, orchestrator updates ProxySQL hostgroups directly (promote new master, demote old)
- Optional: read ProxySQL query routing stats to inform failover decisions

#### 3.3 Database abstraction layer

- Extract MySQL-specific logic from `go/inst/` into a database-provider interface
- Core concepts (topology discovery, failure detection, recovery orchestration) become database-agnostic
- MySQL becomes the first "provider"
- Architectural prerequisite for future PostgreSQL support (streaming replication, Patroni-style failover)

#### 3.4 API modernization

- Versioned REST API (`/api/v2/`) alongside existing
- Structured JSON responses with consistent error handling
- OpenAPI/Swagger spec for programmatic integration

#### 3.5 Observability

- Prometheus metrics endpoint (replace/supplement existing go-metrics/Graphite)
- Structured logging (JSON) as option alongside text logs
- Health check endpoints for Kubernetes probes

#### 3.6 Replace go-martini

- Migrate web framework to maintained alternative (`chi`, `gin`, or stdlib `net/http`)
- Timed with API rework to minimize churn

## Current Technical State (for reference)

- **Version:** 3.2.6
- **Go:** 1.25.7 (go.mod/Docker), 1.16 (CI workflows — broken mismatch)
- **Module path:** `github.com/openark/orchestrator`
- **Web framework:** go-martini (unmaintained since 2017)
- **Raft:** custom fork of hashicorp/raft from openark era
- **Backend:** MySQL or SQLite
- **CI:** GitHub Actions — unit, integration, system, upgrade, CVE tests
- **Docs:** 60+ markdown files, all referencing openark/percona
- **Vendoring:** full vendor directory
- **License:** Apache 2.0

## Workflow

All changes follow a GitHub issue + pull request + review workflow. Each Phase 1 item gets its own issue. Phase 2 and 3 items are filed as issues when work begins on that phase.

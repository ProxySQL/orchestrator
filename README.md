![](https://github.com/proxysql/orchestrator/workflows/CI/badge.svg)
![](https://github.com/proxysql/orchestrator/workflows/upgrade/badge.svg)
![](https://github.com/proxysql/orchestrator/workflows/system%20tests/badge.svg)
[![downloads](https://img.shields.io/github/downloads/proxysql/orchestrator/total.svg)](https://github.com/proxysql/orchestrator/releases) [![release](https://img.shields.io/github/release/proxysql/orchestrator.svg)](https://github.com/proxysql/orchestrator/releases)

> **Maintained by [ProxySQL LLC](https://proxysql.com).** Orchestrator is actively maintained and open to contributions. We believe in orchestrator's potential as the go-to MySQL HA tool, especially when paired with ProxySQL. Bug reports, feature requests, and pull requests are welcome — see [CONTRIBUTING.md](CONTRIBUTING.md) to get started.

# orchestrator [[Documentation]](https://github.com/proxysql/orchestrator/tree/master/docs)

![Orchestrator logo](docs/images/orchestrator-logo-wide.png)

`orchestrator` is a MySQL high availability and replication management tool, runs as a service and provides command line access, HTTP API and Web interface. `orchestrator` supports:

#### Discovery

`orchestrator` actively crawls through your topologies and maps them. It reads basic MySQL info such as replication status and configuration.

It provides you with slick visualization of your topologies, including replication problems, even in the face of failures.

#### Refactoring

`orchestrator` understands replication rules. It knows about binlog file:position, GTID, Pseudo GTID, Binlog Servers.

Refactoring replication topologies can be a matter of drag & drop a replica under another master. Moving replicas around is safe: `orchestrator` will reject an illegal refactoring attempt.

Fine-grained control is achieved by various command line options.

#### Recovery

`orchestrator` uses a holistic approach to detect master and intermediate master failures. Based on information gained from the topology itself, it recognizes a variety of failure scenarios.

Configurable, it may choose to perform automated recovery (or allow the user to choose type of manual recovery). Intermediate master recovery achieved internally to `orchestrator`. Master failover supported by pre/post failure hooks.

Recovery process utilizes _orchestrator's_ understanding of the topology and of its ability to perform refactoring. It is based on _state_ as opposed to _configuration_: `orchestrator` picks the best recovery method by investigating/evaluating the topology at the time of
recovery itself.

#### The interface

`orchestrator` supports:

- Command line interface (love your debug messages, take control of automated scripting)
- Web API (HTTP GET access)
- Web interface, a _slick_ one.

![Orcehstrator screenshot](docs/images/orchestrator-topology-8-screenshot.png)

#### ProxySQL Integration

Built-in failover hooks that update [ProxySQL](https://proxysql.com) hostgroups automatically — no custom scripts needed:

- **Pre-failover:** Drains the old master in ProxySQL (OFFLINE_SOFT)
- **Post-failover:** Updates writer/reader hostgroups to route traffic to the new master
- **Topology API:** Query ProxySQL server configuration via `/api/proxysql/servers`
- **CLI tools:** `proxysql-test` and `proxysql-servers` commands

See [ProxySQL hooks documentation](docs/proxysql-hooks.md).

#### Observability

- **Prometheus metrics** at `/metrics` — discovery, replication, recovery, and cluster gauges
- **Kubernetes health endpoints** — `/health/live`, `/health/ready`, `/health/leader`
- **Structured API** — versioned `/api/v2/` with JSON envelopes and proper HTTP status codes

See [Observability documentation](docs/observability.md) and [API v2 documentation](docs/api-v2.md).

#### Additional perks

- Highly available (shared backend or Raft consensus)
- Controlled master takeovers
- Manual and automated failovers with full audit trail
- Pseudo-GTID and Oracle GTID support
- Datacenter/physical location awareness
- Database provider abstraction (MySQL, PostgreSQL foundation)
- HTTP security/authentication methods

Read the [Orchestrator documentation](https://github.com/proxysql/orchestrator/tree/master/docs)

#### Lineage

Authored by [Shlomi Noach](https://github.com/shlomi-noach):

- 2014 at [Outbrain](http://outbrain.com) as https://github.com/outbrain/orchestrator
- 2015 at [Booking.com](http://booking.com) as https://github.com/outbrain/orchestrator
- 2016-2020 at [GitHub](http://github.com) as https://github.com/github/orchestrator
- 2020- as https://github.com/openark/orchestrator

Maintained by [Percona](https://percona.com) as https://github.com/percona/orchestrator

Maintained since 2026 by [ProxySQL LLC](https://proxysql.com) as https://github.com/proxysql/orchestrator

#### Community

- [Contributing Guide](CONTRIBUTING.md) — how to file issues, submit PRs, and coding standards
- [Code of Conduct](CODE_OF_CONDUCT.md) — expected behavior in the community
- [Security Policy](SECURITY.md) — how to report vulnerabilities
- [Maintainers](MAINTAINERS.md) — current project maintainers

#### Related projects

- Orchestrator Puppet module: https://github.com/github/puppet-orchestrator-for-mysql
- Orchestrator Chef Cookbook (1): https://github.com/silviabotros/chef-orchestrator
- Orchestrator Chef Cookbook (2): https://supermarket.chef.io/cookbooks/orchestrator
- Nagios / Icinga check based on Orchestrator API: https://github.com/mcrauwel/go-check-orchestrator
- Light Python wrapper for Orchestrator API: https://github.com/stirlab/python-mysql-orchestrator

#### Quick Start

```bash
# Build
go build -o bin/orchestrator ./go/cmd/orchestrator

# Start with SQLite backend
bin/orchestrator -config conf/orchestrator-sample.conf.json http

# Discover your MySQL topology
curl http://localhost:3000/api/discover/your-master-host/3306

# Open the web UI
open http://localhost:3000
```

See the [Quick Start Guide](docs/quickstart.md) for a complete 5-minute walkthrough.

#### Developers

Get started developing Orchestrator by [reading the developer docs](/docs/developers.md). Common commands:

```bash
make build      # Build binary
make test       # Run unit tests
make lint       # Run golangci-lint
make fmt        # Format code
```

#### License

`orchestrator` is free and open sourced under the [Apache 2.0 license](LICENSE).

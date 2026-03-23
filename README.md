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

#### Additional perks

- Highly available
- Controlled master takeovers
- Manual failovers
- Failover auditing
- Audited operations
- Pseudo-GTID
- Datacenter/physical location awareness
- MySQL-Pool association
- HTTP security/authentication methods
- More...

#### Future Vision

- **ProxySQL-native integration** — built-in hooks and topology awareness for seamless orchestrator + ProxySQL HA workflows, no custom scripts needed.
- **PostgreSQL exploration** — a database-provider abstraction layer to support PostgreSQL streaming replication alongside MySQL.

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

#### Developers

Get started developing Orchestrator by [reading the developer docs](/docs/developers.md). Thanks for your interest!

#### License

`orchestrator` is free and open sourced under the [Apache 2.0 license](LICENSE).

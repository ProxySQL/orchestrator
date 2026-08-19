# Consolidated UI browser acceptance — 2026-08-19

The rebuilt Linux/arm64 Orchestrator service was tested in the Codex in-app Browser against the three-node functional lab. Every route was explicitly reloaded after the rebuild. Browser warning/error counts are new entries observed during each route check.

| Route | Viewport | State | Navigation | Interaction | Body overflow | Console |
|---|---:|---|---|---|---|---|
| `/web/clusters` | 1440×900 | 1 cluster | Expanded | Cluster link visible | none (1440/1440) | 0 |
| `/web/cluster/mysql1:3306` | 1440×900 | 3 cards, 3 nodes, 2 links | Expanded | Collapse/expand, View, Details, dismiss | none (1440/1440) | 0 |
| `/web/clusters-analysis` | 1440×900 | ErrantGTIDStructureWarning | Expanded | Incident visible | none (1440/1440) | 0 |
| `/web/discover` | 1440×900 | Form ready | Expanded | Inputs usable | none (1440/1440) | 0 |
| `/web/search/mysql1` | 1440×900 | Populated | Expanded | Results visible | none (1440/1440) | 0 |
| `/web/search/no-such-instance` | 1440×900 | Empty | Expanded | Empty state visible | none (1440/1440) | 0 |
| `/web/audit` | 1440×900 | 20 rows | Expanded | Next `/1`, Previous `/0` | none (1440/1440) | 0 |
| `/web/audit-failure-detection` | 1440×900 | Empty | Expanded | Empty state visible | none (1440/1440) | 0 |
| `/web/audit-recovery` | 1440×900 | Empty | Expanded | Empty state visible | none (1440/1440) | 0 |
| `/web/audit-recovery/id/999999` | 1440×900 | Empty detail | Expanded | Not-found state visible | none (1440/1440) | 0 |
| `/web/status` | 1440×900 | Populated | Expanded | Status panels visible | none (1440/1440) | 0 |
| `/web/about` | 1440×900 | Populated | Expanded | ProxySQL links/content visible | none (1440/1440) | 0 |
| `/web/faq` | 1440×900 | Populated | Expanded | Documentation content visible | none (1440/1440) | 0 |
| `/web/agents` | 1440×900 | Disabled | Expanded | Disabled state visible | none (1440/1440) | 0 |
| `/web/seeds` | 1440×900 | Disabled | Expanded | Disabled state visible | none (1440/1440) | 0 |
| `/web/clusters` | 794×900 | 1 cluster | Collapsed; toggler visible | Toggler and Home open once | none (794/794) | 0 |
| `/web/cluster/mysql1:3306` | 794×900 | 3 cards, 3 nodes, 2 links | Collapsed | Collapse/expand, View, Details | none (794/794) | 0 |
| `/web/clusters-analysis` | 794×900 | ErrantGTIDStructureWarning | Collapsed | Incident visible | none (794/794) | 0 |
| `/web/discover` | 794×900 | Form ready | Collapsed | Inputs usable | none (794/794) | 0 |
| `/web/search/mysql1` | 794×900 | Populated | Collapsed | Results visible | none (794/794) | 0 |
| `/web/search/no-such-instance` | 794×900 | Empty | Collapsed | Empty state visible | none (794/794) | 0 |
| `/web/audit` | 794×900 | 20 rows | Collapsed | Next `/1`, Previous `/0` | none (794/794) | 0 |
| `/web/audit-failure-detection` | 794×900 | Empty | Collapsed | Empty state visible | none (794/794) | 0 |
| `/web/audit-recovery` | 794×900 | Empty | Collapsed | Empty state visible | none (794/794) | 0 |
| `/web/audit-recovery/id/999999` | 794×900 | Empty detail | Collapsed | Not-found state visible | none (794/794) | 0 |
| `/web/status` | 794×900 | Populated | Collapsed | Status panels visible | none (794/794) | 0 |
| `/web/about` | 794×900 | Populated | Collapsed | Links/content visible | none (794/794) | 0 |
| `/web/faq` | 794×900 | Populated | Collapsed | Documentation content visible | none (794/794) | 0 |
| `/web/agents` | 794×900 | Disabled | Collapsed | Disabled state visible | none (794/794) | 0 |
| `/web/seeds` | 794×900 | Disabled | Collapsed | Disabled state visible | none (794/794) | 0 |
| `/web/clusters` | 390×844 | 1 cluster | Collapsed; toggler visible | Toggler and Home open once | none (390/390) | 0 |
| `/web/cluster/mysql1:3306` | 390×844 | 3 cards, 3 nodes, 2 links | Collapsed | Collapse/expand, View, Details, canvas scroll | none (390/390) | 0 |
| `/web/clusters-analysis` | 390×844 | ErrantGTIDStructureWarning | Collapsed | Incident visible | none (390/390) | 0 |
| `/web/discover` | 390×844 | Form ready | Collapsed | Inputs usable | none (390/390) | 0 |
| `/web/search/mysql1` | 390×844 | Populated | Collapsed | Results visible | none (390/390) | 0 |
| `/web/search/no-such-instance` | 390×844 | Empty | Collapsed | Empty state visible | none (390/390) | 0 |
| `/web/audit` | 390×844 | 20 rows | Collapsed | Next `/1`, Previous `/0` | none (390/390) | 0 |
| `/web/audit-failure-detection` | 390×844 | Empty | Collapsed | Empty state visible | none (390/390) | 0 |
| `/web/audit-recovery` | 390×844 | Empty | Collapsed | Empty state visible | none (390/390) | 0 |
| `/web/audit-recovery/id/999999` | 390×844 | Empty detail | Collapsed | Not-found state visible | none (390/390) | 0 |
| `/web/status` | 390×844 | Populated | Collapsed | Status panels visible | none (390/390) | 0 |
| `/web/about` | 390×844 | Populated | Collapsed | Links/content visible | none (390/390) | 0 |
| `/web/faq` | 390×844 | Populated | Collapsed | Documentation content visible | none (390/390) | 0 |
| `/web/agents` | 390×844 | Disabled | Collapsed | Disabled state visible | none (390/390) | 0 |
| `/web/seeds` | 390×844 | Disabled | Collapsed | Disabled state visible | none (390/390) | 0 |

## Interaction evidence

- Topology began at 3 semantic cards / 3 D3 nodes / 2 links. Collapsing the primary produced 1 node / 0 links; expanding restored 3 / 2.
- One View activation opened exactly one menu. One Details activation opened exactly one `#node_modal` and one backdrop; dismissal removed both.
- The Details activation left the card's `left` and `top` coordinates unchanged, proving the interactive click did not initiate drag.
- At 390px, `#cluster_canvas` measured 342px client width and 960px scroll width with `overflow-x: auto`; real browser scrolling moved `scrollLeft` from 0 to 450 while the body remained 390px wide.
- At responsive widths, the navbar, Home, Audit, and Problems menus each opened exactly once; Problems remained inside the collapsed navigation below 992px.
- The viewport override was reset after testing. The deliverable tab was left at `/web/clusters` in the natural 1280×720 environment with no overflow or console entries.

## Lab safety evidence

- MySQL container IDs were unchanged before and after the Orchestrator-only rebuild: mysql1 `76e92eb4a8be3381c6fbe047dc5b2ac08038a0ec386404859142ad6b59a04ae8`, mysql2 `ca05b9577b38739f475a6799252d7c513fcc8858d6af7830812af94e8f40b41d`, mysql3 `cf2ffd96825ec9fa135f047c5546cc4214d83c7b573e44d2eadf09a60629a1ad`.
- Roles remained mysql1 writable primary and mysql2/mysql3 read-only replicas with IO/SQL replication running.
- ProxySQL remained running under container ID prefix `da43b`; no dependency lifecycle operation and no failover occurred.

# Final verification: populated audit history

## Scope and commits

This handoff verifies the recovered, restored functional lab and the populated
audit-history UI evidence produced by this work.

Commits created before this handoff:

- `5556dbe9 test(ui): persist audit history in functional lab`
- `52bbe45f test(ui): verify functional audit persistence`
- `e722ba7c test(ui): exercise populated audit history`

Initial report commit: `d3c81f67 docs(ui): record populated audit verification`.

No production UI correction was required after browser review.

## Recovery and audit evidence

The controlled failure produced successful `DeadMaster` recovery records for
`mysql1:3306`; the recorded successor was `mysql2:3306`. Two recovery records
are present, each records `IsSuccessful: true`, `AnalysisEntry.Analysis:
DeadMaster`, and the successor `mysql2:3306` (the most recent is ID 2).

Fresh API counts from 2026-08-12 14:54 ICT:

| Endpoint | Records |
| --- | ---: |
| `/api/audit/0` | 20 |
| `/api/audit-failure-detection/0` | 2 |
| `/api/audit-recovery/0` | 2 |

Both detection records and both recovery records represent `DeadMaster` for
`mysql1:3306`; the recovery records are successful with `mysql2:3306` as
successor.

## Restored topology and identity

Fresh container inspection retained the IDs captured by the recovery harness:

| Service | Container ID | State | Role / replication |
| --- | --- | --- | --- |
| mysql1 | `76e92eb4a8be` | healthy | `read_only=0` |
| mysql2 | `ca05b9577b38` | healthy | source `mysql1`; IO `Yes`; SQL `Yes` |
| mysql3 | `cf2ffd96825e` | healthy | source `mysql1`; IO `Yes`; SQL `Yes` |

This matches the pre-restoration identity record: no MySQL container was
recreated. `SHOW REPLICA STATUS\\G` for mysql2 and mysql3 also reported zero
last IO and SQL errors and zero seconds behind source.

## Automated verification

All prescribed commands were run fresh and exited zero:

| Command / suite | Result |
| --- | --- |
| `go test ./go/http -count=1` | 1 package passed; fresh JSON run counted 77 passing Go tests |
| `for file in go/http/testdata/*_test.js; do node --test "$file" \|\| exit 1; done` | 4 Node test files; 23/23 tests passed |
| `node --check resources/public/js/*.js` | 30/30 JavaScript files parsed successfully |
| `bash tests/functional/test-smoke.sh` | 35 passed, 0 failed, 0 skipped |
| `git diff --check` | no whitespace errors |

The smoke run rediscovered all three instances and passed its audit-persistence,
web/API, health, metrics, and ProxySQL checks.

## Commit hygiene

After the initial report commit `d3c81f67`, `git status --short` produced no
output. The tracked worktree was clean; this report was the only file staged
and committed for that handoff.

## Browser evidence

Task 3 inspected the populated application at the default desktop viewport and
again at 390x844. At both sizes:

- `/web/audit` displayed its populated rows and correct pager states.
- `/web/audit-failure-detection` displayed two `DeadMaster` detections; the
  expanded detection showed the two replicas, changelog, processing node, and
  its recovery link.
- `/web/audit-recovery` displayed two `DeadMaster` recoveries and working UID
  detail links.
- `/web/audit-recovery/id/2` displayed failed `mysql1:3306`, successor
  `mysql2:3306`, timing and acknowledgement data, affected replicas, and all
  26 recovery steps. Its related-detection link also rendered the corresponding
  detail.

At 390px, the table/detail shells scrolled internally without document-level
horizontal overflow; empty and unavailable states stayed hidden while populated
content was shown. Browser console inspection found **0 errors and 0 warnings**
at both viewport sizes.

## Safety and unresolved concerns

The final state has the original mysql1 writer and two healthy replicas sourced
from mysql1. The recovery workflow restored this topology without recreating
containers, deleting volumes, or discarding SQLite history.

Unresolved concerns: **none**. Docker Compose emitted its pre-existing
obsolete-top-level-`version` notice and the MySQL client emitted its standard
command-line-password warning during topology inspection; neither is an
application/browser-console warning or a verification failure.

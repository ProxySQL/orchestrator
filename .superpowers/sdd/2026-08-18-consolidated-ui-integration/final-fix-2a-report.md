# Final review fix round 2A — Malformed failure-analysis payloads

Status: COMPLETE (scoped verification green; full Node run has concurrent audit-domain failures)

## Finding and root cause

`hasValidClustersAnalysisResponses` checked only the three response envelopes. A partial HTTP-200 response could therefore reach `buildClustersAnalysisModel`, which dereferenced `FailedInstanceKey`, `ClusterDetails`, and `AnalyzedInstanceKey`, or invoked `.forEach` on a truthy non-array `StructureAnalysis`. The exception escaped the jQuery `.done` callback before the existing unavailable renderer could hide the loading state, leaving loading or stale content visible.

## RED evidence

Four adapter-level behavior regressions were added to `go/http/testdata/clusters_analysis_state_test.js` for:

- a blocked recovery missing `FailedInstanceKey`;
- an analysis entry missing `ClusterDetails`;
- an analysis entry missing `AnalyzedInstanceKey`;
- a string-valued `StructureAnalysis`.

Each regression requires the adapter to hide loading, render `Analysis unavailable`, and avoid healthy, empty, incident-list, or loading markup. Before the production change, the focused command reported 13 passed and 4 failed. Each failure was the expected uncaught `TypeError` at the corresponding production dereference or iteration site.

## Implementation

The existing response validator now validates all display-relevant record and scalar shapes before model construction:

- cluster identity, alias, and instance count;
- analysis code, instance key, cluster details, replica count, downtime state, and structural-analysis values;
- blocked-recovery analysis and failed-instance key.

`StructureAnalysis` accepts an array of strings and also JSON `null`, because Go's nil `[]AnalysisCode` serializes as `null` and the existing model intentionally treats that as an empty list. Any other value, or a missing field, is rejected. Malformed successful responses now follow the existing unavailable-state return path. Model API, state precedence, unmatched actionable detection, sorting, topology paths, and escaping were not changed.

## GREEN and verification evidence

- `node --test go/http/testdata/clusters_analysis_state_test.js`: 17 passed, 0 failed.
- `node --check resources/public/js/clusters-analysis.js`: exit 0.
- `go test ./go/http`: passed.
- Scoped `git diff --check -- resources/public/js/clusters-analysis.js go/http/testdata/clusters_analysis_state_test.js`: passed.
- Full `node --test go/http/testdata/*.js`: all 17 clusters-analysis tests passed; the combined run reported 32 passed and 4 failures, all in concurrently added `audit_ui_safety_test.js` against concurrent audit-recovery/failure work outside this fix's scope.

## Files changed by fix 2A

- `resources/public/js/clusters-analysis.js`
- `go/http/testdata/clusters_analysis_state_test.js`
- `.superpowers/sdd/2026-08-18-consolidated-ui-integration/final-fix-2a-report.md`

No files were staged or committed. Concurrent audit-domain changes were left untouched.

## Concerns

- The shared worktree's full Node UI suite is not globally green while concurrent audit safety work is incomplete. The failures are outside this change and do not involve failure-analysis code.

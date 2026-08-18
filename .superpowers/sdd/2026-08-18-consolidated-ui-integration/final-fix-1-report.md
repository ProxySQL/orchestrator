# Final review fix round 1 — Dynamic Bootstrap controls

Status: COMPLETE

## Finding and root cause

`bootstrap-legacy-bridge.js` normalizes legacy Bootstrap attributes only during `init` and `DOMContentLoaded`. The recovery dropdown in `cluster.js` and the two alert dismiss buttons in `orchestrator.js` are appended after those passes, so Bootstrap 5's data API could not activate the late controls from their legacy-only attributes.

## RED evidence

Added `TestDynamicBootstrapControlsEmitNativeAttributes` in `go/http/static_assets_test.go`. Before the implementation change:

- `go test ./go/http -run '^TestDynamicBootstrapControlsEmitNativeAttributes$' -count=1` failed because the recovery dropdown lacked `data-bs-toggle="dropdown"`.
- The same test found zero of the two required alert emitters with `data-bs-dismiss="alert"`.

## Implementation

- The dynamic recovery button now emits both `data-toggle="dropdown"` and `data-bs-toggle="dropdown"`.
- `addAlert` and `addModalAlert` now emit both `data-dismiss="alert"` and `data-bs-dismiss="alert"`.
- `bootstrap-legacy-bridge.js` was not changed; no delegated bridge listener was added and its idempotence remains unchanged.
- Recovery `aria-expanded="true"` behavior remains unchanged.

## Automated GREEN evidence

- Focused regression: passed.
- `node --check resources/public/js/cluster.js`: passed.
- `node --check resources/public/js/orchestrator.js`: passed.
- All six `go/http/testdata/*_test.js` files: 26 tests passed, 0 failed.
- `go test ./go/http -count=1`: passed.
- `go test ./... -count=1`: all packages passed.
- Functional smoke: 50 passed, 0 failed, 0 skipped.
- `git diff --check`: passed before final verification.

## Browser evidence

After rebuilding the Linux/arm64 binary and recreating only Orchestrator with `--no-deps`, the Codex in-app Browser explicitly reloaded `/web/cluster/mysql1:3306`.

A safe `?orchestrator-msg=dynamic-bootstrap-fix` query created one alert after page initialization. The single dismiss button had both `data-dismiss="alert"` and `data-bs-dismiss="alert"`. One real click removed the sole alert and dismiss button; both counts changed from one to zero. There were no warning or error console entries before or after the click, and no duplicate Bootstrap action/error was observed.

The safe lab exposed no actionable recovery state and therefore rendered zero recovery dropdowns. Recovery dropdown activation remains static-contract-only; no failover was induced and no MySQL instance was stopped.

## Lab safety evidence

MySQL container IDs were unchanged before and after the Orchestrator-only rebuild:

- mysql1: `76e92eb4a8be3381c6fbe047dc5b2ac08038a0ec386404859142ad6b59a04ae8`
- mysql2: `ca05b9577b38739f475a6799252d7c513fcc8858d6af7830812af94e8f40b41d`
- mysql3: `cf2ffd96825ec9fa135f047c5546cc4214d83c7b573e44d2eadf09a60629a1ad`

ProxySQL remained running under unchanged container ID `da43b809c43d9ff755589194c3d154d40f28c6f4a9965b848849c9e0f7157a15`. mysql1 remained writable; mysql2/mysql3 remained read-only replicas of mysql1 with both IO and SQL threads running. Only Orchestrator changed container ID, from `bff2c0db72949309bfdb9454a879ae95e09782c79c190418ce9721107fbcb465` to `90780062e111efa000cf4cccc16b7919f5b1b16d0ef9fbaedcd8495ede44abbb`.

## Concerns

- Live recovery-dropdown interaction was intentionally not exercised because the safe lab had no actionable recovery state. The focused static regression proves the late emitter includes both legacy and native Bootstrap attributes.

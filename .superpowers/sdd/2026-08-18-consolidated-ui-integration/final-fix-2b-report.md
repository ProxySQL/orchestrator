# Final fix 2B report — audit route mode and DOM safety

## Scope

Owned changes:

- `resources/public/js/audit-recovery.js`
- `resources/public/js/audit-failure-detection.js`
- `go/http/testdata/audit_ui_safety_test.js`
- this report

Concurrent changes in `resources/public/js/clusters-analysis.js` and
`go/http/testdata/clusters_analysis_state_test.js` were not edited, staged, or
otherwise managed by this work.

## Root cause

1. Recovery list/detail mode was inferred from response cardinality with
   `auditEntries.length == 1`. A normal list or filtered route that happened to
   return one recovery therefore hid its pager/table and rendered the detail
   workspace. Route intent is supplied independently by `recoveryId()` and
   `recoveryUid()` and must be the only detail-mode signal.
2. Both audit scripts mixed API strings into HTML fragments before passing the
   result to jQuery. Stored host, cluster, analysis, changelog, error,
   acknowledgement, user, and comment values could therefore become parsed DOM.
   The same values were appended to API and web paths without segment encoding.
   Failure-detection expansion also interpolated an API ID into a selector.
3. Failure-detection pagination initialized `baseWebUri` through `appUrl()` and
   then passed the complete prefixed URI through `appUrl()` again in both click
   handlers. With a `/proxy` deployment prefix, navigation therefore targeted
   `/proxy/proxy/web/...`; the working audit and recovery handlers assign the
   already-prefixed base plus page directly.

## Fix

- `resolveRecoveryViewState` now selects `unavailable`, `empty`, `detail`, or
  `list` explicitly. A non-empty response becomes detail only on an ID or UID
  route; a one-row list remains a list with its table and pager.
- API text is rendered through jQuery-created elements using `.text()` and safe
  `.attr()` calls. Only fixed application markup is created as markup.
- Route segments for recovery and failure-detection API calls, detail links,
  search links, cluster links, related-record links, and pager bases use
  `encodeURIComponent`.
- Recovery acknowledgement and metadata renderers and failure-detection metadata
  renderers are small browser-compatible helpers with CommonJS exports for real
  behavior tests.
- Failure-detection row expansion compares data attributes rather than building
  a selector from an API value.
- `failureDetectionPagerUrl` appends the page to the already-prefixed base URI;
  previous and next handlers no longer apply the application prefix twice.

## TDD evidence

Initial RED:

```text
node --test go/http/testdata/audit_ui_safety_test.js
tests 6; pass 0; fail 6
```

All six assertions failed because the route-state and safe DOM builders did not
exist. A second RED added direct request/pager path coverage and failed 1 of 7
tests because the segment helpers were not exported.

An adjacent pager RED then passed the existing seven tests and failed one new
assertion because `failureDetectionPagerUrl` did not exist. The fixture used the
prefixed base `/proxy/web/audit-failure-detection/` and required previous/next
targets containing exactly one `/proxy` prefix.

Focused GREEN:

```text
node --test go/http/testdata/audit_ui_safety_test.js
tests 8; pass 8; fail 0
```

The hostile fixture contains an `<img onerror>` payload plus quotes, apostrophe,
ampersand, slashes, spaces, and question marks. Tests assert inert escaped text,
encoded URL segments, one-row list mode, ID/UID detail mode, and exclusive empty
and unavailable states.

## Verification

Fresh verification completed at HEAD `7e68960817c5f5f28429da409908cf8f00fce037`:

```text
node --check resources/public/js/audit-recovery.js                         PASS
node --check resources/public/js/audit-failure-detection.js                PASS
for file in go/http/testdata/*_test.js; do node --test "$file"; done       PASS (38 tests)
go test ./go/http -count=1                                                  PASS
git diff --check                                                            PASS
```

No Docker, browser, git-index, commit, or GitHub operation was performed, per
the task boundary.

## Concerns

- Verification is automated only because this independent fix explicitly
  prohibited Docker and browser work. The helpers exercise actual rendering
  construction with a hostile DOM serializer, while existing live acceptance
  remains the integration-level UI evidence.
- Recovery steps are rendered by the separate shared script with `.text()`;
  this change only encodes the UID before handing it to that script.

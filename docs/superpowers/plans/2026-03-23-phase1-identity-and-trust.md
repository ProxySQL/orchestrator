# Phase 1: Identity & Trust — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the orchestrator repo clearly say "ProxySQL LLC maintains this — it's alive, come contribute." Tag `4.0.0-rc1`.

**Architecture:** Each task is an independent PR against `master`. Tasks 1-5 can be done in parallel. Task 6 (docs update) depends on Task 1 (README) to avoid conflicts. Task 7 (tag) depends on Task 5 (CI fix) passing green.

**Tech Stack:** Markdown, YAML (GitHub Actions), Git

**Spec:** `docs/superpowers/specs/2026-03-23-orchestrator-refresh-design.md`

---

### Task 1: README overhaul

**GitHub Issue Title:** Rebrand README for ProxySQL LLC maintainership

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Rewrite README.md**

Replace the entire README with updated content:

- Replace Percona fork notice with ProxySQL LLC maintainer statement
- Update lineage: Outbrain (2014) → GitHub (2016) → openark (2020) → Percona → ProxySQL LLC (2026)
- Update badge URLs from `openark/orchestrator` → `proxysql/orchestrator`
- Update documentation link from `percona/orchestrator` → `proxysql/orchestrator`
- Update logo image URL to local path `docs/images/orchestrator-logo-wide.png` (avoid external dependency)
- Add "Future Vision" section mentioning ProxySQL-native integration and PostgreSQL exploration
- Link to CONTRIBUTING.md, CODE_OF_CONDUCT.md, SECURITY.md
- Keep the existing feature descriptions (Discovery, Refactoring, Recovery, Interface, Additional perks) — they're good
- Remove the `orchestrator@percona.com` contact and Percona-specific language
- Update "Orchestrator documentation" link to `proxysql/orchestrator`

- [ ] **Step 2: Verify all links in README are valid**

Run:
```bash
grep -oP 'https?://[^\s\)]+' README.md
```
Manually inspect each URL references `proxysql/orchestrator` where appropriate.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "Rebrand README for ProxySQL LLC maintainership"
```

---

### Task 2: LICENSE and copyright update

**GitHub Issue Title:** Add ProxySQL LLC copyright notice to LICENSE

**Files:**
- Modify: `LICENSE`

- [ ] **Step 1: Read current LICENSE**

The LICENSE is Apache 2.0. Lines 189-192 contain the copyright history (Outbrain, Booking.com, GitHub, Shlomi Noach). Do NOT remove any existing attributions.

- [ ] **Step 2: Add ProxySQL LLC copyright**

Add at the end of the copyright notices section:

```
Copyright 2026 ProxySQL LLC
```

Keep all existing copyright lines intact.

- [ ] **Step 3: Commit**

```bash
git add LICENSE
git commit -m "Add ProxySQL LLC copyright notice"
```

---

### Task 3: Governance files

**GitHub Issue Title:** Add governance files (CONTRIBUTING, CODE_OF_CONDUCT, SECURITY, MAINTAINERS)

**Files:**
- Create: `CONTRIBUTING.md`
- Create: `CODE_OF_CONDUCT.md`
- Create: `SECURITY.md`
- Create: `MAINTAINERS.md`

- [ ] **Step 1: Create CONTRIBUTING.md**

Contents should cover:
- Welcome message — contributions are encouraged
- How to report bugs (GitHub Issues with template)
- How to suggest features (GitHub Issues with "feature request" template)
- How to submit PRs:
  - Fork the repo, create a branch, submit PR against `master`
  - Reference the related issue
  - Code must pass `gofmt` (not `goimports`)
  - Code must pass all CI tests (unit, integration, system)
  - Include tests for new functionality
- DCO sign-off requirement (`Signed-off-by` line, same as Linux kernel)
  - Explain: `git commit -s` adds it automatically
- Coding standards: follow existing conventions, `gofmt -s`, test coverage expected

- [ ] **Step 2: Create CODE_OF_CONDUCT.md**

Use the Contributor Covenant v2.1 (https://www.contributor-covenant.org/version/2/1/code_of_conduct/). Set enforcement contact to an appropriate ProxySQL email.

- [ ] **Step 3: Create SECURITY.md**

Contents:
- How to report security vulnerabilities (email, NOT public issues)
- Contact: security@proxysql.com (or appropriate address — confirm with René)
- Expected response time
- Supported versions (4.x)
- Credit policy for responsible disclosure

- [ ] **Step 4: Create MAINTAINERS.md**

Contents:
- René Cannaò — Project Lead, ProxySQL LLC
- (Add other maintainers as needed)
- Roles: what maintainers do (review PRs, triage issues, manage releases)

- [ ] **Step 5: Commit**

```bash
git add CONTRIBUTING.md CODE_OF_CONDUCT.md SECURITY.md MAINTAINERS.md
git commit -m "Add governance files for community contribution"
```

---

### Task 4: GitHub templates refresh

**GitHub Issue Title:** Update GitHub issue and PR templates

**Files:**
- Delete: `.github/ISSUE_TEMPLATE.md`
- Create: `.github/ISSUE_TEMPLATE/bug_report.md`
- Create: `.github/ISSUE_TEMPLATE/feature_request.md`
- Create: `.github/ISSUE_TEMPLATE/question.md`
- Modify: `.github/PULL_REQUEST_TEMPLATE.md`

- [ ] **Step 1: Create bug report template**

File: `.github/ISSUE_TEMPLATE/bug_report.md`

YAML frontmatter with `name: Bug Report`, `about: Report a bug`, `labels: bug`.

Body should request:
- orchestrator version
- Backend (MySQL/SQLite)
- orchestrator.conf.json (sanitized)
- Topology info (`orchestrator-client -c topology -alias my-cluster`)
- Steps to reproduce
- Expected vs actual behavior
- Logs (`--debug --stack`)

- [ ] **Step 2: Create feature request template**

File: `.github/ISSUE_TEMPLATE/feature_request.md`

YAML frontmatter with `name: Feature Request`, `about: Suggest a new feature`, `labels: enhancement`.

Body should request:
- Problem description / use case
- Proposed solution
- Alternatives considered
- Additional context

- [ ] **Step 3: Create question template**

File: `.github/ISSUE_TEMPLATE/question.md`

YAML frontmatter with `name: Question`, `about: Ask a question about orchestrator`, `labels: question`.

Body: free-form, with a note to check docs first.

- [ ] **Step 4: Update PR template**

File: `.github/PULL_REQUEST_TEMPLATE.md`

- Change issue URL from `https://github.com/openark/orchestrator/issues/0123456789` → `https://github.com/proxysql/orchestrator/issues/`
- Reference CONTRIBUTING.md
- Update checklist:
  - `[ ]` Code formatted with `gofmt`
  - `[ ]` Tests added/updated
  - `[ ]` CI passes
  - `[ ]` DCO sign-off included
  - `[ ]` Related issue linked

- [ ] **Step 5: Delete old issue template**

```bash
git rm .github/ISSUE_TEMPLATE.md
```

- [ ] **Step 6: Add .github/FUNDING.yml (if applicable)**

If ProxySQL has a GitHub Sponsors profile or other funding channels, create `.github/FUNDING.yml`:

```yaml
github: proxysql
```

If no funding channels exist yet, skip this step — it can be added later.

- [ ] **Step 7: Commit**

```bash
git add .github/ISSUE_TEMPLATE/ .github/PULL_REQUEST_TEMPLATE.md .github/FUNDING.yml
git commit -m "Refresh GitHub issue and PR templates"
```

---

### Task 5: Fix CI workflows

**GitHub Issue Title:** Fix CI: update Go version and GitHub Actions to current versions

**Files:**
- Modify: `.github/workflows/main.yml`
- Modify: `.github/workflows/system.yml`
- Modify: `.github/workflows/upgrade.yml`

- [ ] **Step 1: Update main.yml**

Changes:
- `actions/checkout@v2` → `actions/checkout@v4`
- `actions/setup-go@v1` → `actions/setup-go@v5`
- `go-version: 1.16` → `go-version: '1.25.7'` (quote to avoid YAML float parsing)
- `actions/upload-artifact@v1` → `actions/upload-artifact@v4`

- [ ] **Step 2: Update system.yml**

Changes:
- `actions/checkout@v2` → `actions/checkout@v4`
- `actions/setup-go@v1` → `actions/setup-go@v5`
- `go-version: 1.16` → `go-version: '1.25.7'`
- Update `orchestrator-ci-env` clone URLs from `https://github.com/openark/orchestrator-ci-env.git` → `https://github.com/percona/orchestrator-ci-env.git` (Note: use percona here because they maintain the CI env repo; update to proxysql if/when we fork it)

- [ ] **Step 3: Update upgrade.yml**

Changes:
- `actions/checkout@v2` → `actions/checkout@v4`
- `actions/setup-go@v1` → `actions/setup-go@v5`
- `go-version: 1.16` → `go-version: '1.25.7'`

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/main.yml .github/workflows/system.yml .github/workflows/upgrade.yml
git commit -m "Fix CI: update Go 1.16 to 1.25.7 and bump GitHub Actions versions"
```

- [ ] **Step 5: Push and verify CI passes**

Push the branch and confirm all three workflows pass green. This is the gate for tagging 4.0.0-rc1.

---

### Task 6: Docs references update

**GitHub Issue Title:** Update documentation links from openark/percona to proxysql

**Files to modify** (all in `docs/`):
- `docs/install.md`
- `docs/license.md`
- `docs/orchestrator-client.md`
- `docs/pseudo-gtid-manual-injection.md`
- `docs/risks.md`
- `docs/users.md`
- `docs/using-the-web-api.md`
- `docs/agents.md`
- `docs/bugs.md`
- `docs/build.md`
- `docs/ci-env.md`
- `docs/ci.md`
- `docs/configuration-discovery-pseudo-gtid.md`
- `docs/configuration.md`
- `docs/docker.md`
- `docs/download.md`
- `docs/faq.md`

**Depends on:** Task 1 (README) merged first to avoid conflicts.

- [ ] **Step 1: Bulk-replace openark/orchestrator references**

In all docs/ markdown files, replace:
- `github.com/openark/orchestrator` → `github.com/proxysql/orchestrator`
- `https://github.com/openark/orchestrator` → `https://github.com/proxysql/orchestrator`

Be careful NOT to change:
- Historical references that are clearly about the old repo (e.g., "originally at openark/orchestrator")
- The `go.mod` module path (that changes in Phase 2)
- References to `openark/orchestrator-ci-env` (separate repo, changes when we fork it)

- [ ] **Step 2: Replace percona/orchestrator references**

- `github.com/percona/orchestrator` → `github.com/proxysql/orchestrator`
- `https://github.com/percona/orchestrator` → `https://github.com/proxysql/orchestrator`

- [ ] **Step 3: Verify no broken links**

```bash
grep -rn "openark/orchestrator" docs/ | grep -v "ci-env" | grep -v "spec"
grep -rn "percona/orchestrator" docs/ | grep -v "spec"
```
Both should return empty (excluding the design spec which correctly references history).

- [ ] **Step 4: Commit**

```bash
git add docs/
git commit -m "Update documentation links to proxysql/orchestrator"
```

---

### Task 7: Tag 4.0.0-rc1

**GitHub Issue Title:** Release 4.0.0-rc1 — ProxySQL LLC maintainership announcement

**Files:**
- Modify: `RELEASE_VERSION`

**Depends on:** All previous tasks merged. CI (Task 5) passing green.

- [ ] **Step 1: Update RELEASE_VERSION**

Change content from `3.2.6` to `4.0.0-rc1`.

- [ ] **Step 2: Commit**

```bash
git add RELEASE_VERSION
git commit -m "Bump version to 4.0.0-rc1"
```

- [ ] **Step 3: Create annotated tag**

```bash
git tag -a v4.0.0-rc1 -m "v4.0.0-rc1: ProxySQL LLC takes over orchestrator maintainership"
```

- [ ] **Step 4: Create GitHub Release**

Use `gh release create v4.0.0-rc1` with release notes covering:
- ProxySQL LLC is the new maintainer of orchestrator
- Brief history and motivation
- What changed: governance, CI fixes, updated docs
- What's coming in 4.0.0: Go module path migration, dependency updates, code quality improvements
- How to contribute: link to CONTRIBUTING.md
- Mark as pre-release

---

## Execution Order

```
Tasks 1-5: can run in parallel (independent PRs)
Task 6: after Task 1 is merged (avoids README/docs conflicts)
Task 7: after all tasks merged and CI green
```

## GitHub Issues Summary

Create these 7 issues before starting work:

1. "Rebrand README for ProxySQL LLC maintainership"
2. "Add ProxySQL LLC copyright notice to LICENSE"
3. "Add governance files (CONTRIBUTING, CODE_OF_CONDUCT, SECURITY, MAINTAINERS)"
4. "Update GitHub issue and PR templates"
5. "Fix CI: update Go version and GitHub Actions to current versions"
6. "Update documentation links from openark/percona to proxysql"
7. "Release 4.0.0-rc1 — ProxySQL LLC maintainership announcement"

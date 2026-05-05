# Repository Alignment Audit Report

Branch: `012-repo-alignment-audit`
Audit date: 2026-05-04

## Overall Status

COMPLIANT.

The repository is aligned with the latest constitution after the minimal fixes
in this branch, including the previously identified Principle XIII follow-ups.
All validation commands listed below pass.

## Confirmed Violations Fixed In This Branch

| ID | Severity | Area | Finding | Resolution |
|----|----------|------|---------|------------|
| C01 | HIGH | Principle IV / CI | Report generation and release artifacts could include wall-clock build metadata, weakening deterministic output and release reproducibility checks. | Report `GeneratedAt` now derives from snapshot timestamps with a deterministic epoch fallback. Release metadata derives from `SOURCE_DATE_EPOCH`, and CI verifies repeat release hashes. |
| C02 | HIGH | Enforcement | The constitution pre-push guard was interactive and could not act as a reliable CI-failing gate. | Added explicit CI mode and wired the CI test job to run the guard against the PR diff with full history. |
| C03 | MEDIUM | Principle XIII | The mysqladmin UI kept both the canonical multi-selection localStorage key and a legacy single-selection key path. | Removed the legacy key and legacy fallback loading path. |
| C04 | MEDIUM | Spec / source drift | Feedback success and throttle behavior diverged from the approved contract around issue numbers, focus movement, retry messaging, and preflight cache headers. | Updated the embedded client and Worker tests/contracts to use the canonical success link, focus target, `Retry-After`, and no-store preflight behavior. |
| C05 | MEDIUM | Spec / source drift | Feature 003 docs described stale REST/GraphQL flow, dependency policy, Node version, Go version, and idempotency guarantees. | Updated spec, plan, and quickstart wording to match current Worker and repository behavior. |
| C06 | LOW | Spec / source drift | Feature 002 voice attachment references used stale duration wording. | Updated contract and tasks to 10 minutes. |
| C07 | LOW | Documentation drift | Active feature pointers still named the prior feature. | Added feature 012 artifacts and aligned `.specify/feature.json`, `AGENTS.md`, and `CLAUDE.md`. |
| C08 | LOW | Template / manifest drift | Spec Kit template hashes were stale after the constitution alignment merge. | Updated `.specify/integrations/speckit.manifest.json`. |
| C09 | LOW | Documentation drift | README omitted `.agents/`, and the changelog omitted shipped version headings. | Added the missing repository layout entry and changelog version headings. |
| C10 | LOW | Enforcement coverage | Import lint did not include the first-party `findings` package. | Added `findings` to the linted package set. |
| C11 | MEDIUM | Principle XII | `go.mod` did not match the current repository toolchain. | Updated the Go line to `1.26`. |
| C12 | MEDIUM | Enforcement portability | CI exposed that the guard's English-only array loop was not portable to Bash 3 on macOS under `set -u`. | Appended English-only violations directly to the shared violation list instead of iterating an empty array. |
| C13 | HIGH | Principle XIII | Formatting and report-value helpers were duplicated across `render` and `findings`. | Added canonical `reportutil` helpers and removed duplicate render/findings implementations. |
| C14 | MEDIUM | Principle XIII / contracts | Feedback endpoints, categories, and validation limits were represented separately in Go, browser JS, Worker TypeScript, and tests. | Added canonical `render/assets/feedback-contract.json`, made Go/browser/Worker consume it, removed legacy per-URL dialog attributes, and updated feature-003 docs. |
| C15 | MEDIUM | Principle III / XIII | Parser compatibility fallbacks for netstat and environment meminfo were not consistently observable. | Added structured diagnostics for no-TS netstat compatibility, ambiguous state-first `ESTAB`, and env-meminfo partial-sample fallback; Discover now attaches env-meminfo sidecar diagnostics. |

## Confirmed Follow-Ups Resolved In This Branch

| ID | Severity | Area | Finding | Resolution |
|----|----------|------|---------|-----------------|
| F01 | HIGH | Principle XIII | Formatting and metric helper behavior was duplicated across `render` and `findings` (`formatNum`/`humanInt`, `gaugeLast`/`variableFloat`, byte formatting). | Resolved by canonical `reportutil` package with focused tests. No wrapper shims or duplicate old helper implementations remain. |
| F02 | MEDIUM | Principle XIII / contracts | Feedback payload and response semantics were represented in separate Go templates, browser JavaScript, TypeScript validation, and tests. | Resolved by canonical feedback contract JSON consumed by Go, browser JS, and Worker validation/request-size gates. |
| F03 | MEDIUM | Principle III / XIII | Some parser degradation paths, especially netstat and memory fallbacks, were broad compatibility branches with limited diagnostics. | Resolved for the confirmed paths by emitting structured diagnostics while preserving compatibility behavior. |

## Risks And Ambiguities

| ID | Severity | Area | Finding | Decision |
|----|----------|------|---------|----------|
| R01 | LOW | External degradation | Feedback Worker label resolution can skip missing remote GitHub labels. | Accepted as an external-service degradation risk for now; should be covered by a future Worker contract hardening pass. |
| R02 | LOW | Historical governance text | The constitution contains historical references to the older 8-gate model inside prior sync-impact history. | Accepted as historical record, not live policy. Live gates list uses the current 9-gate model. |
| R03 | LOW | External degradation | The browser client still falls back to GitHub when `fetch` is unavailable or the Worker/network fails. | Accepted as visible external degradation, not a duplicate implementation for the Worker path. |
| R04 | LOW | UI contract wording | Historical feature 002 popup-blocked fallback wording is narrower than the current feature 003 Worker-first submit flow. | Accepted as historical wording; feature 003 owns current submit behavior. |
| R05 | LOW | Feedback URL construction | Fallback URL construction appends encoded query parameters manually. | Accepted as low risk because the canonical base URL has a controlled query shape and all dynamic values are encoded. |
| R06 | LOW | Golden enforcement comments | A test comment still describes future/soft golden coverage even though hooks and golden tests now provide coverage. | Documentation cleanup only; not a live enforcement bypass. |
| R07 | LOW | English-only exemptions | Two exemptions are currently file:line based, which is brittle after edits. | The current guard passes; future guard hardening should switch to stable content/context matching. |

## False Positives

| ID | Area | Finding | Rationale |
|----|------|---------|-----------|
| P01 | Principle XV | Large generated or dependency files appeared in a raw line-count scan. | Governed source excludes dependency trees, worktrees, dist artifacts, golden snapshots, specs, docs, and fixtures. First-party governed source files remain under 1000 lines. |
| P02 | Agent docs | `.agents/skills` and `.claude/skills` look similar. | They are agent-specific entry points required by the side-by-side agent contract, not competing runtime implementations. |
| P03 | English-only | User-provided non-English text appears in historical request quotes. | Durable artifacts are English except explicitly allowed verbatim user-input exceptions already documented by the constitution guard. |

## Spec Kit Analysis

No high-severity drift was found across `spec.md`, `plan.md`, and `tasks.md`
for feature 012.

| Requirement | Covered By | Notes |
|-------------|------------|-------|
| FR-001 consolidated audit report | T004, T005 | This file is the canonical report. |
| FR-002 severity classification | T005 | Findings are classified as HIGH / MEDIUM / LOW. |
| FR-003 separated confirmed, risk, false positive groups | T005 | Sections are separated above. |
| FR-004 minimal canonical fixes | T006-T011, T017-T019 | Confirmed canonical-path follow-ups were resolved without adding parallel implementations. |
| FR-005 validation | T012-T015 | Validation commands passed. |
| FR-006 documented follow-ups | T011, T017-T019 | Previously deferred follow-ups are resolved above. |

## Validation Results

| Command | Result |
|---------|--------|
| `git diff --check` | PASS |
| `go test -count=1 ./...` | PASS |
| `go vet ./...` | PASS |
| `make lint` | PASS |
| `make test` | PASS |
| `cd feedback-worker && npm run typecheck && npm test` | PASS |
| `bash -n scripts/hooks/pre-push-constitution-guard.sh` | PASS |
| `CONSTITUTION_GUARD_CI=1 CONSTITUTION_GUARD_RANGE=origin/main..HEAD scripts/hooks/pre-push-constitution-guard.sh` | PASS |
| `.specify/scripts/bash/check-prerequisites.sh --json --require-tasks --include-tasks` | PASS |
| `/speckit-analyze` style review for feature 012 | PASS, no high-severity drift |
| `make release` | PASS |

## Final Compliance Decision

COMPLIANT. The confirmed source helper duplication, feedback contract
duplication, and parser fallback observability findings are resolved in this
branch, and validation is clean.

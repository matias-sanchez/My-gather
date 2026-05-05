# Repository Alignment Audit Report

Branch: `012-repo-alignment-audit`
Audit date: 2026-05-04

## Overall Status

PARTIAL.

The repository is materially aligned with the latest constitution after the
minimal fixes in this branch, and all validation commands listed below pass.
The status remains PARTIAL because the audit confirmed broader Principle XIII
canonical-path concerns that require a dedicated source refactor and should not
be partially solved in this governance branch.

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

## Confirmed Follow-Ups Deferred

| ID | Severity | Area | Finding | Reason Deferred |
|----|----------|------|---------|-----------------|
| F01 | HIGH | Principle XIII | Formatting and metric helper behavior is duplicated across `render` and `findings` (`formatNum`/`humanInt`, `gaugeLast`/`variableFloat`, byte formatting). | Requires a designed package-boundary refactor so one helper owner becomes canonical without creating an import cycle or a partial parallel path. |
| F02 | MEDIUM | Principle XIII / contracts | Feedback payload and response semantics are represented in separate Go templates, browser JavaScript, TypeScript validation, and tests. | A machine-readable contract or generation step needs a dedicated design; this branch only fixed direct drift. |
| F03 | MEDIUM | Principle III / XIII | Some parser degradation paths, especially netstat and memory fallbacks, are broad compatibility branches with limited diagnostics. | Needs parser-specific behavior and fixture review; changing it here risks altering report semantics beyond the audit scope. |

## Risks And Ambiguities

| ID | Severity | Area | Finding | Decision |
|----|----------|------|---------|----------|
| R01 | LOW | External degradation | Feedback Worker label resolution can skip missing remote GitHub labels. | Accepted as an external-service degradation risk for now; should be covered by a future Worker contract hardening pass. |
| R02 | LOW | Historical governance text | The constitution contains historical references to the older 8-gate model inside prior sync-impact history. | Accepted as historical record, not live policy. Live gates list uses the current 9-gate model. |
| R03 | LOW | Report compatibility | The browser client still has an explicit no-Worker fallback when old reports are generated without `WORKER_URL`. | Accepted as visible external degradation, not a duplicate implementation for the Worker path. |
| R04 | LOW | UI contract wording | Historical feature 002 popup-blocked fallback wording is narrower than the current feature 003 Worker-first submit flow. | Accepted as historical wording; feature 003 owns current submit behavior. |
| R05 | LOW | Feedback URL construction | Legacy fallback URL construction appends a query string manually instead of using `URL.searchParams.set()`. | Accepted as low risk because the base URL has a controlled query shape; can be cleaned up in a future feedback refactor. |
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
| FR-004 minimal canonical fixes | T006-T011 | Larger refactors are deferred instead of partially implemented. |
| FR-005 validation | T012-T015 | Validation commands passed. |
| FR-006 documented follow-ups | T011 | Deferred follow-ups are listed above. |

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

PARTIAL until F01-F03 are resolved. The current branch is reviewable and
validation-clean, but the repository is not yet fully 100% aligned with the
canonical-path direction because at least one confirmed source helper
duplication remains by design.

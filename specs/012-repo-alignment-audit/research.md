# Research: Repository Alignment Audit

## Decision: Patch Small Confirmed Violations In This Branch

Confirmed issues that are local, low-risk, and directly tied to existing owners
are remediated here. Examples include stale active-feature pointers, manifest
hash drift, the legacy mysqladmin persistence path, and response/header contract
gaps.

Rationale: these are alignment fixes, not new features.

## Decision: Defer Larger Canonical-Path Refactors

The audit confirmed deeper Principle XIII concerns around shared formatting and
metric helpers plus duplicated feedback constants. These require package-boundary
design or generation strategy and are intentionally left as follow-ups.

Rationale: extracting shared helpers or generating cross-language contracts is
not a minimal audit correction and should have its own spec/PR.

## Decision: Keep External Degradation Explicit

The feedback Worker fallback remains allowed because it is the named
constitution exception, visible to users, and routed through the existing
feedback owner.

# Research: Reconciliation Cleanup

## R1: Netstat malformed `TS` detection

**Decision**: add a broader netstat-specific timestamp-header detector that
recognizes any `TS <token> <timestamp>` header, then pass the token through the
canonical epoch parser. This keeps malformed boundaries observable without
changing other collectors that rely on the narrower numeric timestamp regex.

**Rationale**: The bug is not epoch parsing itself; it is that malformed
non-numeric or negative tokens do not reach epoch parsing.

## R2: Spec Kit git helper loading

**Decision**: the Bash git feature script must source the installed canonical
`.specify/scripts/bash/common.sh` only. The PowerShell script may use its
PowerShell git helper because no installed PowerShell core helper exists in
this repository, but it must not probe absent fallback locations.

**Rationale**: Principle XIII forbids hidden fallback paths and duplicate
helper implementations for the same behavior. Where no same-language canonical
core helper exists, the extension helper is the canonical path for that shell
family.

## R3: Feature 001 drift

**Decision**: edit stale historical wording to name the current shipped report
shape and constitution model without changing runtime behavior.

**Rationale**: Feature 001 remains historical, but feature 013 established that
historical artifacts should not contradict shipped behavior in normative text.

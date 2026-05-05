# Data Model: Repository Alignment Audit

## Audit Finding

A normalized finding from one or more audit agents.

Fields:

- `id`: stable local identifier
- `severity`: HIGH, MEDIUM, or LOW
- `category`: confirmed violation, risk/ambiguity, or false positive
- `source_agents`: audit dimensions that reported the finding
- `evidence`: file and line references
- `decision`: fixed, deferred, or dismissed
- `rationale`: concise reason for the decision

## Audit Report

The consolidated output for the PR.

Rules:

- Duplicate findings are merged before remediation.
- Confirmed findings that are not fixed must carry a follow-up rationale.
- False positives must cite the source of the exemption or alignment.

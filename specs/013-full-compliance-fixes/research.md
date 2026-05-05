# Research: Full Compliance Fixes

## Decisions

### R1: Remove the old core feature creation script

**Decision**: Keep branch creation in `.specify/extensions/git/scripts/` and
delete `.specify/scripts/bash/create-new-feature.sh`.

**Rationale**: The repo-local git feature skill already names the extension
script as the canonical implementation. Keeping the old core script preserves a
competing source path and violates Principle XIII.

### R2: Fail closed on missing feature pointer

**Decision**: `get_feature_paths` fails when `.specify/feature.json` is missing
or unusable unless `SPECIFY_FEATURE_DIRECTORY` is explicitly provided.

**Rationale**: Branch-name inference is a hidden fallback from the canonical
feature pointer. Explicit operator override remains allowed because it is not
silent.

### R3: Use Workers Vitest pool

**Decision**: Configure Vitest with `@cloudflare/vitest-pool-workers` and the
checked-in `wrangler.toml`.

**Rationale**: Feature 003 already declares the dependency and documentation
expects Worker-runtime-compatible tests. Using the declared pool aligns tests
with the documented runtime without adding a dependency.

### R4: Keep historical specs, fix contradictions

**Decision**: Do not rewrite old feature task history wholesale. Only remove
contradictory normative wording found by the audit.

**Rationale**: Historical specs are useful as delivery records. The compliance
problem is contradiction, not the existence of history.

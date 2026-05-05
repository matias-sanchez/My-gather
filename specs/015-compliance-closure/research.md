# Research: Compliance Closure

## Decisions

### Historical specs with backlog

**Decision**: Mark historical specs as shipped or complete with tracked backlog
instead of marking all historical backlog tasks complete.

**Rationale**: Principle XIII forbids misleading parallel interpretations, but
it does not require rewriting history. The accurate state is shipped behavior
plus explicitly tracked backlog.

### Unsupported collector versions

**Decision**: Preserve parse status in render inputs and render
unsupported-version states per affected subview.

**Rationale**: Parse already records `ParseUnsupported`; the missing path is
render presentation. Reusing that status keeps one canonical parse owner.

### Worker route coverage

**Decision**: Add route-level tests for existing Worker behavior only.

**Rationale**: The audit identified coverage gaps, not product gaps. Tests
should exercise `/health`, 404, malformed JSON, size rejection, idempotency, and
success behavior without changing endpoints.

# File Size Contract

## Limit

Governed first-party source code files must be at or below 1000 lines.

## Exemptions

The mechanical test only checks source-code file extensions. It explicitly
exempts:

- `_references/**`
- `testdata/**`
- vendored third-party minified assets

Specs, docs, JSON data, golden snapshots, and generated lockfiles are outside
this source-code rule.

## Mechanical Enforcement

```bash
go test ./tests/coverage -run TestGovernedSourceFileLineLimit
```

The test must fail with each offending path and line count.

## Embedded Asset Contract

Report assets are stored as smaller source parts:

- `render/assets/app-js/*.js`
- `render/assets/app-css/*.css`

The render package concatenates each set in lexical filename order and embeds
the result into the same report fields used before this feature.

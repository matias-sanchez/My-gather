# Quickstart: Richer Processlist Metrics

## 1. Run focused tests

```bash
go test ./parse -run 'TestProcesslist'
go test ./render -run 'Test.*Processlist|TestRenderIncludesProcesslist'
```

Expected result: tests pass and cover active/sleeping totals, longest age,
row-count maxima, query-text counts, payload shape, and markup callouts.

## 2. Run the full Go suite

```bash
go test ./...
make lint
```

Expected result: all tests and vet checks pass.

## 3. Validate against the incident collection

```bash
tmp="$(mktemp -d)"
go run ./cmd/my-gather -o "$tmp/report.html" --overwrite \
  /Users/matias/Documents/Incidents/CS0060148/eu-hrznp-d003/pt-stalk
```

Expected result:

- command exits with status 0
- generated report is a single HTML file
- Processlist subview has summary callouts for active/sleeping, age, rows, and
  query-text rows
- Processlist chart includes a `By Activity` control

## 4. Determinism spot check

```bash
tmp="$(mktemp -d)"
go run ./cmd/my-gather -o "$tmp/a.html" --overwrite testdata/example2
go run ./cmd/my-gather -o "$tmp/b.html" --overwrite testdata/example2
diff -u "$tmp/a.html" "$tmp/b.html" | head
```

Expected result: no unexpected differences beyond the existing generated-time
allowance already handled by the test suite.

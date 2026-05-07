# Quickstart: Top CPU Processes Caption

## What this feature ships

A short caption above the "Top CPU processes" chart in the OS
Usage section that explains the chart's two non-obvious rules:

1. The chart shows only the top 3 processes by average CPU.
2. The `mysqld` process is always included, even when it would
   not be in the top 3 by CPU.

Plus a regression test that locks the existing mysqld pin in
`render/concat.go::concatTop`.

## Verify locally

```sh
# 1. Build the report binary.
go build ./cmd/...

# 2. Run the full test suite (caption-presence test + pin tests).
go test ./...

# 3. Render a report against a known fixture and grep for the caption.
go run ./cmd/my-gather-report \
  --input testdata/<some-fixture> \
  --output /tmp/report.html
grep -F 'Showing the top 3 processes by average CPU. mysqld is always included' /tmp/report.html
```

The grep MUST return exactly one match for any report that has
`-top` data.

## Verify the pin still works

```sh
go test ./render/ -run TestConcatTopMysqldPinnedWhenLowest -v
```

This test fails if a future change drops the mysqld pin.

## Verify the caption is gated

Open a report generated from a fixture with no `-top` capture (or
unparseable `-top`). The caption MUST NOT appear in that report;
the existing "Data not available" banner MUST appear instead.

# Quickstart: Synchronized Chart Zoom + Per-Window Legend Stats

## Build and validate

```bash
# from the repo root
go build ./...
go test ./...
make lint
bash scripts/hooks/pre-push-constitution-guard.sh
```

## Manual verification

1. Build the binary: `go build ./cmd/my-gather`.
2. Run it against any pt-stalk fixture: `./my-gather -in testdata/<fixture> -out /tmp/report.html`.
3. Open `/tmp/report.html` in a browser.
4. Drag-zoom the x-axis on any time-series chart. Confirm every other
   time-series chart on the page updates to the same window.
5. Expand a previously-collapsed `<details>` section that contains a chart;
   confirm the chart already shows the zoomed window.
6. Click the `⛶` reset-zoom button on any chart. Confirm every chart resets to
   its full data extent.
7. Check the legend pills under each chart: each pill displays Min, Max, and
   Avg per series. Zoom in to a sub-range and confirm the values recompute.
8. Confirm the InnoDB HLL sparkline does NOT change when other charts are
   zoomed (it intentionally opts out of sync).

## Re-run determinism

```bash
./my-gather -in testdata/<fixture> -out /tmp/a.html
./my-gather -in testdata/<fixture> -out /tmp/b.html
diff -u /tmp/a.html /tmp/b.html   # must be empty
```

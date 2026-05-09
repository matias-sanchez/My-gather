# Quickstart: Remove Collection-Size Cap

## Validate the change locally

```sh
# 1. Build and run all tests, including the new streaming regression.
go test -count=1 ./...

# 2. Run go vet.
go vet ./...

# 3. Run the constitution guard.
bash scripts/hooks/pre-push-constitution-guard.sh

# 4. Confirm the cap identifiers are gone.
grep -rn "MaxCollectionBytes\|SizeErrorTotal\|DefaultMaxCollectionBytes" \
    parse/ model/ cmd/ render/ findings/ reportutil/ \
    || echo "OK: no occurrences"
```

## Verify the user-visible change

A previously-rejected capture is now parsed:

```sh
# Pre-feature behavior on a 1.63 GB capture:
#   my-gather: collection size 1630757735 bytes exceeds 1073741824-byte limit at <path>
#   exit 6

# Post-feature behavior:
go run ./cmd/my-gather -input /path/to/large/capture -out /tmp/r.html
# exit 0; report written.
```

## Acceptance for issue #50

- `go test -count=1 ./...` is green, including the new
  `>1.1 GiB` streaming regression in `parse/streaming_large_test.go`.
- `parse.Discover` returns no `*SizeError{Kind: SizeErrorTotal}` —
  the kind no longer exists.
- The CLI no longer prints
  `collection size N bytes exceeds 1073741824-byte limit`.
- Peak heap delta during the regression test stays below 256 MiB
  even though the input exceeds 1.1 GiB.

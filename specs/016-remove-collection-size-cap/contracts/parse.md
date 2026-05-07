# Contract: `parse.Discover` after the cap removal

## Inputs (unchanged)

- `ctx context.Context`
- `rootDir string`
- `opts DiscoverOptions{ Sink, MaxFileBytes }` — note
  `MaxCollectionBytes` is **removed**.

## Outputs

- `(*model.Collection, error)`. Error cases (after this feature):
  - `*PathError` — root path missing/unreadable/not a directory.
  - `ErrNotAPtStalkDir` — no recognised pt-stalk signals at the root.
  - `*SizeError{Kind: SizeErrorFile, ...}` — at least one individual
    source file exceeded `MaxFileBytes`.
  - `ctx.Err()` — context cancelled mid-walk or mid-parse.
  - Any wrapped read error from `os.ReadDir` or `os.Stat`.

The previously-returned `*SizeError{Kind: SizeErrorTotal, ...}` is
**no longer possible**.

## Removed identifiers (must not exist after this feature)

- `parse.DefaultMaxCollectionBytes`
- `parse.DiscoverOptions.MaxCollectionBytes`
- `parse.SizeErrorTotal` (enum value)
- The `SizeErrorTotal` arm of `parse.SizeError.Error`
- The total-collection check in `Discover`:
  `if totalBytes > maxCollection { ... }`
- The `SizeErrorTotal` arm of `cmd/my-gather` `mapDiscoverError`

## Invariants newly enforced by test

- For an input directory whose total size exceeds 1.1 GiB,
  `Discover` returns `(*model.Collection, nil)` with no `*SizeError`
  of any kind, provided no individual file exceeds `MaxFileBytes` and
  the directory is recognisable as a pt-stalk root.
- During the call, peak in-process heap delta (after a forced GC)
  stays below 256 MiB, demonstrating that no parser stage buffers the
  whole collection.

## Streaming guarantee (audited, not enforced by code)

Every per-collector parser registered in `runOneParser`
(`parseIostat`, `parseTop`, `parseVmstat`, `parseMeminfo`,
`parseVariables`, `parseInnodbStatus`, `parseMysqladmin`,
`parseProcesslist`, `parseNetstat`, `parseNetstatS`) consumes its
source file via `io.Reader` + the package-shared `newLineScanner`
(token cap 32 MiB). The streaming-regression test pins this property
by measurement.

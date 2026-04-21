# pt-mext worked-example fixture

Source: `_references/pt-mext/pt-mext-improved.cpp` comment block at the
tail of that file.

- `input.txt` — verbatim copy of the example input at lines 146–177 of
  the C++ file.
- `expected.txt` — verbatim copy of the expected output at lines
  181–189 of the C++ file.

These two files together are the normative correctness anchor for the
Go `-mysqladmin` parser's delta algorithm (spec FR-028, research R8).
The Go implementation, when given `input.txt`, MUST produce the
per-counter aggregates (total / min / max / avg) shown in
`expected.txt`.

**Comparison strategy**: structural (F10 resolution). Whitespace
normalisation is permitted between the Go output and `expected.txt`
because the C++ comment-block formatting uses visual spacing that is
not guaranteed to match the C++ program's actual stdout. The parser
test (T065) parses both into a `map[varname] -> {total, min, max, avg}`
and compares element-wise.

To regenerate `expected.txt` from an actual pt-mext run:

```bash
c++ -O2 -std=c++17 \
  ../../_references/pt-mext/pt-mext-improved.cpp \
  -o /tmp/pt-mext
/tmp/pt-mext input.txt > expected.txt
```

Re-run whenever the C++ reference changes, which should be approximately
never.

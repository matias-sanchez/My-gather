package render_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/matias-sanchez/My-gather/tests/goldens"
)

// TestCollapsePersistenceRoundTrip: T034 / SC-010 / Principle VIII.
//
// SC-010 requires every `<details>` collapse state in the report to
// persist across page reloads, scoped so reports from different
// collections don't share state. The shipped implementation in
// render/assets/app.js uses a `collapseKey(id)` helper that returns
//
//     "mygather:" + REPORT_ID + ":collapse:" + sectionId
//
// — a `REPORT_ID`-namespaced key that the round-trip save / load
// pair reads and writes via `storageGet` / `storageSet` (which wrap
// localStorage with graceful fallbacks when it's unavailable).
//
// A real browser-level round-trip would need a JS harness
// (jsdom / Playwright); Principle X rejects both. Instead the test
// walks the JS source and pins the five moving parts that make the
// round trip correct:
//
//   1. `storageGet` / `storageSet` helpers exist.
//   2. Keys include the `REPORT_ID` substring (so two reports in the
//      same browser don't share collapse state — SC-010's primary
//      invariant).
//   3. A `collapseKey` function builds the namespaced key.
//   4. On toggle, `storageSet(collapseKey(d.id), d.open ? "open" : "closed")`
//      records the state.
//   5. On load, `storageGet(collapseKey(d.id))` is read and applied
//      to `d.open`.
//
// The five tokens together form a save-then-load round trip. A
// regression that removed any ONE of them would break persistence
// silently; a developer deleting the handler in a refactor fails
// this test immediately.
func TestCollapsePersistenceRoundTrip(t *testing.T) {
	root := goldens.RepoRoot(t)
	src, err := os.ReadFile(filepath.Join(root, "render", "assets", "app.js"))
	if err != nil {
		t.Fatalf("read render/assets/app.js: %v", err)
	}
	js := string(src)

	tokens := []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{"storageGet helper", regexp.MustCompile(`function\s+storageGet\s*\(`)},
		{"storageSet helper", regexp.MustCompile(`function\s+storageSet\s*\(`)},
		{"REPORT_ID symbol", regexp.MustCompile(`\bREPORT_ID\b`)},
		{"collapseKey function", regexp.MustCompile(`function\s+collapseKey\s*\(`)},
		{"REPORT_ID inside collapseKey body", regexp.MustCompile(`(?s)function\s+collapseKey\s*\([^)]*\)\s*\{[^{}]*REPORT_ID[^{}]*\}`)},
		{"collapse:key namespace token in a key-building expression", regexp.MustCompile(`["']mygather:["']\s*\+\s*REPORT_ID|REPORT_ID\s*\+\s*["']:collapse:["']`)},
	}
	for _, tok := range tokens {
		if !tok.pattern.MatchString(js) {
			t.Errorf("render/assets/app.js is missing the %q marker for SC-010 collapse persistence; regex %s did not match",
				tok.name, tok.pattern)
		}
	}

	// Save path: SOMEWHERE in the file, `storageSet(collapseKey(…),
	// …)` must appear. That's the write half of the round trip.
	saveRE := regexp.MustCompile(`storageSet\s*\(\s*collapseKey\s*\(`)
	if !saveRE.MatchString(js) {
		t.Errorf("no `storageSet(collapseKey(…), …)` call in app.js; the save half of the collapse round-trip is missing")
	}

	// Load path: SOMEWHERE in the file, `storageGet(collapseKey(…))`
	// must appear and be used to set a details element's `.open`
	// attribute (directly, or via conditional logic). Proximity
	// check: within 400 chars of the storageGet call, the source
	// must reference `.open` (the DOM property used to drive the
	// element's collapse state).
	loadRE := regexp.MustCompile(`storageGet\s*\(\s*collapseKey\s*\(`)
	loc := loadRE.FindStringIndex(js)
	if loc == nil {
		t.Fatalf("no `storageGet(collapseKey(…))` call in app.js; the load half of the collapse round-trip is missing")
	}
	// Window around the storageGet call. 400 chars is generous
	// enough to absorb the typical if/else that applies the saved
	// value, tight enough to fail if the load lives in a completely
	// unrelated part of the file.
	start := loc[0]
	end := loc[1] + 400
	if end > len(js) {
		end = len(js)
	}
	window := js[start:end]
	if !strings.Contains(window, ".open") {
		t.Errorf("storageGet(collapseKey(…)) at offset %d is not followed by a `.open` property reference within 400 chars; the load path isn't applying the saved state to a <details> element", start)
	}
}

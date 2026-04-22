package render_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/matias-sanchez/My-gather/tests/goldens"
)

// TestKeyboardShortcutWiring: T104 / FR-036 / Principle VIII.
//
// FR-036 requires the report to expose a keyboard shortcut that
// collapses / expands the left-hand nav rail: `Cmd+\` on macOS,
// `Ctrl+\` elsewhere (the two modifier keys share a code path). The
// shipped implementation lives in `render/assets/app.js`; a real
// browser-level test would need a JS harness (jsdom, Playwright,
// or similar) which the repo explicitly rejects per Principle X.
//
// Instead the test asserts the presence of the shortcut wiring in
// the JS source: a keydown handler that
//
//   - checks for the Cmd OR Ctrl modifier (`metaKey || ctrlKey`);
//   - checks the `key` property against `"\\"` (backslash, which in
//     a JS string literal is written `"\\\\"`);
//   - calls `preventDefault()` so the browser's default action for
//     the combo (navigate back in some UAs) does not fire;
//   - toggles the nav-hidden state (asserted indirectly via the
//     `classList.contains("nav-hidden")` check plus the
//     storage-update call that follows).
//
// Source-level assertions are weaker than a real behaviour test but
// strictly stronger than nothing: a developer who deletes the
// handler in a refactor will fail the build immediately, and the
// error message points at exactly which invariant slipped.
//
// If a future task (T034 collapse-persistence harness) introduces a
// real JS harness, this test can be rewritten to dispatch a
// synthetic KeyboardEvent and assert the class toggled.
func TestKeyboardShortcutWiring(t *testing.T) {
	root := goldens.RepoRoot(t)
	src, err := os.ReadFile(filepath.Join(root, "render", "assets", "app.js"))
	if err != nil {
		t.Fatalf("read render/assets/app.js: %v", err)
	}
	js := string(src)

	// Find the keydown handler whose body references `\\`. The
	// shipped code uses a single addEventListener block that checks
	// for both metaKey and ctrlKey plus `e.key === "\\"`.
	// Regex-scan for the combination of tokens rather than the
	// specific indentation / variable names so a minor refactor
	// doesn't break the test.
	tokens := []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{"addEventListener keydown", regexp.MustCompile(`addEventListener\(\s*["']keydown["']`)},
		{"modifier check (Cmd or Ctrl)", regexp.MustCompile(`metaKey\s*\|\|\s*[a-zA-Z_.$]*ctrlKey|ctrlKey\s*\|\|\s*[a-zA-Z_.$]*metaKey`)},
		{`backslash "\\" literal`, regexp.MustCompile(`key\s*===\s*"\\\\"`)},
		{"preventDefault", regexp.MustCompile(`preventDefault\s*\(\)`)},
		{"nav-hidden class toggle", regexp.MustCompile(`classList\.contains\(\s*["']nav-hidden["']\s*\)`)},
	}
	for _, tok := range tokens {
		if !tok.pattern.MatchString(js) {
			t.Errorf("render/assets/app.js is missing the %q marker for the Cmd/Ctrl+\\ shortcut (FR-036); regex %s did not match", tok.name, tok.pattern)
		}
	}

	// The previous checks are token-level: each token can live
	// anywhere in the file. Tighten by requiring that the modifier
	// check AND the backslash-key check co-locate inside a single
	// keydown handler body. The handler is bounded by a `function`
	// keyword open-paren and the next `}` that closes the callback;
	// we look for the modifier-and-backslash combo inside the body
	// of ONE such handler.
	//
	// A co-location failure typically means the FR-036 handler was
	// split across two functions, which breaks behaviour because
	// preventDefault runs on a different event object than the
	// toggle logic.
	handlerRE := regexp.MustCompile(`addEventListener\(\s*"keydown",\s*function\s*\([^)]*\)\s*\{([^{}]|\{[^{}]*\})*\}`)
	handlers := handlerRE.FindAllString(js, -1)
	if len(handlers) == 0 {
		t.Fatalf("no keydown addEventListener handler bodies extracted from app.js")
	}
	coLocated := false
	for _, body := range handlers {
		hasMod := regexp.MustCompile(`metaKey\s*\|\|\s*[a-zA-Z_.$]*ctrlKey|ctrlKey\s*\|\|\s*[a-zA-Z_.$]*metaKey`).MatchString(body)
		hasBackslash := regexp.MustCompile(`key\s*===\s*"\\\\"`).MatchString(body)
		if hasMod && hasBackslash {
			coLocated = true
			break
		}
	}
	if !coLocated {
		t.Errorf("no single keydown handler in app.js carries BOTH the (meta||ctrl) modifier check AND the backslash-key check; FR-036 requires them in the same callback")
	}

	// Belt-and-suspenders: the handler must be bound to
	// `document.addEventListener`, not a narrower scope such as a
	// specific section. A body-scoped keydown listener would miss
	// keyboard events when the focus is on a child element that
	// stops propagation. Check the raw source for the exact
	// `document.addEventListener("keydown"` prefix and that at
	// least ONE such binding lives reasonably close to a
	// `"nav-hidden"` reference (same 200-char window).
	idx := strings.Index(js, `document.addEventListener("keydown"`)
	for idx != -1 {
		end := idx + 500
		if end > len(js) {
			end = len(js)
		}
		window := js[idx:end]
		if strings.Contains(window, `"nav-hidden"`) {
			return // found the FR-036 handler bound to document
		}
		next := strings.Index(js[idx+1:], `document.addEventListener("keydown"`)
		if next == -1 {
			break
		}
		idx += 1 + next
	}
	t.Errorf("no document-level keydown handler in app.js sits within 500 chars of a \"nav-hidden\" class reference; FR-036 expects the shortcut to toggle the nav rail via the document-level listener")
}

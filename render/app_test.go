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
//     a JS string literal is written `"\\"` — two source characters
//     for one backslash);
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
		{`backslash "\\" literal`, regexp.MustCompile(`key\s*===\s*(?:"\\\\"|'\\\\')`)},
		{"preventDefault", regexp.MustCompile(`preventDefault\s*\(\)`)},
		{"nav-hidden class toggle", regexp.MustCompile(`classList\.contains\(\s*["']nav-hidden["']\s*\)`)},
	}
	for _, tok := range tokens {
		if !tok.pattern.MatchString(js) {
			t.Errorf("render/assets/app.js is missing the %q marker for the Cmd/Ctrl+\\ shortcut (FR-036); regex %s did not match", tok.name, tok.pattern)
		}
	}

	// The previous checks are token-level: each token can live
	// anywhere in the file. Tighten by requiring that EVERY FR-036
	// moving part lives inside the SAME handler body, AND that that
	// handler is bound to `document.addEventListener(...)` — not to
	// a narrower scope like a section-local element (which would
	// miss keydown events on children that stop propagation).
	//
	// Consolidating the handler-content check and the document-scope
	// check into a single per-handler scan is the test's strongest
	// assertion: a refactor that accidentally moves the shortcut
	// logic onto a non-document listener while leaving an unrelated
	// `document.addEventListener("keydown")` near a `nav-hidden`
	// reference would have passed an earlier split-check version of
	// this test. With the unified scan, the same regression fails
	// here — the only way past is one document-bound handler that
	// carries all four FR-036 content invariants inside its body.
	//
	// Accept three callback shapes — `function (…) { … }`, a
	// parenthesised arrow `(…) => { … }`, and a bare single-arg
	// arrow `ident => { … }` — so a behaviour-preserving refactor
	// to arrow syntax (or single-quoted event names) doesn't false-
	// fail this test. All three forms still enclose a `{ … }` body
	// which the regex captures via one-level brace nesting (enough
	// for the shipped handler's single nested `if` block).
	docHandlerRE := regexp.MustCompile(`document\.addEventListener\(\s*["']keydown["']\s*,\s*(?:function\s*\([^)]*\)|\([^)]*\)\s*=>|[a-zA-Z_$][\w$]*\s*=>)\s*\{([^{}]|\{[^{}]*\})*\}`)
	docHandlers := docHandlerRE.FindAllString(js, -1)
	if len(docHandlers) == 0 {
		t.Fatalf("no `document.addEventListener(\"keydown\", …)` handler bodies extracted from app.js (accepted shapes: function-expression or arrow, single or double quotes); FR-036 requires a document-level listener")
	}
	wired := false
	for _, body := range docHandlers {
		hasMod := regexp.MustCompile(`metaKey\s*\|\|\s*[a-zA-Z_.$]*ctrlKey|ctrlKey\s*\|\|\s*[a-zA-Z_.$]*metaKey`).MatchString(body)
		hasBackslash := regexp.MustCompile(`key\s*===\s*(?:"\\\\"|'\\\\')`).MatchString(body)
		hasPreventDefault := regexp.MustCompile(`\bpreventDefault\s*\(\s*\)`).MatchString(body)
		hasNavHidden := strings.Contains(body, `"nav-hidden"`) || strings.Contains(body, `'nav-hidden'`)
		if hasMod && hasBackslash && hasPreventDefault && hasNavHidden {
			wired = true
			break
		}
	}
	if !wired {
		t.Errorf("no `document.addEventListener(\"keydown\", …)` handler in app.js carries ALL of: (meta||ctrl) modifier check, backslash-key check, preventDefault(), and a \"nav-hidden\" class interaction inside the same callback body. FR-036 requires all four invariants and document-level scope on a single handler.")
	}
}

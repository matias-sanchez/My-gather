package render

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestBuildFeedbackViewIsDeterministic(t *testing.T) {
	a := BuildFeedbackView()
	b := BuildFeedbackView()
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("BuildFeedbackView not deterministic: %#v vs %#v", a, b)
	}
}

func TestFeedbackViewShape(t *testing.T) {
	v := BuildFeedbackView()
	if !reflect.DeepEqual(v.Categories, canonicalFeedbackContract.Categories) {
		t.Errorf("Categories: got %#v, want %#v", v.Categories, canonicalFeedbackContract.Categories)
	}
	var parsed feedbackContract
	if err := json.Unmarshal([]byte(v.ContractJSON), &parsed); err != nil {
		t.Fatalf("ContractJSON is not valid JSON: %v", err)
	}
	if !reflect.DeepEqual(parsed, canonicalFeedbackContract) {
		t.Errorf("ContractJSON: got %#v, want %#v", parsed, canonicalFeedbackContract)
	}
}

func TestFeedbackViewCategoriesAreIsolated(t *testing.T) {
	a := BuildFeedbackView()
	a.Categories[0] = "TAMPERED"
	b := BuildFeedbackView()
	if b.Categories[0] != canonicalFeedbackContract.Categories[0] {
		t.Errorf("mutating one view's Categories leaked to another: got %q", b.Categories[0])
	}
}

func TestFeedbackClientReadsCanonicalContract(t *testing.T) {
	wantSnippets := []string{
		"dialog.dataset.feedbackContract",
		"LIMITS.titleMaxChars",
		"LIMITS.bodyMaxBytes",
		"LIMITS.reportVersionMaxChars",
		"LIMITS.legacyUrlMaxChars",
		"LIMITS.workerTimeoutMs",
		"LIMITS.authorMaxChars",
	}
	for _, snippet := range wantSnippets {
		if !strings.Contains(embeddedAppJS, snippet) {
			t.Errorf("embedded app JS does not consume feedback contract snippet %q", snippet)
		}
	}
}

// TestFeedbackViewExposesAuthorMaxChars: spec 021-feedback-author-field
// FR-009 — the Author character cap MUST be expressed exactly once
// in the canonical contract JSON and surfaced by the Go view so the
// rendered <input maxlength="..."> matches the worker's validation
// limit. Asserts both that the view carries the value and that it
// equals the parsed contract field (no hardcoded constant).
func TestFeedbackViewExposesAuthorMaxChars(t *testing.T) {
	v := BuildFeedbackView()
	if v.AuthorMaxChars <= 0 {
		t.Fatalf("FeedbackView.AuthorMaxChars must be positive, got %d", v.AuthorMaxChars)
	}
	if v.AuthorMaxChars != canonicalFeedbackContract.Limits.AuthorMaxChars {
		t.Errorf("FeedbackView.AuthorMaxChars = %d, want canonical contract %d",
			v.AuthorMaxChars, canonicalFeedbackContract.Limits.AuthorMaxChars)
	}
}

// TestFeedbackContractParserValidatesAuthorMaxChars: spec
// 021-feedback-author-field FR-009 / Copilot review on PR #61 —
// parseFeedbackContract must validate limits.authorMaxChars as a
// positive integer alongside the other limits.* fields. Without
// this, a malformed embedded contract could leave LIMITS.authorMaxChars
// undefined and the Submit gate's `> LIMITS.authorMaxChars`
// comparison would silently degrade (NaN comparisons evaluate
// false, disabling the cap entirely).
func TestFeedbackContractParserValidatesAuthorMaxChars(t *testing.T) {
	mustContain := func(snippet string) {
		t.Helper()
		if !strings.Contains(embeddedAppJS, snippet) {
			t.Errorf("embedded app JS missing parser-validation snippet %q", snippet)
		}
	}
	// The parser must reject contracts whose authorMaxChars is
	// missing, non-numeric, non-finite, non-integer, or non-positive.
	mustContain(`typeof limits.authorMaxChars !== "number"`)
	mustContain(`!isFinite(limits.authorMaxChars)`)
	mustContain(`Math.floor(limits.authorMaxChars) !== limits.authorMaxChars`)
	mustContain(`limits.authorMaxChars <= 0`)
}

// TestFeedbackPersistsSubmittedAuthorSnapshot: spec
// 021-feedback-author-field US2 / Codex review on PR #61 — the
// success arm of doSubmit must persist the author value captured
// at submit time (and sent in the POST), not whatever is sitting
// in authorInput.value when the worker's response arrives. If the
// user edits the field between Submit and the success callback,
// the persisted value would otherwise diverge from the issue
// body's "Submitted by:" line.
//
// Asserts structural facts in the embedded JS:
//
//  1. doSubmit captures the trimmed author into a local snapshot
//     before constructing the payload (`authorAtSubmit = ...trim()`).
//  2. The payload's `author:` field is sourced from that snapshot,
//     not from a fresh `authorInput.value` read.
//  3. savePersistedAuthor is invoked with the snapshot
//     (`r.authorAtSubmit`), not `authorInput.value`.
func TestFeedbackPersistsSubmittedAuthorSnapshot(t *testing.T) {
	mustContain := func(label, snippet string) {
		t.Helper()
		if !strings.Contains(embeddedAppJS, snippet) {
			t.Errorf("%s: embedded app JS missing required snippet %q", label, snippet)
		}
	}
	mustNotContain := func(label, snippet string) {
		t.Helper()
		if strings.Contains(embeddedAppJS, snippet) {
			t.Errorf("%s: embedded app JS still contains forbidden snippet %q", label, snippet)
		}
	}
	mustContain("snapshot capture", `var authorAtSubmit = authorInput.value.trim();`)
	mustContain("payload uses snapshot", `author: authorAtSubmit,`)
	mustContain("success arm uses snapshot", `savePersistedAuthor(r.authorAtSubmit);`)
	// The pre-fix bug was savePersistedAuthor(authorInput.value.trim())
	// inside the success arm. Guard against regression.
	mustNotContain("success arm reads live input",
		`savePersistedAuthor(authorInput.value.trim());`)
}

// TestFeedbackPersistedAuthorKeyReferencedConsistently: spec
// 021-feedback-author-field US2 — the localStorage key
// "mygather.feedback.lastAuthor" must appear in the embedded JS
// for both the read (loadPersistedAuthor → getItem) AND the write
// (savePersistedAuthor → setItem). A typo on either side would
// silently break pre-fill. Asserts a structural pairing rather
// than a substring count so an unrelated comment mentioning the
// key cannot satisfy the test.
func TestFeedbackPersistedAuthorKeyReferencedConsistently(t *testing.T) {
	const key = `"mygather.feedback.lastAuthor"`
	getCall := `getItem(` + key + `)`
	setCall := `setItem(` + key + `,`
	if !strings.Contains(embeddedAppJS, getCall) {
		t.Errorf("embedded app JS must read the persisted author via %s", getCall)
	}
	if !strings.Contains(embeddedAppJS, setCall) {
		t.Errorf("embedded app JS must write the persisted author via %s<value>)", setCall)
	}
}

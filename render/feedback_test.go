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

// TestFeedbackPersistedAuthorKeyReferencedConsistently: spec
// 021-feedback-author-field US2 — the localStorage key
// "mygather.feedback.lastAuthor" must appear in the embedded JS
// for both the read (loadPersistedAuthor) and the write
// (savePersistedAuthor). A typo on either side would silently break
// pre-fill. Counts the literal occurrences and asserts >= 2 so a
// future code-comment mentioning the key does not over-flag.
func TestFeedbackPersistedAuthorKeyReferencedConsistently(t *testing.T) {
	const key = `"mygather.feedback.lastAuthor"`
	count := strings.Count(embeddedAppJS, key)
	if count < 2 {
		t.Fatalf("embedded app JS must reference %s at least twice (read + write), got %d", key, count)
	}
}

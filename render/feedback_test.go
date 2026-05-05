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
	}
	for _, snippet := range wantSnippets {
		if !strings.Contains(embeddedAppJS, snippet) {
			t.Errorf("embedded app JS does not consume feedback contract snippet %q", snippet)
		}
	}
}

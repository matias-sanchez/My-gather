package render

import (
	"reflect"
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
	wantURL := "https://github.com/matias-sanchez/My-gather/issues/new?labels=user-feedback,needs-triage"
	if v.GitHubURL != wantURL {
		t.Errorf("GitHubURL: got %q, want %q", v.GitHubURL, wantURL)
	}
	wantWorker := "https://my-gather-feedback.mati-orfeo.workers.dev/feedback"
	if v.WorkerURL != wantWorker {
		t.Errorf("WorkerURL: got %q, want %q", v.WorkerURL, wantWorker)
	}
	wantCats := []string{"UI", "Parser", "Advisor", "Other"}
	if !reflect.DeepEqual(v.Categories, wantCats) {
		t.Errorf("Categories: got %#v, want %#v", v.Categories, wantCats)
	}
}

func TestFeedbackViewCategoriesAreIsolated(t *testing.T) {
	a := BuildFeedbackView()
	a.Categories[0] = "TAMPERED"
	b := BuildFeedbackView()
	if b.Categories[0] != "UI" {
		t.Errorf("mutating one view's Categories leaked to another: got %q", b.Categories[0])
	}
}

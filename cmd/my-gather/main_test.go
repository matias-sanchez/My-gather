package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallRenderedFileDoesNotOverwriteExistingOutput(t *testing.T) {
	dir := t.TempDir()
	tmpPath := filepath.Join(dir, ".report.html.tmp")
	outPath := filepath.Join(dir, "report.html")

	if err := os.WriteFile(tmpPath, []byte("new report"), 0o600); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	if err := os.WriteFile(outPath, []byte("existing report"), 0o600); err != nil {
		t.Fatalf("write existing output: %v", err)
	}

	err := installRenderedFile(tmpPath, outPath, false)
	if !errors.Is(err, errOutputExists) {
		t.Fatalf("installRenderedFile error = %v, want errOutputExists", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(got) != "existing report" {
		t.Fatalf("output was overwritten: got %q", got)
	}
	if _, err := os.Stat(tmpPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temp file should be cleaned up, stat err = %v", err)
	}
}

func TestInstallRenderedFileLinksCompletedOutputWhenNotOverwriting(t *testing.T) {
	dir := t.TempDir()
	tmpPath := filepath.Join(dir, ".report.html.tmp")
	outPath := filepath.Join(dir, "report.html")

	if err := os.WriteFile(tmpPath, []byte("new report"), 0o600); err != nil {
		t.Fatalf("write temp: %v", err)
	}

	if err := installRenderedFile(tmpPath, outPath, false); err != nil {
		t.Fatalf("installRenderedFile: %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(got) != "new report" {
		t.Fatalf("output content = %q, want new report", got)
	}
	if _, err := os.Stat(tmpPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temp file should be removed after successful install, stat err = %v", err)
	}
}

func TestInstallRenderedFileOverwritesWhenRequested(t *testing.T) {
	dir := t.TempDir()
	tmpPath := filepath.Join(dir, ".report.html.tmp")
	outPath := filepath.Join(dir, "report.html")

	if err := os.WriteFile(tmpPath, []byte("new report"), 0o600); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	if err := os.WriteFile(outPath, []byte("existing report"), 0o600); err != nil {
		t.Fatalf("write existing output: %v", err)
	}

	if err := installRenderedFile(tmpPath, outPath, true); err != nil {
		t.Fatalf("installRenderedFile: %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(got) != "new report" {
		t.Fatalf("output content = %q, want new report", got)
	}
}

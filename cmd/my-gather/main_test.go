package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matias-sanchez/My-gather/parse"
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

func TestRunAcceptsZipArchiveInput(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "capture.zip")
	writeZipFixture(t, archivePath, "nested/pt-stalk")
	outPath := filepath.Join(dir, "report.html")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--overwrite", "-o", outPath, archivePath}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("run exit = %d, want %d\nstderr:\n%s", code, exitOK, stderr.String())
	}
	if st, err := os.Stat(outPath); err != nil || st.Size() == 0 {
		t.Fatalf("output stat = (%v, %v), want non-empty report", st, err)
	}
}

func TestRunAcceptsTarGzipArchiveInput(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "capture.tar.gz")
	writeTarGzipFixture(t, archivePath, "pt-stalk")
	outPath := filepath.Join(dir, "report.html")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--overwrite", "-o", outPath, archivePath}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("run exit = %d, want %d\nstderr:\n%s", code, exitOK, stderr.String())
	}
	if st, err := os.Stat(outPath); err != nil || st.Size() == 0 {
		t.Fatalf("output stat = (%v, %v), want non-empty report", st, err)
	}
}

func TestRunAcceptsGzipArchiveContainingTarStream(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "capture.tar_abc123.gz")
	writeTarGzipFixture(t, archivePath, "pt-stalk")
	outPath := filepath.Join(dir, "report.html")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--overwrite", "-o", outPath, archivePath}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("run exit = %d, want %d\nstderr:\n%s", code, exitOK, stderr.String())
	}
	if st, err := os.Stat(outPath); err != nil || st.Size() == 0 {
		t.Fatalf("output stat = (%v, %v), want non-empty report", st, err)
	}
}

func TestRunRejectsArchivePathTraversal(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	t.Setenv("TEMP", dir)
	archivePath := filepath.Join(dir, "bad.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	zw := zip.NewWriter(file)
	entry, err := zw.Create("../escape")
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := entry.Write([]byte("bad")); err != nil {
		t.Fatalf("write zip entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--overwrite", "-o", filepath.Join(dir, "report.html"), archivePath}, &stdout, &stderr)
	if code != exitInputPath {
		t.Fatalf("run exit = %d, want %d\nstderr:\n%s", code, exitInputPath, stderr.String())
	}
	if !strings.Contains(stderr.String(), "unsafe archive") {
		t.Fatalf("stderr = %q, want unsafe archive message", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "escape")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("path traversal created file, stat err = %v", err)
	}
}

func TestRunAcceptsTarArchiveWithRootDirectoryEntry(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "capture.tar.gz")
	writeTarGzipFixtureWithOptions(t, archivePath, "pt-stalk", true)
	outPath := filepath.Join(dir, "report.html")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--overwrite", "-o", outPath, archivePath}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("run exit = %d, want %d\nstderr:\n%s", code, exitOK, stderr.String())
	}
	if st, err := os.Stat(outPath); err != nil || st.Size() == 0 {
		t.Fatalf("output stat = (%v, %v), want non-empty report", st, err)
	}
}

func TestRunMapsInvalidZipToInputPathError(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "broken.zip")
	if err := os.WriteFile(archivePath, []byte("not a zip"), 0o600); err != nil {
		t.Fatalf("write broken zip: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--overwrite", "-o", filepath.Join(dir, "report.html"), archivePath}, &stdout, &stderr)
	if code != exitInputPath {
		t.Fatalf("run exit = %d, want %d\nstderr:\n%s", code, exitInputPath, stderr.String())
	}
	if !strings.Contains(stderr.String(), "invalid archive input") {
		t.Fatalf("stderr = %q, want invalid archive input message", stderr.String())
	}
}

func TestWriteExtractedFileEnforcesPerFileLimit(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "large")
	var written int64

	err := writeExtractedFileWithLimits(target, 0o600, strings.NewReader("abcdef"), &written, 100, 5)
	var sizeErr *parse.SizeError
	if !errors.As(err, &sizeErr) {
		t.Fatalf("error = %v, want parse.SizeError", err)
	}
	if sizeErr.Kind != parse.SizeErrorFile {
		t.Fatalf("size error kind = %v, want %v", sizeErr.Kind, parse.SizeErrorFile)
	}
}

func TestRunRejectsArchiveWithMultiplePtStalkRoots(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "multi.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	zw := zip.NewWriter(file)
	for _, name := range []string{
		"a/2026_04_21_16_51_41-top",
		"b/2026_04_21_16_51_41-top",
	} {
		entry, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := entry.Write([]byte("TS 1776790303.009325313 2026-04-21 16:51:43\n")); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--overwrite", "-o", filepath.Join(dir, "report.html"), archivePath}, &stdout, &stderr)
	if code != exitNotAPtStalkDir {
		t.Fatalf("run exit = %d, want %d\nstderr:\n%s", code, exitNotAPtStalkDir, stderr.String())
	}
	if !strings.Contains(stderr.String(), "multiple pt-stalk collections") {
		t.Fatalf("stderr = %q, want multiple-root message", stderr.String())
	}
}

func writeZipFixture(t *testing.T, archivePath, prefix string) {
	t.Helper()
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	defer func() {
		if err := zw.Close(); err != nil {
			t.Fatalf("close zip: %v", err)
		}
	}()
	walkFixture(t, func(srcPath, rel string, info os.FileInfo) {
		if info.IsDir() {
			return
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			t.Fatalf("zip header %s: %v", rel, err)
		}
		header.Name = filepath.ToSlash(filepath.Join(prefix, rel))
		writer, err := zw.CreateHeader(header)
		if err != nil {
			t.Fatalf("zip create %s: %v", rel, err)
		}
		copyFileToWriter(t, srcPath, writer)
	})
}

func writeTarGzipFixture(t *testing.T, archivePath, prefix string) {
	t.Helper()
	writeTarGzipFixtureWithOptions(t, archivePath, prefix, false)
}

func writeTarGzipFixtureWithOptions(t *testing.T, archivePath, prefix string, includeRootDir bool) {
	t.Helper()
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create tar.gz: %v", err)
	}
	defer file.Close()
	gz := gzip.NewWriter(file)
	defer func() {
		if err := gz.Close(); err != nil {
			t.Fatalf("close gzip: %v", err)
		}
	}()
	tw := tar.NewWriter(gz)
	defer func() {
		if err := tw.Close(); err != nil {
			t.Fatalf("close tar: %v", err)
		}
	}()
	if includeRootDir {
		if err := tw.WriteHeader(&tar.Header{Name: "./", Mode: 0o755, Typeflag: tar.TypeDir}); err != nil {
			t.Fatalf("tar write root header: %v", err)
		}
	}
	walkFixture(t, func(srcPath, rel string, info os.FileInfo) {
		if info.IsDir() {
			return
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			t.Fatalf("tar header %s: %v", rel, err)
		}
		header.Name = filepath.ToSlash(filepath.Join(prefix, rel))
		if err := tw.WriteHeader(header); err != nil {
			t.Fatalf("tar write header %s: %v", rel, err)
		}
		copyFileToWriter(t, srcPath, tw)
	})
}

func walkFixture(t *testing.T, visit func(srcPath, rel string, info os.FileInfo)) {
	t.Helper()
	root := filepath.Join("..", "..", "testdata", "example2")
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		visit(path, rel, info)
		return nil
	})
	if err != nil {
		t.Fatalf("walk fixture: %v", err)
	}
}

func copyFileToWriter(t *testing.T, srcPath string, writer io.Writer) {
	t.Helper()
	src, err := os.Open(srcPath)
	if err != nil {
		t.Fatalf("open fixture %s: %v", srcPath, err)
	}
	defer src.Close()
	if _, err := io.Copy(writer, src); err != nil {
		t.Fatalf("copy fixture %s: %v", srcPath, err)
	}
}

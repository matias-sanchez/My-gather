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

func TestRunMapsCorruptGzipPayloadToInputPathError(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "broken.gz")
	writeTruncatedGzip(t, archivePath)

	var stdout, stderr bytes.Buffer
	code := run([]string{"--overwrite", "-o", filepath.Join(dir, "report.html"), archivePath}, &stdout, &stderr)
	if code != exitInputPath {
		t.Fatalf("run exit = %d, want %d\nstderr:\n%s", code, exitInputPath, stderr.String())
	}
	if !strings.Contains(stderr.String(), "invalid archive input") {
		t.Fatalf("stderr = %q, want invalid archive input message", stderr.String())
	}
}

func TestRunRejectsArchiveWithNoPtStalkRoot(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "unrelated.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	zw := zip.NewWriter(file)
	entry, err := zw.Create("notes/readme.txt")
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := entry.Write([]byte("not a pt-stalk collection")); err != nil {
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
	if code != exitNotAPtStalkDir {
		t.Fatalf("run exit = %d, want %d\nstderr:\n%s", code, exitNotAPtStalkDir, stderr.String())
	}
	if !strings.Contains(stderr.String(), "no pt-stalk collection was found in its subdirectories") {
		t.Fatalf("stderr = %q, want no-root pt-stalk message", stderr.String())
	}
}

func TestRunRejectsUnsupportedRegularInputFile(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "capture.txt")
	if err := os.WriteFile(inputPath, []byte("not an archive"), 0o600); err != nil {
		t.Fatalf("write input: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--overwrite", "-o", filepath.Join(dir, "report.html"), inputPath}, &stdout, &stderr)
	if code != exitInputPath {
		t.Fatalf("run exit = %d, want %d\nstderr:\n%s", code, exitInputPath, stderr.String())
	}
	got := stderr.String()
	if !strings.Contains(got, "not a supported input archive") {
		t.Fatalf("stderr = %q, want unsupported archive message", got)
	}
	for _, format := range []string{".zip", ".tar", ".tar.gz", ".tgz", ".gz"} {
		if !strings.Contains(got, format) {
			t.Fatalf("stderr = %q, want supported format %s", got, format)
		}
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

// TestExtractGzipArchivePassesThroughExtractedSizeError guards the
// canonical size-bound exit path for plain .gz inputs. Before the fix,
// extractGzipArchive only recognised *parse.SizeError as a passthrough
// and wrapped *archiveExtractedSizeError as an archiveInputError, which
// made oversized plain .gz inputs exit through exitInputPath
// ("invalid archive input") instead of exitSizeBound. Both extractors
// (tar and gzip) must surface *archiveExtractedSizeError unwrapped so
// mapInputPreparationError routes them to exitSizeBound.
func TestExtractGzipArchivePassesThroughExtractedSizeError(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "oversized.gz")

	// Build a small plain .gz payload (not a tar). looksLikeTarHeader
	// requires "ustar" at offset 257; arbitrary bytes ensure the gzip
	// extractor takes the single-file decompression branch.
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	gzw := gzip.NewWriter(file)
	payload := bytes.Repeat([]byte("X"), 2048)
	if _, err := gzw.Write(payload); err != nil {
		t.Fatalf("write gzip payload: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}

	destDir := t.TempDir()
	// Force the total-extracted ceiling to bite by exercising the
	// underlying writer with a maxTotal smaller than the payload. We
	// drive the helper directly because the production
	// maxArchiveExtractedBytes is 64 GiB; allocating that much in tests
	// is infeasible. The boundary under test is the gzip extractor's
	// error type discrimination, which is independent of the ceiling
	// magnitude.
	gzReader, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer gzReader.Close()
	gzr, err := gzip.NewReader(gzReader)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gzr.Close()

	target := filepath.Join(destDir, "decompressed")
	var written int64
	writeErr := writeExtractedFileWithLimits(target, 0o600, gzr, &written, 100, 1<<20)
	var extractedSizeErr *archiveExtractedSizeError
	if !errors.As(writeErr, &extractedSizeErr) {
		t.Fatalf("writeExtractedFileWithLimits error = %v, want *archiveExtractedSizeError", writeErr)
	}

	// Now exercise extractGzipArchive end-to-end with a small
	// ceiling so we assert the error type surfaced for an oversize
	// plain .gz. Production calls extractGzipArchive, which delegates
	// to extractGzipArchiveWithLimits — the same canonical path.
	var extractWritten int64
	extractErr := extractGzipArchiveWithLimits(archivePath, t.TempDir(), &extractWritten, 100, 1<<20)
	var sizeErr *archiveExtractedSizeError
	if !errors.As(extractErr, &sizeErr) {
		t.Fatalf("extractGzipArchive error type = %T (%v), want *archiveExtractedSizeError; "+
			"oversized plain .gz inputs must reach mapInputPreparationError unwrapped so "+
			"they route to exitSizeBound, not exitInputPath", extractErr, extractErr)
	}
	var wrapped *archiveInputError
	if errors.As(extractErr, &wrapped) {
		t.Fatalf("extractGzipArchive error = %v also matches *archiveInputError; "+
			"size-bound errors must not be wrapped as invalid-input errors", extractErr)
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

func writeTruncatedGzip(t *testing.T, path string) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write([]byte("truncated payload")); err != nil {
		t.Fatalf("write gzip payload: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	data := buf.Bytes()
	if len(data) < 8 {
		t.Fatalf("gzip fixture too small: %d bytes", len(data))
	}
	if err := os.WriteFile(path, data[:len(data)-4], 0o600); err != nil {
		t.Fatalf("write truncated gzip: %v", err)
	}
}

// copyFixtureDir copies the testdata/example2 fixture into destDir so
// destDir satisfies parse.LooksLikePtStalkRoot. Used by the directory-
// input end-to-end tests below.
func copyFixtureDir(t *testing.T, destDir string) {
	t.Helper()
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", destDir, err)
	}
	walkFixture(t, func(srcPath, rel string, info os.FileInfo) {
		if info.IsDir() {
			return
		}
		dst := filepath.Join(destDir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(dst), err)
		}
		out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			t.Fatalf("create %s: %v", dst, err)
		}
		copyFileToWriter(t, srcPath, out)
		if err := out.Close(); err != nil {
			t.Fatalf("close %s: %v", dst, err)
		}
	})
}

func TestCLIDirInputNestedSingle(t *testing.T) {
	root := t.TempDir()
	tmp := filepath.Join(root, "input")
	// 6 directories below the input root - mirrors the dominant
	// real-world layout (case-folder/host/tmp/pt/collected/host/).
	deep := filepath.Join(tmp, "case", "host", "tmp", "pt", "collected", "host")
	copyFixtureDir(t, deep)
	outPath := filepath.Join(root, "report.html")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--overwrite", "-o", outPath, tmp}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("run exit = %d, want %d\nstderr:\n%s", code, exitOK, stderr.String())
	}
	if st, err := os.Stat(outPath); err != nil || st.Size() == 0 {
		t.Fatalf("output stat = (%v, %v), want non-empty report", st, err)
	}
}

func TestCLIDirInputMultipleRoots(t *testing.T) {
	root := t.TempDir()
	tmp := filepath.Join(root, "input")
	hostA := filepath.Join(tmp, "alpha", "host")
	hostB := filepath.Join(tmp, "beta", "host")
	for _, host := range []string{hostA, hostB} {
		if err := os.MkdirAll(host, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", host, err)
		}
		// Minimal pt-stalk signal; we only need the recognition rule
		// to fire, not a full collection.
		if err := os.WriteFile(filepath.Join(host, "2026_05_08_12_00_00-mysqladmin"), []byte("Uptime: 1\n"), 0o644); err != nil {
			t.Fatalf("write pt-stalk fixture: %v", err)
		}
	}
	outPath := filepath.Join(root, "report.html")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--overwrite", "-o", outPath, tmp}, &stdout, &stderr)
	if code != exitNotAPtStalkDir {
		t.Fatalf("run exit = %d, want %d\nstderr:\n%s", code, exitNotAPtStalkDir, stderr.String())
	}
	if _, err := os.Stat(outPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("output file unexpectedly created: stat err = %v", err)
	}
	stderrText := stderr.String()
	if !strings.Contains(stderrText, "multiple pt-stalk collections") {
		t.Fatalf("stderr = %q, want multi-root message", stderrText)
	}
	wantA, _ := filepath.Abs(hostA)
	wantB, _ := filepath.Abs(hostB)
	idxA := strings.Index(stderrText, wantA)
	idxB := strings.Index(stderrText, wantB)
	if idxA < 0 || idxB < 0 {
		t.Fatalf("stderr missing one root path:\n%s\nwantA=%s wantB=%s", stderrText, wantA, wantB)
	}
	if idxA > idxB {
		t.Fatalf("roots not in lexical order in stderr:\n%s", stderrText)
	}
}

func TestCLIDirInputNoRoot(t *testing.T) {
	root := t.TempDir()
	tmp := filepath.Join(root, "input")
	// Populate the directory with unrelated files at multiple depths
	// so the walk has something to do but finds no pt-stalk root.
	if err := os.MkdirAll(filepath.Join(tmp, "docs", "notes"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	outPath := filepath.Join(root, "report.html")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--overwrite", "-o", outPath, tmp}, &stdout, &stderr)
	if code != exitNotAPtStalkDir {
		t.Fatalf("run exit = %d, want %d\nstderr:\n%s", code, exitNotAPtStalkDir, stderr.String())
	}
	if _, err := os.Stat(outPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("output file unexpectedly created: stat err = %v", err)
	}
	stderrText := stderr.String()
	if !strings.Contains(stderrText, "subdirectories") {
		t.Fatalf("stderr = %q, want message mentioning subdirectories were searched", stderrText)
	}
	if !strings.Contains(stderrText, "depth 8") {
		t.Fatalf("stderr = %q, want depth-limit number in message", stderrText)
	}
}

func TestCLIDirInputTopLevelFastPathUnchanged(t *testing.T) {
	tmp := t.TempDir()
	// Input directory IS the pt-stalk root (existing supported
	// layout); the auto-descent path must not run.
	inputDir := filepath.Join(tmp, "input")
	copyFixtureDir(t, inputDir)
	outPath := filepath.Join(tmp, "report.html")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--overwrite", "-o", outPath, inputDir}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("run exit = %d, want %d\nstderr:\n%s", code, exitOK, stderr.String())
	}
	if st, err := os.Stat(outPath); err != nil || st.Size() == 0 {
		t.Fatalf("output stat = (%v, %v), want non-empty report", st, err)
	}
}

// TestRunArchiveNoRootDropsDepthPhrase guards the Codex round-6 P3
// finding: archive inputs walk with MaxDepth=UnlimitedRootSearchDepth,
// so the CLI's no-root diagnostic must NOT claim "searched up to
// depth 8" for archives. The depth phrase is dropped in that case.
func TestRunArchiveNoRootDropsDepthPhrase(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "unrelated.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	zw := zip.NewWriter(file)
	entry, err := zw.Create("notes/readme.txt")
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := entry.Write([]byte("not a pt-stalk collection")); err != nil {
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
	if code != exitNotAPtStalkDir {
		t.Fatalf("run exit = %d, want %d\nstderr:\n%s", code, exitNotAPtStalkDir, stderr.String())
	}
	out := stderr.String()
	if !strings.Contains(out, "no pt-stalk collection was found in its subdirectories") {
		t.Fatalf("stderr missing subdirectories phrase: %q", out)
	}
	if strings.Contains(out, "depth") {
		t.Fatalf("archive no-root message must not mention depth (it ran unbounded): %q", out)
	}
}

// TestRunDirInputNoRootIncludesDepthPhrase guards the symmetric
// directory-input case: the depth phrase IS rendered there because
// the walker did apply DefaultMaxRootSearchDepth.
func TestRunDirInputNoRootIncludesDepthPhrase(t *testing.T) {
	root := t.TempDir()
	tmp := filepath.Join(root, "input")
	if err := os.MkdirAll(filepath.Join(tmp, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	outPath := filepath.Join(root, "report.html")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--overwrite", "-o", outPath, tmp}, &stdout, &stderr)
	if code != exitNotAPtStalkDir {
		t.Fatalf("run exit = %d, want %d\nstderr:\n%s", code, exitNotAPtStalkDir, stderr.String())
	}
	out := stderr.String()
	if !strings.Contains(out, "depth 8") {
		t.Fatalf("dir no-root message must include depth 8: %q", out)
	}
}

// TestRunAcceptsZipArchiveWithDeeplyNestedRoot guards the Codex
// round-5 regression for archive inputs: a zip whose pt-stalk root is
// nested deeper than the directory-input default depth cap (8) must
// still be accepted, because the pre-feature findExtractedPtStalkRoot
// helper had no depth cap and the archive call site now passes
// MaxDepth=parse.UnlimitedRootSearchDepth + IncludeHidden=true to
// preserve that.
func TestRunAcceptsZipArchiveWithDeeplyNestedRoot(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "deep.zip")
	// 10 levels deep, beyond the directory-input default cap of 8.
	deepPrefix := "a/b/c/d/e/f/g/h/i/j/pt-stalk"
	writeZipFixture(t, archivePath, deepPrefix)
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

// TestRunAcceptsZipArchiveWithRootUnderHiddenDir guards the same
// regression for archives whose pt-stalk root sits beneath a
// hidden-named subdirectory: directory inputs prune those paths, but
// archive inputs (per the pre-feature findExtractedPtStalkRoot
// behaviour) must continue to descend into them.
func TestRunAcceptsZipArchiveWithRootUnderHiddenDir(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "hidden.zip")
	writeZipFixture(t, archivePath, ".cache/pt-stalk")
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

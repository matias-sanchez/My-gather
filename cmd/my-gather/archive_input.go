package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/matias-sanchez/My-gather/parse"
)

// maxArchiveExtractedBytes is a defence-in-depth ceiling on total
// bytes extracted from a single archive input. It is intentionally
// generous (64 GiB) — high enough to accommodate any real-world
// pt-stalk capture (the largest observed are in the low single-digit
// GB range; feature 016-remove-collection-size-cap removed the parser
// 1 GiB total cap to unblock 1.63 GB+ captures), and low enough to
// still bound a runaway extraction (infinite-loop tar with circular
// hard links, pathologically expanding gzip stream). The per-file
// ceiling maxArchiveFileBytes provides the primary defence against
// compression-ratio bombs by capping any single extracted file.
//
// This constant is local to the archive-input boundary; the parser
// no longer has a total-collection bound.
const maxArchiveExtractedBytes int64 = 64 << 30

const maxArchiveFileBytes = parse.DefaultMaxFileBytes

// archiveExtractedSizeError reports that an archive's total
// extracted bytes exceeded the local archive-extraction ceiling
// (maxArchiveExtractedBytes). It is a typed error so callers can
// branch via errors.As; the parser's *SizeError no longer covers a
// total-collection case.
type archiveExtractedSizeError struct {
	Path  string
	Bytes int64
	Limit int64
}

// Error implements the error interface.
func (e *archiveExtractedSizeError) Error() string {
	return fmt.Sprintf("archive extracted size %d bytes exceeds %d-byte limit at %s",
		e.Bytes, e.Limit, e.Path)
}

var errUnsupportedArchive = errors.New("unsupported archive format")

type preparedInput struct {
	parseDir  string
	tempDir   string
	isArchive bool
	cleanup   func()
}

type unsafeArchivePathError struct {
	entry string
}

func (e *unsafeArchivePathError) Error() string {
	return fmt.Sprintf("archive entry %q would extract outside the temporary directory", e.entry)
}

type archiveInputError struct {
	path  string
	entry string
	err   error
}

func (e *archiveInputError) Error() string {
	if e.entry != "" {
		return fmt.Sprintf("archive %s entry %q: %v", e.path, e.entry, e.err)
	}
	return fmt.Sprintf("archive %s: %v", e.path, e.err)
}

func (e *archiveInputError) Unwrap() error { return e.err }

func newArchiveInputError(path, entry string, err error) error {
	return &archiveInputError{path: path, entry: entry, err: err}
}

type archiveFormat int

const (
	archiveUnsupported archiveFormat = iota
	archiveZip
	archiveTar
	archiveTarGzip
	archiveGzip
)

func prepareInput(ctx context.Context, inputPath string) (*preparedInput, error) {
	info, err := os.Stat(inputPath)
	if err != nil {
		return nil, &parse.PathError{Op: "stat", Path: inputPath, Err: err}
	}
	if info.IsDir() {
		// Route directory inputs through the canonical
		// parse.FindPtStalkRoot with default options
		// (DefaultMaxRootSearchDepth = 8, hidden directories skipped).
		// The walker tests rootDir itself against LooksLikePtStalkRoot
		// before descending so the existing already-a-root invocation
		// returns the input directly, and adding any pre-check here
		// would be a duplicate path (Principle XIII). The depth and
		// hidden-dir bounds are appropriate for user-typed paths
		// where bounding accidental misdirection matters.
		root, err := parse.FindPtStalkRoot(ctx, inputPath, parse.FindPtStalkRootOptions{})
		if err != nil {
			return nil, err
		}
		return &preparedInput{parseDir: root, cleanup: func() {}}, nil
	}
	if !info.Mode().IsRegular() {
		return nil, &parse.PathError{Op: "stat", Path: inputPath, Err: errors.New("not a directory or regular archive file")}
	}
	if archiveKind(inputPath) == archiveUnsupported {
		return nil, fmt.Errorf("%w: %s", errUnsupportedArchive, inputPath)
	}

	tempDir, err := os.MkdirTemp("", "my-gather-input-*")
	if err != nil {
		return nil, fmt.Errorf("create extraction temp dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }

	if err := extractArchive(inputPath, tempDir); err != nil {
		cleanup()
		return nil, err
	}
	// Archive inputs walk the extraction tempDir with the depth bound
	// disabled and hidden directories included. The pre-feature
	// findExtractedPtStalkRoot helper had neither cap, so a customer
	// archive whose pt-stalk root nested deeper than 8 levels (e.g.
	// zip-of-zip layouts) or extracted under a hidden-named top-level
	// directory worked then and must keep working now (Codex round-5
	// finding). The total-bytes cap on extraction
	// (maxArchiveExtractedBytes = 1 GiB) and the entry cap inside
	// FindPtStalkRoot still bound resource use.
	root, err := parse.FindPtStalkRoot(ctx, tempDir, parse.FindPtStalkRootOptions{
		MaxDepth:      parse.UnlimitedRootSearchDepth,
		IncludeHidden: true,
	})
	if err != nil {
		cleanup()
		return nil, err
	}
	return &preparedInput{
		parseDir:  root,
		tempDir:   tempDir,
		isArchive: true,
		cleanup:   cleanup,
	}, nil
}

func archiveKind(path string) archiveFormat {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return archiveZip
	case strings.HasSuffix(lower, ".tar"):
		return archiveTar
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return archiveTarGzip
	case strings.HasSuffix(lower, ".gz"):
		return archiveGzip
	default:
		return archiveUnsupported
	}
}

func extractArchive(archivePath, destDir string) error {
	var written int64
	switch archiveKind(archivePath) {
	case archiveZip:
		return extractZipArchive(archivePath, destDir, &written)
	case archiveTar:
		file, err := os.Open(archivePath)
		if err != nil {
			return &parse.PathError{Op: "open", Path: archivePath, Err: err}
		}
		defer file.Close()
		return extractTarReader(tar.NewReader(file), archivePath, destDir, &written)
	case archiveTarGzip:
		file, err := os.Open(archivePath)
		if err != nil {
			return &parse.PathError{Op: "open", Path: archivePath, Err: err}
		}
		defer file.Close()
		gz, err := gzip.NewReader(file)
		if err != nil {
			return newArchiveInputError(archivePath, "", err)
		}
		defer gz.Close()
		return extractTarReader(tar.NewReader(gz), archivePath, destDir, &written)
	case archiveGzip:
		return extractGzipArchive(archivePath, destDir, &written)
	default:
		return fmt.Errorf("%w: %s", errUnsupportedArchive, archivePath)
	}
}

func extractZipArchive(archivePath, destDir string, written *int64) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return newArchiveInputError(archivePath, "", err)
	}
	defer reader.Close()

	for _, entry := range reader.File {
		target, isRoot, err := safeArchiveTarget(destDir, entry.Name)
		if err != nil {
			return err
		}
		mode := entry.FileInfo().Mode()
		if isRoot && !entry.FileInfo().IsDir() {
			return newArchiveInputError(archivePath, entry.Name, errors.New("root archive entry must be a directory"))
		}
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("create archive directory %s: %w", target, err)
			}
			continue
		}
		if !mode.IsRegular() {
			return newArchiveInputError(archivePath, entry.Name, errors.New("unsupported non-regular entry"))
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create archive parent %s: %w", filepath.Dir(target), err)
		}
		src, err := entry.Open()
		if err != nil {
			return newArchiveInputError(archivePath, entry.Name, err)
		}
		if err := writeExtractedFile(target, mode.Perm(), src, written); err != nil {
			_ = src.Close()
			return err
		}
		if err := src.Close(); err != nil {
			return newArchiveInputError(archivePath, entry.Name, err)
		}
	}
	return nil
}

func extractTarReader(reader *tar.Reader, archivePath, destDir string, written *int64) error {
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return newArchiveInputError(archivePath, "", err)
		}
		target, isRoot, err := safeArchiveTarget(destDir, header.Name)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("create archive directory %s: %w", target, err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if isRoot {
				return newArchiveInputError(archivePath, header.Name, errors.New("root archive entry must be a directory"))
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("create archive parent %s: %w", filepath.Dir(target), err)
			}
			if err := writeExtractedFile(target, os.FileMode(header.Mode).Perm(), reader, written); err != nil {
				return err
			}
		case tar.TypeXGlobalHeader, tar.TypeXHeader:
			continue
		default:
			return newArchiveInputError(archivePath, header.Name, errors.New("unsupported non-regular entry"))
		}
	}
}

func extractGzipArchive(archivePath, destDir string, written *int64) error {
	return extractGzipArchiveWithLimits(archivePath, destDir, written, maxArchiveExtractedBytes, maxArchiveFileBytes)
}

func extractGzipArchiveWithLimits(archivePath, destDir string, written *int64, maxTotal, maxFile int64) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return &parse.PathError{Op: "open", Path: archivePath, Err: err}
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return newArchiveInputError(archivePath, "", err)
	}
	defer gz.Close()

	buffered := bufio.NewReader(gz)
	if block, err := buffered.Peek(512); err == nil && looksLikeTarHeader(block) {
		return extractTarReader(tar.NewReader(buffered), archivePath, destDir, written)
	}

	name := strings.TrimSuffix(filepath.Base(archivePath), filepath.Ext(archivePath))
	if name == "" || name == "." {
		name = "decompressed"
	}
	target, _, err := safeArchiveTarget(destDir, name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create archive parent %s: %w", filepath.Dir(target), err)
	}
	if err := writeExtractedFileWithLimits(target, 0o600, buffered, written, maxTotal, maxFile); err != nil {
		// Pass through size-bound errors unwrapped so
		// mapInputPreparationError can route them to exitSizeBound.
		// Both *parse.SizeError (per-file ceiling) and
		// *archiveExtractedSizeError (total-extracted ceiling) are
		// emitted by writeExtractedFileWithLimits and must reach the
		// caller as their original type.
		var sizeErr *parse.SizeError
		if errors.As(err, &sizeErr) {
			return err
		}
		var extractedSizeErr *archiveExtractedSizeError
		if errors.As(err, &extractedSizeErr) {
			return err
		}
		return newArchiveInputError(archivePath, name, err)
	}
	return nil
}

func looksLikeTarHeader(block []byte) bool {
	if len(block) < 265 {
		return false
	}
	return bytes.HasPrefix(block[257:], []byte("ustar"))
}

func safeArchiveTarget(destDir, entryName string) (string, bool, error) {
	entryName = strings.ReplaceAll(entryName, "\\", "/")
	if entryName == "" {
		return "", false, &unsafeArchivePathError{entry: entryName}
	}
	clean := filepath.Clean(filepath.FromSlash(entryName))
	if clean == "." {
		return destDir, true, nil
	}
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", false, &unsafeArchivePathError{entry: entryName}
	}
	target := filepath.Join(destDir, clean)
	rel, err := filepath.Rel(destDir, target)
	if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false, &unsafeArchivePathError{entry: entryName}
	}
	return target, false, nil
}

func writeExtractedFile(target string, mode os.FileMode, src io.Reader, written *int64) error {
	return writeExtractedFileWithLimits(target, mode, src, written, maxArchiveExtractedBytes, maxArchiveFileBytes)
}

func writeExtractedFileWithLimits(target string, mode os.FileMode, src io.Reader, written *int64, maxTotal, maxFile int64) error {
	perm := mode.Perm()
	if perm == 0 {
		perm = 0o600
	}
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return fmt.Errorf("create extracted file %s: %w", target, err)
	}
	defer file.Close()

	remaining := maxTotal - *written
	if remaining < 0 {
		remaining = 0
	}
	limit := maxFile
	if remaining < limit {
		limit = remaining
	}
	n, err := io.Copy(file, io.LimitReader(src, limit+1))
	*written += n
	if n > maxFile {
		return &parse.SizeError{
			Kind:  parse.SizeErrorFile,
			Path:  target,
			Bytes: n,
			Limit: maxFile,
		}
	}
	if *written > maxTotal {
		return &archiveExtractedSizeError{
			Path:  target,
			Bytes: *written,
			Limit: maxTotal,
		}
	}
	if err != nil {
		return fmt.Errorf("write extracted file %s: %w", target, err)
	}
	return nil
}

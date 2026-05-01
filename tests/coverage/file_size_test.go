package coverage_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/matias-sanchez/My-gather/tests/goldens"
)

const governedSourceLineLimit = 1000

var governedSourceExtensions = map[string]bool{
	".css":  true,
	".go":   true,
	".js":   true,
	".jsx":  true,
	".sh":   true,
	".tmpl": true,
	".ts":   true,
	".tsx":  true,
}

// TestGovernedSourceFileLineLimit enforces Constitution Principle XV:
// maintained first-party source files must stay below the god-file line limit.
func TestGovernedSourceFileLineLimit(t *testing.T) {
	root := goldens.RepoRoot(t)
	paths := governedSourceFiles(t, root)

	var offenders []string
	for _, path := range paths {
		lines := countLines(t, filepath.Join(root, filepath.FromSlash(path)))
		if lines > governedSourceLineLimit {
			offenders = append(offenders, path+" has "+strconv.Itoa(lines)+" lines")
		}
	}

	if len(offenders) == 0 {
		return
	}
	sort.Strings(offenders)
	t.Fatalf(
		"governed source files must not exceed %d lines:\n%s",
		governedSourceLineLimit,
		strings.Join(offenders, "\n"),
	)
}

func governedSourceFiles(t *testing.T, root string) []string {
	t.Helper()
	if paths, ok := gitTrackedSourceFiles(root); ok {
		return paths
	}
	return walkedSourceFiles(t, root)
}

func gitTrackedSourceFiles(root string) ([]string, bool) {
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, true
	}

	var paths []string
	for _, path := range lines {
		if isGovernedSourcePath(path) {
			paths = append(paths, path)
		}
	}
	return paths, true
}

func walkedSourceFiles(t *testing.T, root string) []string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "node_modules":
				return filepath.SkipDir
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if strings.HasPrefix(rel, "_references/") || strings.HasPrefix(rel, "testdata/") {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if isGovernedSourcePath(rel) {
			paths = append(paths, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk source files: %v", err)
	}
	sort.Strings(paths)
	return paths
}

func isGovernedSourcePath(path string) bool {
	if strings.HasPrefix(path, "_references/") || strings.HasPrefix(path, "testdata/") {
		return false
	}
	if path == "render/assets/chart.min.js" || path == "render/assets/chart.min.css" {
		return false
	}
	return governedSourceExtensions[filepath.Ext(path)]
}

func countLines(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if len(data) == 0 {
		return 0
	}
	lines := bytes.Count(data, []byte{'\n'})
	if !bytes.HasSuffix(data, []byte{'\n'}) {
		lines++
	}
	return lines
}

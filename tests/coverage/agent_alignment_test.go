package coverage_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/matias-sanchez/My-gather/tests/goldens"
)

func TestAgentAlignmentActiveFeaturePointers(t *testing.T) {
	root := goldens.RepoRoot(t)
	featureSlug := activeFeatureSlugFromJSON(t, filepath.Join(root, ".specify", "feature.json"))

	for _, path := range []string{"AGENTS.md", "CLAUDE.md"} {
		path := path
		t.Run(path, func(t *testing.T) {
			got := activeFeatureFromContext(t, filepath.Join(root, path))
			if got != featureSlug {
				t.Fatalf("%s active feature = %q, want %q from .specify/feature.json",
					path, got, featureSlug)
			}
		})
	}
}

func TestAgentAlignmentSkillMirrors(t *testing.T) {
	root := goldens.RepoRoot(t)
	claudeSkills := skillDirs(t, filepath.Join(root, ".claude", "skills"))
	codexSkills := skillDirs(t, filepath.Join(root, ".agents", "skills"))

	var missing []string
	for name := range claudeSkills {
		if !codexSkills[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("Claude skills missing Codex mirrors under .agents/skills: %s",
			strings.Join(missing, ", "))
	}
}

func TestAgentAlignmentSpecKitPlanTargets(t *testing.T) {
	root := goldens.RepoRoot(t)
	assertFileContains(t,
		filepath.Join(root, ".agents", "skills", "speckit-plan", "SKILL.md"),
		"AGENTS.md",
	)
	assertFileContains(t,
		filepath.Join(root, ".claude", "skills", "speckit-plan", "SKILL.md"),
		"CLAUDE.md",
	)
}

func activeFeatureSlugFromJSON(t *testing.T, path string) string {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var data struct {
		FeatureDirectory string `json:"feature_directory"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if data.FeatureDirectory == "" {
		t.Fatalf("%s has no feature_directory", path)
	}
	return filepath.Base(data.FeatureDirectory)
}

func activeFeatureFromContext(t *testing.T, path string) string {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	re := regexp.MustCompile(`(?m)^Active feature: \*\*([^*]+)\*\*`)
	match := re.FindSubmatch(payload)
	if len(match) != 2 {
		t.Fatalf("%s does not contain an active feature line", path)
	}
	return string(match[1])
}

func skillDirs(t *testing.T, root string) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}
	out := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, entry.Name(), "SKILL.md")); err != nil {
			t.Fatalf("%s is missing SKILL.md: %v", filepath.Join(root, entry.Name()), err)
		}
		out[entry.Name()] = true
	}
	return out
}

func assertFileContains(t *testing.T, path string, needle string) {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(payload), needle) {
		t.Fatalf("%s does not contain %q", path, needle)
	}
}

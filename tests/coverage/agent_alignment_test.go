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
	startupSkills := codexStartupSkillDirs(t)

	var missing []string
	for name := range claudeSkills {
		if !codexSkills[name] && !startupSkills[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("Claude skills missing Codex exposure under .agents/skills or ~/.codex/skills: %s",
			strings.Join(missing, ", "))
	}
}

func TestAgentAlignmentNoDuplicateCodexSkillSlugs(t *testing.T) {
	root := goldens.RepoRoot(t)
	repoSkills := skillDirs(t, filepath.Join(root, ".agents", "skills"))
	startupSkills := codexStartupSkillDirs(t)

	var duplicates []string
	for name := range repoSkills {
		if startupSkills[name] {
			duplicates = append(duplicates, name)
		}
	}
	sort.Strings(duplicates)
	if len(duplicates) > 0 {
		t.Fatalf("Codex skill slug(s) exist in both .agents/skills and ~/.codex/skills: %s",
			strings.Join(duplicates, ", "))
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

func TestAgentAlignmentCodexStartupSkillCoverage(t *testing.T) {
	startupSkills := codexStartupSkillDirs(t)
	if startupSkills == nil {
		t.Skip("~/.codex/skills is not configured on this machine")
	}

	for _, name := range requiredMyGatherSkills() {
		if !startupSkills[name] {
			t.Fatalf("Codex startup skill %q is missing from ~/.codex/skills", name)
		}
	}
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
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, entry.Name(), "SKILL.md")); err != nil {
			t.Fatalf("%s is missing SKILL.md: %v", filepath.Join(root, entry.Name()), err)
		}
		out[entry.Name()] = true
	}
	return out
}

func codexStartupSkillDirs(t *testing.T) map[string]bool {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("resolve home directory: %v", err)
	}
	codexSkills := filepath.Join(home, ".codex", "skills")
	if _, err := os.Stat(codexSkills); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("stat %s: %v", codexSkills, err)
	}
	entries, err := os.ReadDir(codexSkills)
	if err != nil {
		t.Fatalf("read %s: %v", codexSkills, err)
	}
	out := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if _, err := os.Stat(filepath.Join(codexSkills, entry.Name(), "SKILL.md")); err != nil {
			continue
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

func requiredMyGatherSkills() []string {
	return []string{
		"pr-review-fix-my-gather",
		"pr-review-loop-my-gather",
		"pr-review-trigger-my-gather",
	}
}

package render

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

//go:embed templates/*.tmpl
var embeddedTemplates embed.FS

//go:embed assets/chart.min.js
var embeddedChartJS string

//go:embed assets/chart.min.css
var embeddedChartCSS string

//go:embed assets/app.js
var embeddedAppJS string

//go:embed assets/app.css
var embeddedAppCSS string

//go:embed assets/mysql-defaults.json
var embeddedMySQLDefaultsJSON []byte

//go:embed assets/mysqladmin-categories.json
var embeddedMysqladminCategoriesJSON []byte

//go:embed assets/logo.png
var embeddedLogoPNG []byte

type mysqladminCategoryDef struct {
	Key             string   `json:"key"`
	Label           string   `json:"label"`
	Description     string   `json:"description"`
	Matchers        []string `json:"matchers"`
	ExcludeMatchers []string `json:"exclude_matchers"`
	Members         []string `json:"members"`
}

var (
	mysqladminCategories     []mysqladminCategoryDef
	mysqladminCategoriesOnce sync.Once
)

func loadMysqladminCategories() []mysqladminCategoryDef {
	mysqladminCategoriesOnce.Do(func() {
		var v struct {
			Categories []mysqladminCategoryDef `json:"categories"`
		}
		// Embedded at build time — a parse error is a programmer
		// error, not a user-runtime condition. Fail loudly instead
		// of silently producing an uncategorised chart view.
		if err := json.Unmarshal(embeddedMysqladminCategoriesJSON, &v); err != nil {
			panic(fmt.Sprintf("render/assets/mysqladmin-categories.json: malformed embedded JSON: %v", err))
		}
		mysqladminCategories = v.Categories
	})
	return mysqladminCategories
}

// classifyMysqladminCategory returns the slugs of every category that
// claims `name`. Matching is case-insensitive on prefixes. Explicit
// `members` take precedence; `exclude_matchers` carve out a subset of
// a broader matcher (e.g., buffer-pool excludes the read/write
// subviews so they live in InnoDB Reads / Writes instead).
func classifyMysqladminCategory(cats []mysqladminCategoryDef, name string) []string {
	lower := strings.ToLower(name)
	var hits []string
	for _, c := range cats {
		// Members list — highest priority.
		matched := false
		for _, m := range c.Members {
			if strings.EqualFold(m, name) {
				matched = true
				break
			}
		}
		if !matched {
			for _, mp := range c.Matchers {
				if strings.HasPrefix(lower, strings.ToLower(mp)) {
					matched = true
					break
				}
			}
			if matched {
				for _, ex := range c.ExcludeMatchers {
					if strings.HasPrefix(lower, strings.ToLower(ex)) {
						matched = false
						break
					}
				}
			}
		}
		if matched {
			hits = append(hits, c.Key)
		}
	}
	return hits
}

// mysqlDefaults holds the versioned defaults table parsed from the
// embedded JSON. The outer map is variable-name → (major-version →
// documented default). Major versions are the short form "5.7",
// "8.0", "8.4" matching the "versions" list in the JSON.
//
// mysqlDefaultsVersions is the canonical chronological list used for
// fallback resolution by resolveVersion: when a capture comes from a
// version we don't have a column for, resolveVersion selects the
// latest listed version that is still ≤ captured (e.g. captured "8.1"
// → "8.0" when only "5.7"/"8.0"/"8.4" are listed).
var (
	mysqlDefaults         map[string]map[string]string
	mysqlDefaultsVersions []string
	mysqlDefaultsOnce     sync.Once
)

func loadMySQLDefaults() (map[string]map[string]string, []string) {
	mysqlDefaultsOnce.Do(func() {
		var v struct {
			Versions  []string                     `json:"versions"`
			Variables map[string]map[string]string `json:"variables"`
		}
		// Embedded at build time — a parse error is a programmer
		// error. Fail loudly so the "non-default" badges can't
		// silently lose their reference map.
		if err := json.Unmarshal(embeddedMySQLDefaultsJSON, &v); err != nil {
			panic(fmt.Sprintf("render/assets/mysql-defaults.json: malformed embedded JSON: %v", err))
		}
		if len(v.Versions) == 0 || len(v.Variables) == 0 {
			panic("render/assets/mysql-defaults.json: missing versions or variables")
		}
		mysqlDefaults = v.Variables
		mysqlDefaultsVersions = v.Versions
	})
	return mysqlDefaults, mysqlDefaultsVersions
}

// majorVersion extracts the "5.7" / "8.0" / "8.4" short form from a
// raw MySQL version string (e.g. "8.0.32-24", "5.7.44-48-log",
// "8.4.2"). Returns "" if the input doesn't start with at least
// "N.N".
func majorVersion(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) < 3 {
		return ""
	}
	// First three characters must be digit.dot.digit — anything else
	// (e.g. "unknown", "Ver 8") is unresolvable so we return empty and
	// the caller (classifyVariable) falls through to the "unknown"
	// result.
	if !(raw[0] >= '0' && raw[0] <= '9') || raw[1] != '.' || !(raw[2] >= '0' && raw[2] <= '9') {
		return ""
	}
	return raw[:3]
}

// resolveVersion picks which column of the defaults table to consult
// for a captured version string. Returns the chosen major-version key
// or "" if no match is possible. Fallback order:
//  1. Exact match on captured major version (e.g. "8.0" → "8.0").
//  2. Latest supported version ≤ captured, walking
//     mysqlDefaultsVersions in reverse so newer-but-≤-captured wins
//     (e.g. captured "8.1" falls back to "8.0" when 8.1 isn't listed).
//  3. "" when the captured version is older than every column.
func resolveVersion(captured string, supported []string) string {
	mv := majorVersion(captured)
	if mv == "" {
		return ""
	}
	// 1. Exact.
	for _, v := range supported {
		if v == mv {
			return v
		}
	}
	// 2. Latest supported ≤ captured. Order of `supported` is
	//    irrelevant — we scan all entries and keep the highest one
	//    that is still ≤ the captured version. Comparison is
	//    lexicographic on the 3-char form ("8.0" vs "8.4") which is
	//    correct for the current range (all short forms are "N.N"
	//    with single-digit components).
	best := ""
	for _, v := range supported {
		if v <= mv && v > best {
			best = v
		}
	}
	return best
}

// classifyVariable compares a captured variable value against the
// documented compiled-in default for the captured MySQL version.
// `version` is the raw `version` variable as captured by pt-stalk
// (e.g. "8.0.32-24"). Returns:
//   - "default"  — value matches the documented default
//   - "modified" — value differs from the documented default
//   - "unknown"  — no default is documented for this (variable,
//     version) pair
//
// Matching is tolerant: whitespace-trimmed, case-insensitive, and
// comma-separated values (e.g. sql_mode, tls_version) compared as
// sets so member order does not flag a default as modified.
func classifyVariable(defaults map[string]map[string]string, supported []string, version, name, observed string) string {
	perVersion, ok := defaults[name]
	if !ok {
		return "unknown"
	}
	col := resolveVersion(version, supported)
	if col == "" {
		return "unknown"
	}
	def, ok := perVersion[col]
	if !ok {
		// Variable is documented for some versions but not this one
		// (e.g. innodb_log_file_size has no 8.4 entry because
		// innodb_redo_log_capacity replaced it). Treat as unknown so
		// the UI does not flag the capture as modified against a
		// default that doesn't exist.
		return "unknown"
	}
	if normalisedEqual(def, observed) {
		return "default"
	}
	if commaSetsEqual(def, observed) {
		return "default"
	}
	return "modified"
}

func normalisedEqual(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func commaSetsEqual(a, b string) bool {
	if !strings.Contains(a, ",") && !strings.Contains(b, ",") {
		return false
	}
	split := func(s string) []string {
		parts := strings.Split(s, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			out = append(out, strings.ToLower(p))
		}
		sort.Strings(out)
		return out
	}
	xs, ys := split(a), split(b)
	if len(xs) != len(ys) {
		return false
	}
	for i := range xs {
		if xs[i] != ys[i] {
			return false
		}
	}
	return true
}

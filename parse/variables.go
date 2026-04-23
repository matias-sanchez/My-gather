package parse

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/matias-sanchez/My-gather/model"
)

// parseVariables reads the tab-separated output of `SHOW GLOBAL
// VARIABLES` as emitted by pt-stalk and returns a sorted, deduplicated
// slice of VariableEntry (spec FR-012).
//
// Format:
//
//	SHOW GLOBAL VARIABLES
//
//	Variable_name   Value
//	activate_all_roles_on_login   OFF
//	admin_address
//	...
//
// Variables with the same name appearing multiple times are
// deduplicated (first occurrence wins per FR-012) and a
// SeverityWarning diagnostic records each drop.
func parseVariables(r io.Reader, sourcePath string) (*model.VariablesData, []model.Diagnostic) {
	scanner := newLineScanner(r)

	var diagnostics []model.Diagnostic
	addDiag := func(line int, sev model.Severity, msg string) {
		diagnostics = append(diagnostics, model.Diagnostic{
			SourceFile: sourcePath,
			Location:   fmt.Sprintf("line %d", line),
			Severity:   sev,
			Message:    msg,
		})
	}

	// The pt-stalk -variables file may contain multiple sections:
	//   1. SHOW GLOBAL VARIABLES   (tab-separated, 2 columns)
	//   2. performance_schema.session_variables (3 columns, per-thread)
	// We want only section (1) per spec FR-012. We consume lines while
	// under the "Variable_name<TAB>Value" header and stop (without
	// error) as soon as a 3+-column header appears or a second SHOW
	// command starts.
	seen := map[string]struct{}{}
	values := map[string]string{}
	lineNum := 0
	inGlobalSection := false
	for scanner.Scan() {
		lineNum++
		raw := scanner.Text()
		line := strings.TrimRight(raw, "\r")

		if line == "" {
			continue
		}
		// Detect a new section header or SHOW command.
		if strings.HasPrefix(line, "SHOW ") {
			if inGlobalSection {
				break // end of global section; the next section is out of scope
			}
			continue
		}
		if strings.HasPrefix(line, "Variable_name\t") {
			inGlobalSection = true
			continue
		}
		// A 3+-column header like "THREAD_ID\tVARIABLE_NAME\tVARIABLE_VALUE"
		// marks the start of the per-session section; stop.
		if inGlobalSection && strings.HasPrefix(line, "THREAD_ID\t") {
			break
		}
		if !inGlobalSection {
			continue
		}

		// Data row — exactly two tab-separated columns.
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			// Not a 2-column row; likely leaked into a different section.
			// Stop gracefully rather than mis-parsing.
			break
		}
		name := strings.TrimSpace(parts[0])
		if name == "" {
			continue
		}
		value := strings.TrimRight(parts[1], " \t")
		if _, exists := seen[name]; exists {
			addDiag(lineNum, model.SeverityWarning,
				fmt.Sprintf("variables: duplicate %q (keeping first global value per FR-012)", name))
			continue
		}
		seen[name] = struct{}{}
		values[name] = value
	}
	if err := scanner.Err(); err != nil {
		diagnostics = append(diagnostics, model.Diagnostic{
			SourceFile: sourcePath,
			Severity:   model.SeverityError,
			Message:    fmt.Sprintf("variables read: %v", err),
		})
	}

	if len(values) == 0 {
		return nil, diagnostics
	}

	names := make([]string, 0, len(values))
	for n := range values {
		names = append(names, n)
	}
	sort.Strings(names)
	entries := make([]model.VariableEntry, 0, len(names))
	for _, n := range names {
		entries = append(entries, model.VariableEntry{Name: n, Value: values[n]})
	}
	return &model.VariablesData{Entries: entries}, diagnostics
}

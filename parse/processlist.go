package parse

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/matias-sanchez/My-gather/model"
)

// parseProcesslist reads pt-stalk -processlist output (repeated
// SHOW FULL PROCESSLIST \G captures) and returns one
// ThreadStateSample per sample, each with a per-state count bucket
// (spec FR-017).
//
// Each snapshot begins with a TS line:
//
//	TS <unix.time> YYYY-MM-DD HH:MM:SS
//
// followed by vertical-format rows:
//
//	*************************** 1. row ***************************
//	         Id: 6
//	       User: …
//	        ...
//	      State: <label>
//	        ...
//
// Threads with an empty State or an unknown state bucket into "Other".
func parseProcesslist(r io.Reader, sourcePath string) (*model.ProcesslistData, []model.Diagnostic) {
	scanner := newLineScanner(r)

	var diagnostics []model.Diagnostic

	type rowBuild struct {
		user             string
		host             string
		command          string
		db               string
		state            string
		timeSeconds      float64
		timeMS           float64
		rowsSent         float64
		rowsExamined     float64
		info             string
		haveTime         bool
		haveTimeMS       bool
		haveRowsSent     bool
		haveRowsExamined bool
		haveInfo         bool
		hasCoreField     bool
	}
	type sampleBuild struct {
		t                 time.Time
		state             map[string]int
		user              map[string]int
		host              map[string]int
		command           map[string]int
		db                map[string]int
		row               rowBuild
		total             int
		active            int
		sleeping          int
		maxTimeMS         float64
		hasTimeMetric     bool
		maxRowsExamined   float64
		hasRowsExamined   bool
		maxRowsSent       float64
		hasRowsSent       bool
		rowsWithQueryText int
		hasQueryText      bool
	}
	var samples []sampleBuild
	var current *sampleBuild
	statesSet := map[string]struct{}{}
	usersSet := map[string]struct{}{}
	hostsSet := map[string]struct{}{}
	commandsSet := map[string]struct{}{}
	dbsSet := map[string]struct{}{}
	observedQueriesByFingerprint := map[string]model.ObservedProcesslistQuery{}

	// flushRow records a completed vertical-format row's dimensions
	// into the current sample. Called on row-separator lines and on
	// sample/file boundaries.
	flushRow := func() {
		if current == nil {
			return
		}
		if !current.row.hasCoreField {
			// Row-separator fired with none of the tracked fields
			// populated — nothing to attribute. This is expected for
			// the first `*** 1. row ***` marker in every sample (it
			// delimits the start of the first row, not the end of a
			// prior row), so skip quietly and do not emit a
			// diagnostic.
			current.row = rowBuild{}
			return
		}
		label := current.row.state
		if label == "" || label == "NULL" {
			label = "Other"
		}
		current.state[label]++
		statesSet[label] = struct{}{}

		user := current.row.user
		if user == "" {
			user = "Other"
		}
		current.user[user]++
		usersSet[user] = struct{}{}

		host := stripHostPort(current.row.host)
		if host == "" {
			host = "Other"
		}
		current.host[host]++
		hostsSet[host] = struct{}{}

		cmd := current.row.command
		if cmd == "" {
			cmd = "Other"
		}
		current.command[cmd]++
		commandsSet[cmd] = struct{}{}

		db := current.row.db
		if db == "" || db == "NULL" {
			db = "Other"
		}
		current.db[db]++
		dbsSet[db] = struct{}{}

		current.total++
		if current.row.command == "Sleep" {
			current.sleeping++
		} else {
			current.active++
		}
		ageMS := 0.0
		if current.row.haveTimeMS {
			ageMS = current.row.timeMS
		} else if current.row.haveTime {
			ageMS = current.row.timeSeconds * 1000
		}
		if current.row.haveTimeMS || current.row.haveTime {
			current.hasTimeMetric = true
		}
		if ageMS > current.maxTimeMS {
			current.maxTimeMS = ageMS
		}
		if current.row.haveRowsExamined {
			current.hasRowsExamined = true
		}
		if current.row.haveRowsExamined && current.row.rowsExamined > current.maxRowsExamined {
			current.maxRowsExamined = current.row.rowsExamined
		}
		if current.row.haveRowsSent {
			current.hasRowsSent = true
		}
		if current.row.haveRowsSent && current.row.rowsSent > current.maxRowsSent {
			current.maxRowsSent = current.row.rowsSent
		}
		if current.row.haveInfo {
			current.hasQueryText = true
		}
		if current.row.haveInfo && hasProcesslistQueryText(current.row.info) {
			current.rowsWithQueryText++
		}
		if q, ok := model.NewObservedProcesslistQuery(
			current.t,
			current.row.user,
			current.row.db,
			current.row.command,
			current.row.state,
			current.row.info,
			ageMS,
			current.row.haveTimeMS || current.row.haveTime,
			current.row.rowsExamined,
			current.row.haveRowsExamined,
			current.row.rowsSent,
			current.row.haveRowsSent,
		); ok {
			if current, exists := observedQueriesByFingerprint[q.Fingerprint]; exists {
				merged := model.MergeObservedProcesslistQueries(
					[]model.ObservedProcesslistQuery{current},
					[]model.ObservedProcesslistQuery{q},
				)
				if len(merged) == 1 {
					observedQueriesByFingerprint[q.Fingerprint] = merged[0]
				}
			} else {
				observedQueriesByFingerprint[q.Fingerprint] = q
			}
		}

		current.row = rowBuild{}
	}

	startNewSample := func(t time.Time) {
		flushRow()
		if current != nil {
			samples = append(samples, *current)
		}
		current = &sampleBuild{
			t:       t,
			state:   map[string]int{},
			user:    map[string]int{},
			host:    map[string]int{},
			command: map[string]int{},
			db:      map[string]int{},
		}
	}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r\n")
		if m := reTimestampLine.FindStringSubmatch(line); m != nil {
			epoch, _ := strconv.ParseFloat(m[1], 64)
			t := time.Unix(int64(math.Floor(epoch)), 0).UTC()
			startNewSample(t)
			continue
		}
		if strings.Contains(line, "*** ") && strings.Contains(line, ". row ***") {
			// Row-separator — flush what we've gathered for the prior row.
			flushRow()
			continue
		}
		// Vertical-format field. The field name is everything up to the
		// first ":" stripped of leading space.
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if current == nil {
			continue
		}
		switch key {
		case "State":
			current.row.state = val
			current.row.hasCoreField = true
		case "User":
			current.row.user = val
			current.row.hasCoreField = true
		case "Host":
			current.row.host = val
			current.row.hasCoreField = true
		case "Command":
			current.row.command = val
			current.row.hasCoreField = true
		case "db":
			current.row.db = val
			current.row.hasCoreField = true
		case "Time":
			if parsed, ok := parseProcesslistNonNegativeFloat(val); ok {
				current.row.timeSeconds = parsed
				current.row.haveTime = true
			}
		case "Time_ms":
			if parsed, ok := parseProcesslistNonNegativeFloat(val); ok {
				current.row.timeMS = parsed
				current.row.haveTimeMS = true
			}
		case "Rows_sent":
			if parsed, ok := parseProcesslistNonNegativeFloat(val); ok {
				current.row.rowsSent = parsed
				current.row.haveRowsSent = true
			}
		case "Rows_examined":
			if parsed, ok := parseProcesslistNonNegativeFloat(val); ok {
				current.row.rowsExamined = parsed
				current.row.haveRowsExamined = true
			}
		case "Info":
			current.row.info = val
			current.row.haveInfo = true
		}
	}
	if err := scanner.Err(); err != nil {
		diagnostics = append(diagnostics, model.Diagnostic{
			SourceFile: sourcePath,
			Severity:   model.SeverityError,
			Message:    fmt.Sprintf("processlist read: %v", err),
		})
	}
	if current != nil {
		flushRow()
		samples = append(samples, *current)
	}

	if len(samples) == 0 {
		return nil, diagnostics
	}

	// Canonical order: alphabetical, "Other" always last (when present).
	canonical := func(set map[string]struct{}) []string {
		var out []string
		hasOther := false
		for s := range set {
			if s == "Other" {
				hasOther = true
				continue
			}
			out = append(out, s)
		}
		sort.Strings(out)
		if hasOther {
			out = append(out, "Other")
		}
		return out
	}
	states := canonical(statesSet)
	users := canonical(usersSet)
	hosts := canonical(hostsSet)
	commands := canonical(commandsSet)
	dbs := canonical(dbsSet)

	// Samples sorted by timestamp ascending (defensive).
	sort.SliceStable(samples, func(i, j int) bool {
		return samples[i].t.Before(samples[j].t)
	})

	out := make([]model.ThreadStateSample, len(samples))
	for i, s := range samples {
		out[i] = model.ThreadStateSample{
			Timestamp:             s.t,
			StateCounts:           s.state,
			UserCounts:            s.user,
			HostCounts:            s.host,
			CommandCounts:         s.command,
			DbCounts:              s.db,
			TotalThreads:          s.total,
			ActiveThreads:         s.active,
			SleepingThreads:       s.sleeping,
			MaxTimeMS:             s.maxTimeMS,
			HasTimeMetric:         s.hasTimeMetric,
			MaxRowsExamined:       s.maxRowsExamined,
			HasRowsExaminedMetric: s.hasRowsExamined,
			MaxRowsSent:           s.maxRowsSent,
			HasRowsSentMetric:     s.hasRowsSent,
			RowsWithQueryText:     s.rowsWithQueryText,
			HasQueryTextMetric:    s.hasQueryText,
		}
	}

	observedQueries := make([]model.ObservedProcesslistQuery, 0, len(observedQueriesByFingerprint))
	for _, q := range observedQueriesByFingerprint {
		observedQueries = append(observedQueries, q)
	}

	return &model.ProcesslistData{
		ThreadStateSamples: out,
		States:             states,
		Users:              users,
		Hosts:              hosts,
		Commands:           commands,
		Dbs:                dbs,
		ObservedQueries:    model.MergeObservedProcesslistQueries(observedQueries),
	}, diagnostics
}

func parseProcesslistNonNegativeFloat(s string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}

func hasProcesslistQueryText(s string) bool {
	trimmed := strings.TrimSpace(s)
	return trimmed != "" && !strings.EqualFold(trimmed, "NULL")
}

// stripHostPort trims the ":port" suffix from a processlist Host value.
// Accepted inputs:
//
//	"10.0.0.1:53412"         → "10.0.0.1"          (IPv4 with port)
//	"localhost"              → "localhost"         (hostname, no port)
//	"localhost:3306"         → "localhost"         (hostname with port)
//	"[::1]:53412"            → "[::1]"             (bracketed IPv6 with port)
//	"[2001:db8::5]"          → "[2001:db8::5]"     (bracketed IPv6, no port)
//	"::1"                    → "::1"               (unbracketed IPv6, no port)
//	"2001:db8::5"            → "2001:db8::5"       (unbracketed IPv6, no port)
//
// Bracketed hosts are always preserved in bracketed form. Non-bracketed
// hosts are treated as having a port only when the colon-split form is
// unambiguously host:port — i.e. exactly one colon in the string. Two
// or more colons indicate an unbracketed IPv6 literal (MySQL cannot
// encode a port on unbracketed IPv6, so no truncation is needed).
func stripHostPort(h string) string {
	if h == "" {
		return ""
	}
	if strings.HasPrefix(h, "[") {
		if i := strings.Index(h, "]"); i >= 0 {
			return h[:i+1]
		}
		return h
	}
	// Multiple ':' → unbracketed IPv6; there is no port suffix to trim.
	if strings.Count(h, ":") != 1 {
		return h
	}
	return h[:strings.IndexByte(h, ':')]
}

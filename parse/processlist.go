package parse

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/matias-sanchez/My-gather/model"
)

// tsLine matches the sample-boundary marker pt-stalk writes at the top
// of each processlist snapshot:
//
//	TS 1776790303.009325313 2026-04-21 16:51:43
var tsLine = regexp.MustCompile(`^TS\s+(\d+(?:\.\d+)?)\s+(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2})`)

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
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 32*1024*1024)

	var diagnostics []model.Diagnostic

	type rowBuild struct {
		user    string
		host    string
		command string
		db      string
		state   string
	}
	type sampleBuild struct {
		t       time.Time
		state   map[string]int
		user    map[string]int
		host    map[string]int
		command map[string]int
		db      map[string]int
		row     rowBuild
	}
	var samples []sampleBuild
	var current *sampleBuild
	statesSet := map[string]struct{}{}
	usersSet := map[string]struct{}{}
	hostsSet := map[string]struct{}{}
	commandsSet := map[string]struct{}{}
	dbsSet := map[string]struct{}{}

	// flushRow records a completed vertical-format row's dimensions
	// into the current sample. Called on row-separator lines and on
	// sample/file boundaries.
	flushRow := func() {
		if current == nil {
			return
		}
		if current.row == (rowBuild{}) {
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
		if m := tsLine.FindStringSubmatch(line); m != nil {
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
		case "User":
			current.row.user = val
		case "Host":
			current.row.host = val
		case "Command":
			current.row.command = val
		case "db":
			current.row.db = val
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
			Timestamp:     s.t,
			StateCounts:   s.state,
			UserCounts:    s.user,
			HostCounts:    s.host,
			CommandCounts: s.command,
			DbCounts:      s.db,
		}
	}

	return &model.ProcesslistData{
		ThreadStateSamples: out,
		States:             states,
		Users:              users,
		Hosts:              hosts,
		Commands:           commands,
		Dbs:                dbs,
	}, diagnostics
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

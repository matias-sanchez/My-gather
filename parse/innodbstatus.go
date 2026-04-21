package parse

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/matias-sanchez/My-gather/model"
)

// parseInnodbStatus reads the text of `SHOW ENGINE INNODB STATUS` as
// captured by pt-stalk's -innodbstatus1 collector and extracts the
// four scalar views required by spec FR-014:
//
//   - SemaphoreCount: number of threads currently waiting on a
//     semaphore (counted from the SEMAPHORES section's "--Thread N
//     has waited …" lines).
//   - PendingReads / PendingWrites: values from the BUFFER POOL AND
//     MEMORY section's "Pending reads" / "Pending writes" lines.
//   - AHIActivity: hash table size + searches-per-second from the
//     INSERT BUFFER AND ADAPTIVE HASH INDEX section.
//   - HistoryListLength: TRANSACTIONS section "History list length".
//
// This is a targeted extractor rather than a full InnoDB status
// parser; we only read what the report renders.
func parseInnodbStatus(r io.Reader, sourcePath string) (*model.InnodbStatusData, []model.Diagnostic) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 32*1024*1024)

	var diagnostics []model.Diagnostic
	data := &model.InnodbStatusData{}

	var inSection string
	threadWaited := 0
	rePendingReads := regexp.MustCompile(`^Pending reads\s+(\d+)`)
	rePendingWritesSum := regexp.MustCompile(`LRU (\d+), flush list (\d+), single page (\d+)`)
	reHashRate := regexp.MustCompile(`([\d.]+)\s+hash searches/s,\s+([\d.]+)\s+non-hash searches/s`)
	reHashTblSize := regexp.MustCompile(`^Hash table size\s+(\d+)`)
	reHLL := regexp.MustCompile(`^History list length (\d+)`)

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r\n")
		trimmed := strings.TrimSpace(line)

		// Section markers are all-dash lines whose NEXT line names the section.
		// Walk by just looking for the known labels directly.
		switch trimmed {
		case "SEMAPHORES", "BACKGROUND THREAD", "FILE I/O",
			"INSERT BUFFER AND ADAPTIVE HASH INDEX", "LOG",
			"BUFFER POOL AND MEMORY", "ROW OPERATIONS",
			"TRANSACTIONS":
			inSection = trimmed
			continue
		}

		switch inSection {
		case "SEMAPHORES":
			if strings.HasPrefix(trimmed, "--Thread ") && strings.Contains(trimmed, " has waited ") {
				threadWaited++
			}
		case "TRANSACTIONS":
			if m := reHLL.FindStringSubmatch(trimmed); m != nil {
				v, _ := strconv.Atoi(m[1])
				data.HistoryListLength = v
			}
		case "BUFFER POOL AND MEMORY":
			if m := rePendingReads.FindStringSubmatch(trimmed); m != nil {
				v, _ := strconv.Atoi(m[1])
				data.PendingReads = v
			}
			if strings.HasPrefix(trimmed, "Pending writes:") {
				if m := rePendingWritesSum.FindStringSubmatch(trimmed); m != nil {
					lru, _ := strconv.Atoi(m[1])
					fl, _ := strconv.Atoi(m[2])
					sp, _ := strconv.Atoi(m[3])
					data.PendingWrites = lru + fl + sp
				}
			}
		case "INSERT BUFFER AND ADAPTIVE HASH INDEX":
			// The first Hash table size line gives the configured size;
			// overwrite on each so we end up with the last-seen value.
			if m := reHashTblSize.FindStringSubmatch(trimmed); m != nil {
				v, _ := strconv.Atoi(m[1])
				data.AHIActivity.HashTableSize = v
			}
			if m := reHashRate.FindStringSubmatch(trimmed); m != nil {
				h, _ := strconv.ParseFloat(m[1], 64)
				nh, _ := strconv.ParseFloat(m[2], 64)
				data.AHIActivity.SearchesPerSec = h
				data.AHIActivity.NonHashSearchesPerSec = nh
			}
		}
	}
	if err := scanner.Err(); err != nil {
		diagnostics = append(diagnostics, model.Diagnostic{
			SourceFile: sourcePath,
			Severity:   model.SeverityError,
			Message:    fmt.Sprintf("innodbstatus read: %v", err),
		})
	}

	data.SemaphoreCount = threadWaited
	return data, diagnostics
}

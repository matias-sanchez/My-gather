package parse

import (
	"bytes"

	"github.com/matias-sanchez/My-gather/model"
)

// DetectFormat classifies a peeked byte slice from a pt-stalk source
// file into one of the supported FormatVersion enum values, branching
// on the file's Suffix. Returns FormatUnknown when the signature does
// not match any supported variant.
//
// Callers typically pass the first 8–16 KB of the file as peeked;
// that is more than sufficient to observe the per-collector header
// block.
//
// The heuristics are intentionally conservative — we prefer
// FormatUnknown (which triggers an "unsupported pt-stalk version"
// banner per spec FR-024) over silently parsing a format we don't
// recognise. See research R2 for the design rationale.
func DetectFormat(peeked []byte, suffix model.Suffix) model.FormatVersion {
	switch suffix {
	case model.SuffixIostat:
		return detectIostat(peeked)
	case model.SuffixTop:
		return detectTop(peeked)
	case model.SuffixVariables:
		return detectVariables(peeked)
	case model.SuffixVmstat:
		return detectVmstat(peeked)
	case model.SuffixMeminfo:
		return detectMeminfo(peeked)
	case model.SuffixInnodbStatus:
		return detectInnodbStatus(peeked)
	case model.SuffixMysqladmin:
		return detectMysqladmin(peeked)
	case model.SuffixProcesslist:
		return detectProcesslist(peeked)
	default:
		return model.FormatUnknown
	}
}

// Per-collector detection heuristics.
//
// Implementations are provisional skeletons for v1: every collector
// returns FormatV1 when a minimal sanity check passes. Proper V1-vs-V2
// discrimination lands with each per-collector parser in the US2/US3/
// US4 phases (tasks T048, T049, T050, T059, T071, T072, T073).

func detectIostat(peeked []byte) model.FormatVersion {
	// iostat output includes a "Device" or "Device:" header column in
	// every supported release. Presence of the token is a near-zero
	// false-positive signal.
	if bytes.Contains(peeked, []byte("Device")) {
		return model.FormatV1
	}
	return model.FormatUnknown
}

func detectTop(peeked []byte) model.FormatVersion {
	if bytes.Contains(peeked, []byte("PID")) && bytes.Contains(peeked, []byte("COMMAND")) {
		return model.FormatV1
	}
	return model.FormatUnknown
}

func detectVariables(peeked []byte) model.FormatVersion {
	// -variables is a pipe-delimited `SHOW GLOBAL VARIABLES` table.
	// Presence of the "Variable_name" header is the signature.
	if bytes.Contains(peeked, []byte("Variable_name")) {
		return model.FormatV1
	}
	return model.FormatUnknown
}

func detectVmstat(peeked []byte) model.FormatVersion {
	// vmstat prints a "procs" / "memory" / "swap" banner and numeric
	// columns. "procs" is present in every supported release.
	if bytes.Contains(peeked, []byte("procs")) && bytes.Contains(peeked, []byte("memory")) {
		return model.FormatV1
	}
	return model.FormatUnknown
}

func detectMeminfo(peeked []byte) model.FormatVersion {
	// /proc/meminfo always starts with MemTotal; pt-stalk prefixes
	// each sample with a "TS <epoch>" marker. Requiring both avoids
	// misclassifying an accidentally-named file.
	if bytes.Contains(peeked, []byte("MemTotal:")) && bytes.Contains(peeked, []byte("TS ")) {
		return model.FormatV1
	}
	return model.FormatUnknown
}

func detectInnodbStatus(peeked []byte) model.FormatVersion {
	if bytes.Contains(peeked, []byte("INNODB MONITOR OUTPUT")) ||
		bytes.Contains(peeked, []byte("Status:")) {
		return model.FormatV1
	}
	return model.FormatUnknown
}

func detectMysqladmin(peeked []byte) model.FormatVersion {
	// Repeated SHOW GLOBAL STATUS tables produce the same
	// "Variable_name" header as -variables but separated by the ruled
	// `+-...-+` lines. We look for both markers.
	if bytes.Contains(peeked, []byte("Variable_name")) && bytes.Contains(peeked, []byte("+-")) {
		return model.FormatV1
	}
	return model.FormatUnknown
}

func detectProcesslist(peeked []byte) model.FormatVersion {
	if bytes.Contains(peeked, []byte("Command")) && bytes.Contains(peeked, []byte("State")) {
		return model.FormatV1
	}
	return model.FormatUnknown
}

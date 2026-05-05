package model

import "time"

// OSSection is the render-ready view of a Collection's OS Usage
// content (iostat, top, vmstat). Each of the three subview fields is
// a pointer: nil when the corresponding source was absent or failed
// to parse, non-nil otherwise. The Missing slice carries the file
// names whose subview should render the "data not available" banner,
// sorted alphabetically for determinism.
type OSSection struct {
	Iostat      *IostatData
	Top         *TopData
	Vmstat      *VmstatData
	Meminfo     *MeminfoData
	NetSockets  *NetstatSocketsData
	NetCounters *NetstatCountersData
	Missing     []string
}

// VariablesSection is the render-ready view of the Variables content,
// one block per Snapshot (spec FR-018). PerSnapshot is ordered by the
// snapshot's timestamp ascending.
type VariablesSection struct {
	PerSnapshot []SnapshotVariables
}

// SnapshotVariables pairs a snapshot prefix with its parsed
// VariablesData (nil when the -variables file was absent from that
// snapshot).
type SnapshotVariables struct {
	SnapshotPrefix string
	Timestamp      time.Time
	Data           *VariablesData
}

// DBSection is the render-ready view of the Database Usage content.
// InnoDB scalars render per-snapshot (multi-snapshot side-by-side per
// FR-018); Mysqladmin and Processlist are merged across snapshots
// with boundary markers.
type DBSection struct {
	InnoDBPerSnapshot []SnapshotInnoDB
	Mysqladmin        *MysqladminData
	Processlist       *ProcesslistData
	Missing           []string
}

// SnapshotInnoDB pairs a snapshot prefix with its parsed
// InnodbStatusData (nil when the -innodbstatus1 file was absent from
// that snapshot).
type SnapshotInnoDB struct {
	SnapshotPrefix string
	Timestamp      time.Time
	Data           *InnodbStatusData
}

// Report is the top-level render envelope built by render.Render from
// a Collection plus RenderOptions. It is intentionally not returned
// to callers of render.Render — the template pipeline consumes it and
// then it's discarded.
//
// GeneratedAt is derived from input data or an explicit RenderOptions
// override so rendered HTML remains deterministic (Constitution
// Principle IV, spec FR-006).
type Report struct {
	Title       string
	Version     string    // tool semver, e.g., "v0.1.0"
	GitCommit   string    // short git SHA; "" when not injected
	BuiltAt     string    // build timestamp, from -ldflags
	GeneratedAt time.Time // filled per-render from input data or explicit options

	// Collection is the input, retained by pointer. Never mutated by
	// render.
	Collection *Collection

	// Section views, each built from Collection's snapshots.
	EnvironmentSection *EnvironmentSection
	OSSection          *OSSection
	VariablesSection   *VariablesSection
	DBSection          *DBSection

	// Navigation is a flat, ordered list of NavEntry items. Render-
	// side code consumes it directly; templates iterate the slice in
	// order without further sorting. See spec FR-031.
	Navigation []NavEntry

	// ReportID is a stable-per-report identifier (12 hex chars) used
	// as the prefix for localStorage keys in the collapse/expand
	// feature (spec FR-032). Computed as the first 12 hex chars of
	// SHA-256 over a canonicalisation of Collection that excludes
	// RootPath and GeneratedAt. Deterministic.
	ReportID string
}

// NavEntry is one row in the report's navigation index.
type NavEntry struct {
	// ID matches the anchor id on the rendered section / subview.
	// Generated deterministically (ordinal counters seeded by
	// section iteration order); stable across re-renders of the same
	// input.
	ID string

	// Title is the human-readable label shown in the nav rail.
	Title string

	// Level: 1 = top-level section, 2 = subview inside a section.
	Level int

	// ParentID references the enclosing Level-1 entry's ID; "" when
	// Level == 1.
	ParentID string
}

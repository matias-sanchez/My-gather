package render

import (
	"sort"

	"github.com/matias-sanchez/My-gather/model"
)

func collectionTitle(c *model.Collection) string {
	if c.Hostname != "" {
		return c.Hostname
	}
	if len(c.Snapshots) > 0 {
		return c.Snapshots[0].Prefix
	}
	return "unknown-collection"
}

// buildOSSection pulls OS-related parsed payloads out of the
// Collection and merges them across Snapshots onto a single time axis
// per FR-018. SnapshotBoundaries on each returned *Data struct records
// the sample indexes at which a new Snapshot's first sample sits; the
// chart layer draws a vertical boundary marker at each corresponding
// timestamp (FR-030 renderer requirement).
//
// A subview is "missing" only if EVERY Snapshot lacked that collector
// (or every present file failed to parse).
func buildOSSection(c *model.Collection) *model.OSSection {
	sec := &model.OSSection{}
	var ios []*model.IostatData
	var tops []*model.TopData
	var vms []*model.VmstatData
	var mems []*model.MeminfoData
	// Per-file groupings so concat* can emit SnapshotBoundaries at the
	// first poll of each new -netstat / -netstat_s file. Each inner
	// slice is one file's polls (TS blocks), parsers emit one sample
	// per poll.
	var netSockets [][]*model.NetstatSocketsSample
	var netCounters [][]*model.NetstatCountersSample
	for _, snap := range c.Snapshots {
		if sf, ok := snap.SourceFiles[model.SuffixIostat]; ok && sf.Parsed != nil {
			if v, ok := sf.Parsed.(*model.IostatData); ok {
				ios = append(ios, v)
			}
		}
		if sf, ok := snap.SourceFiles[model.SuffixTop]; ok && sf.Parsed != nil {
			if v, ok := sf.Parsed.(*model.TopData); ok {
				tops = append(tops, v)
			}
		}
		if sf, ok := snap.SourceFiles[model.SuffixVmstat]; ok && sf.Parsed != nil {
			if v, ok := sf.Parsed.(*model.VmstatData); ok {
				vms = append(vms, v)
			}
		}
		if sf, ok := snap.SourceFiles[model.SuffixMeminfo]; ok && sf.Parsed != nil {
			if v, ok := sf.Parsed.(*model.MeminfoData); ok {
				mems = append(mems, v)
			}
		}
		if sf, ok := snap.SourceFiles[model.SuffixNetstat]; ok && sf.Parsed != nil {
			if v, ok := sf.Parsed.([]*model.NetstatSocketsSample); ok && len(v) > 0 {
				netSockets = append(netSockets, v)
			}
		}
		if sf, ok := snap.SourceFiles[model.SuffixNetstatS]; ok && sf.Parsed != nil {
			if v, ok := sf.Parsed.([]*model.NetstatCountersSample); ok && len(v) > 0 {
				netCounters = append(netCounters, v)
			}
		}
	}
	sec.Iostat = concatIostat(ios)
	sec.Top = concatTop(tops)
	sec.Vmstat = concatVmstat(vms)
	sec.Meminfo = concatMeminfo(mems)
	sec.NetSockets = concatNetstat(netSockets)
	sec.NetCounters = concatNetstatS(netCounters)
	if sec.Iostat == nil {
		sec.Missing = append(sec.Missing, "-iostat")
	}
	if sec.Top == nil {
		sec.Missing = append(sec.Missing, "-top")
	}
	if sec.Vmstat == nil {
		sec.Missing = append(sec.Missing, "-vmstat")
	}
	if sec.Meminfo == nil {
		sec.Missing = append(sec.Missing, "-meminfo")
	}
	if sec.NetSockets == nil && sec.NetCounters == nil {
		sec.Missing = append(sec.Missing, "-netstat")
	}
	sort.Strings(sec.Missing)
	return sec
}

func buildVariablesSection(c *model.Collection) *model.VariablesSection {
	sec := &model.VariablesSection{}
	for _, snap := range c.Snapshots {
		sv := model.SnapshotVariables{
			SnapshotPrefix: snap.Prefix,
			Timestamp:      snap.Timestamp,
		}
		if sf, ok := snap.SourceFiles[model.SuffixVariables]; ok && sf.Parsed != nil {
			if v, ok := sf.Parsed.(*model.VariablesData); ok {
				sv.Data = v
			}
		}
		sec.PerSnapshot = append(sec.PerSnapshot, sv)
	}
	return sec
}

func buildDBSection(c *model.Collection) *model.DBSection {
	sec := &model.DBSection{}
	var mas []*model.MysqladminData
	var pls []*model.ProcesslistData
	for _, snap := range c.Snapshots {
		si := model.SnapshotInnoDB{
			SnapshotPrefix: snap.Prefix,
			Timestamp:      snap.Timestamp,
		}
		if sf, ok := snap.SourceFiles[model.SuffixInnodbStatus]; ok && sf.Parsed != nil {
			if v, ok := sf.Parsed.(*model.InnodbStatusData); ok {
				si.Data = v
			}
		}
		sec.InnoDBPerSnapshot = append(sec.InnoDBPerSnapshot, si)

		if sf, ok := snap.SourceFiles[model.SuffixMysqladmin]; ok && sf.Parsed != nil {
			if v, ok := sf.Parsed.(*model.MysqladminData); ok {
				mas = append(mas, v)
			}
		}
		if sf, ok := snap.SourceFiles[model.SuffixProcesslist]; ok && sf.Parsed != nil {
			if v, ok := sf.Parsed.(*model.ProcesslistData); ok {
				pls = append(pls, v)
			}
		}
	}

	sec.Mysqladmin = model.MergeMysqladminData(mas)
	sec.Processlist = concatProcesslist(pls)

	if allInnoDBNil(sec.InnoDBPerSnapshot) {
		sec.Missing = append(sec.Missing, "-innodbstatus1")
	}
	if sec.Mysqladmin == nil {
		sec.Missing = append(sec.Missing, "-mysqladmin")
	}
	if sec.Processlist == nil {
		sec.Missing = append(sec.Missing, "-processlist")
	}
	sort.Strings(sec.Missing)
	return sec
}

func allInnoDBNil(xs []model.SnapshotInnoDB) bool {
	for _, x := range xs {
		if x.Data != nil {
			return false
		}
	}
	return true
}

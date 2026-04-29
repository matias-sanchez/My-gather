package model

import (
	"math"
	"sort"
	"time"
)

// MergeMysqladminData merges multiple per-snapshot MysqladminData
// payloads onto one deterministic timestamp axis. Counters reset at
// each snapshot boundary per FR-030: the first post-boundary sample's
// delta is NaN. Gauges concatenate their raw values unchanged.
func MergeMysqladminData(ins []*MysqladminData) *MysqladminData {
	nonNil := ins[:0]
	for _, d := range ins {
		if d != nil && d.SampleCount > 0 {
			nonNil = append(nonNil, d)
		}
	}
	if len(nonNil) == 0 {
		return nil
	}
	if len(nonNil) == 1 {
		return nonNil[0]
	}

	nameSet := map[string]bool{}
	for _, d := range nonNil {
		for _, n := range d.VariableNames {
			nameSet[n] = true
		}
	}
	names := make([]string, 0, len(nameSet))
	for n := range nameSet {
		names = append(names, n)
	}
	sort.Strings(names)

	isCounter := make(map[string]bool, len(names))
	for _, n := range names {
		for _, d := range nonNil {
			if d.IsCounter[n] {
				isCounter[n] = true
				break
			}
		}
	}

	var (
		timestamps []time.Time
		boundaries = make([]int, 0, len(nonNil))
		cumulative int
		deltas     = make(map[string][]float64, len(names))
	)
	for _, n := range names {
		deltas[n] = make([]float64, 0)
	}

	for inputIdx, d := range nonNil {
		boundaries = append(boundaries, cumulative)
		timestamps = append(timestamps, d.Timestamps...)
		for _, n := range d.VariableNames {
			src, present := d.Deltas[n]
			if !present {
				continue
			}
			if inputIdx > 0 && isCounter[n] && len(src) > 0 {
				deltas[n] = append(deltas[n], math.NaN())
				deltas[n] = append(deltas[n], src[1:]...)
			} else {
				deltas[n] = append(deltas[n], src...)
			}
		}

		targetLen := cumulative + d.SampleCount
		for _, n := range names {
			if len(deltas[n]) < targetLen {
				for i := len(deltas[n]); i < targetLen; i++ {
					deltas[n] = append(deltas[n], math.NaN())
				}
			}
		}
		cumulative = targetLen
	}

	return &MysqladminData{
		VariableNames:      names,
		SampleCount:        cumulative,
		Timestamps:         timestamps,
		Deltas:             deltas,
		IsCounter:          isCounter,
		SnapshotBoundaries: boundaries,
	}
}

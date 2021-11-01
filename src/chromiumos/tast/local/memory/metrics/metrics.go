// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package metrics

import (
	"context"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/memory"
	memoryarc "chromiumos/tast/local/memory/arc"
)

// MemoryStatsSnapshot has one snapshot of resettable memory stats.
type MemoryStatsSnapshot struct {
	memstats *memory.ZramStats

	// psistats will be nil on systems that don't support it.
	psistats *memory.PSIStats
}

// BaseMemoryStats holds initial metrics, which can be used as a baseline
// for ever-growing metrics (snapshot early in the test, subtract at the end).
type BaseMemoryStats struct {
	initialstate *MemoryStatsSnapshot
	// lateststate starts as nil and is refreshed every time we log.
	// its main purpose is to be a cache to enable fast reset
	lateststate *MemoryStatsSnapshot
}

// Clone creates a deep copy of the provided pointer
func (t *MemoryStatsSnapshot) Clone() *MemoryStatsSnapshot {
	deepcopy := *t
	if t.memstats != nil {
		deepcopy.memstats = t.memstats.Clone()
	}
	if t.psistats != nil {
		deepcopy.psistats = t.psistats.Clone()
	}
	return &deepcopy
}

// Clone creates a deep copy of the provided pointer
func (base *BaseMemoryStats) Clone() *BaseMemoryStats {
	deepcopy := *base
	if base.initialstate != nil {
		deepcopy.initialstate = base.initialstate.Clone()
	}
	if base.lateststate != nil {
		deepcopy.lateststate = base.lateststate.Clone()
	}
	return &deepcopy
}

// NewBaseMemoryStats gathers data on ever-growing metrics, so that they can
// be a baseline to subtract from the same metrics at a later time.
// A test will ideally perform this sequence:
//     base := metrics.NewBaseMemoryStats()
//     ..run the test..
//     metrics.LogMemoryStats( ..., base, ...)
func NewBaseMemoryStats(ctx context.Context, arc *arc.ARC) (*BaseMemoryStats, error) {
	basezram, err := memory.NewZramStats()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get baseline memory stats")
	}
	basepsi, err := memory.NewPSIStats(ctx, arc)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get baseline PSI stats")
	}
	return &BaseMemoryStats{
		initialstate: &MemoryStatsSnapshot{memstats: basezram, psistats: basepsi},
		lateststate:  nil,
	}, nil
}

// Reset sets the base metrics values to "now", so they can be
// a new baseline reflecting the time of "now".
func (base *BaseMemoryStats) Reset() error {
	if base.lateststate == nil {
		return errors.New("attempt to reset without ever logging")
	}

	// Throw away the initial state, and replace it with the newest snapshot.
	base.initialstate = base.lateststate
	base.lateststate = nil

	return nil
}

// LogMemoryStatsSlice generates metrics for a time slice covering
// the difference between the latest snapshots passed in the begin and end
// memory stat sets.
// This is useful for dumping metrics covering a rolling time slice,
// and is only done for the metrics where "delta" makes sense.
func LogMemoryStatsSlice(begin, end *BaseMemoryStats, p *perf.Values, suffix string) {
	if begin.lateststate == nil {
		return
	}
	if end.lateststate == nil {
		return
	}

	memory.LogZramStatMetricsSlice(
		begin.lateststate.memstats,
		end.lateststate.memstats,
		p,
		suffix)

	memory.LogPSIMetricsSlice(
		begin.lateststate.psistats,
		end.lateststate.psistats,
		p,
		suffix)
}

// LogMemoryStats generates log files detailing the memory from the following:
// * smaps_rollup per process.
// * fincore per file used as a disk by crsovm.
// * zram compressed swap memory use and zram overall usage stats.
// * Android adb dumpsys meminfo.
// * PSI memory metrics.
// If p != nil, summaries are added as perf.Values.
// Parameter base is optional - when specified, metrics are computed
// as a delta from the base snapshot til "now" (where possible).
func LogMemoryStats(ctx context.Context, base *BaseMemoryStats, arc *arc.ARC, p *perf.Values, outdir, suffix string) error {

	// These metrics are relatively cheap to get.
	if err := memory.ZramMmStatMetrics(ctx, p, outdir, suffix); err != nil {
		return errors.Wrap(err, "failed to collect zram mm_stats metrics")
	}

	var basecopy *MemoryStatsSnapshot
	var zramprevstats *memory.ZramStats
	var psiprevstats *memory.PSIStats
	if base != nil {
		basecopy = base.initialstate.Clone()
		zramprevstats = basecopy.memstats
		psiprevstats = basecopy.psistats
	}

	if err := memory.ZramStatMetrics(ctx, zramprevstats, p, outdir, suffix); err != nil {
		return errors.Wrap(err, "failed to collect zram stats metrics")
	}

	if err := memory.PSIMetrics(ctx, arc, psiprevstats, p, outdir, suffix); err != nil {
		return errors.Wrap(err, "failed to collect PSI stats metrics")
	}

	if base != nil {
		base.lateststate = basecopy
	}

	// Order is critical here: SmapsMetrics and CrosvmFincoreMetrics do heavy processing,
	// and we don't want that processing to interfere in the earlier, cheaper stats.
	if err := memory.SmapsMetrics(ctx, p, outdir, suffix); err != nil {
		return errors.Wrap(err, "failed to collect smaps_rollup metrics")
	}

	if err := memory.CrosvmFincoreMetrics(ctx, p, outdir, suffix); err != nil {
		return errors.Wrap(err, "failed to collect crosvm fincore metrics")
	}

	if err := memory.ChromeOSAvailableMetrics(ctx, p, suffix); err != nil {
		return errors.Wrap(err, "failed to collect ChromeOS available metrics")
	}

	if arc != nil {
		if err := memoryarc.DumpsysMeminfoMetrics(ctx, arc, p, outdir, suffix); err != nil {
			return errors.Wrap(err, "failed to collect ARC dumpsys meminfo metrics")
		}
	}

	return nil
}

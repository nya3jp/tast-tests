// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package metrics

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/memory"
	memoryarc "chromiumos/tast/local/memory/arc"
	"chromiumos/tast/testing"
)

// BaseMemoryStats holds initial metrics, which can be used as a baseline
// for ever-growing metrics (snapshot early in the test, subtract at the end).
type BaseMemoryStats struct {
	memstats *memory.ZramStats

	// psistats will be nil on systems that don't support it.
	psistats *memory.PSIStats
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
	return &BaseMemoryStats{memstats: basezram, psistats: basepsi}, nil
}

// Reset sets the base metrics values to "now", so they can be
// a new baseline reflecting the time of "now".
func (base *BaseMemoryStats) Reset(ctx context.Context, arc *arc.ARC) error {
	basezram, err := memory.NewZramStats()
	if err != nil {
		return errors.Wrap(err, "unable to get baseline memory stats for reset")
	}
	basepsi, err := memory.NewPSIStats(ctx, arc)
	if err != nil {
		return errors.Wrap(err, "unable to get baseline PSI stats")
	}
	base.memstats = basezram
	base.psistats = basepsi // May be nil, that's okay.
	return nil
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

	var zramprevstats *memory.ZramStats
	var psiprevstats *memory.PSIStats
	if base != nil {
		zramprevstats = base.memstats
		psiprevstats = base.psistats
	}
	if err := memory.ZramStatMetrics(ctx, zramprevstats, p, outdir, suffix); err != nil {
		return errors.Wrap(err, "failed to collect zram stats metrics")
	}
	if err := memory.PSIMetrics(ctx, arc, psiprevstats, p, outdir, suffix); err != nil {
		return errors.Wrap(err, "failed to collect PSI stats metrics")
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
		const dumpsysRetries = 3
		for i := 0; ; i++ {
			// TODO(b:191802472): Make DumpsysMeminfoMetrics silently fail if
			// retries are not enough.
			if err := memoryarc.DumpsysMeminfoMetrics(ctx, arc, p, outdir, suffix); err != nil {
				if i < dumpsysRetries {
					testing.ContextLog(ctx, "Failed to collect ARC dumpsys meminfo metrics: ", err)
					if err := testing.Sleep(ctx, 5*time.Second); err != nil {
						return errors.Wrap(err, "failed to sleep between dumpsys meminfo retries")
					}
					continue
				}
				return errors.Wrapf(err, "failed to collect ARC dumpsys meminfo metrics %d times", i)
			}
			break
		}
	}
	return nil
}

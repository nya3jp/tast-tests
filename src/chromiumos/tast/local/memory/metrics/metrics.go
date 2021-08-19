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

// BaseMetrics holds initial metrics, which can be used as a baseline
// for ever-growing metrics (snapshot early in the test, subtract at the end).
type BaseMetrics struct {
	memstats *memory.ZramStats
}

// NewBaseMetrics gathers data on ever-growing metrics, so that they can
// be a baseline to subtract from the same metrics at a later time.
// A test will ideally perform this sequence:
//     base := NewBaseMetrics()
//     ..run the test..
//     MemoryMetrics( ..., base, ...)
func NewBaseMetrics() (*BaseMetrics, error) {
	basezram, err := memory.NewZramStats()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get baseline memory stats")
	}
	return &BaseMetrics{memstats: basezram}, nil
}

// ResetBaseMetrics sets the base metrics values to "now", so they can be
// a new baseline reflecting the time of "now"
func ResetBaseMetrics(base *BaseMetrics) error {
	basezram, err := memory.NewZramStats()
	if err != nil {
		return errors.Wrap(err, "unable to get baseline memory stats for reset")
	}
	base.memstats = basezram
	return nil
}

// MemoryMetrics generates log files detailing the memory from the following:
// * smaps_rollup per process.
// * fincore per file used as a disk by crsovm.
// * zram compressed swap memory use and zram overall usage stats
// * Android adb dumpsys meminfo.
// If p != nil, summaries are added as perf.Values.
func MemoryMetrics(ctx context.Context, base *BaseMetrics, preARC *arc.ARC, p *perf.Values, outdir, suffix string) error {

	// these metrics are relatively cheap to get
	if err := memory.ZramMmStatMetrics(ctx, p, outdir, suffix); err != nil {
		return errors.Wrap(err, "failed to collect zram mm_stats metrics")
	}

	var zramprevstats *memory.ZramStats
	if base != nil {
		zramprevstats = base.memstats
	}
	if err := memory.ZramStatMetrics(ctx, zramprevstats, p, outdir, suffix); err != nil {
		return errors.Wrap(err, "failed to collect zram stats metrics")
	}

	// order is critical here: SmapsMetrics and CrosvmFincoreMetrics do heavy processing,
	// we don't want that processing to interfere in the earlier, cheaper stats
	if err := memory.SmapsMetrics(ctx, p, outdir, suffix); err != nil {
		return errors.Wrap(err, "failed to collect smaps_rollup metrics")
	}
	if err := memory.CrosvmFincoreMetrics(ctx, p, outdir, suffix); err != nil {
		return errors.Wrap(err, "failed to collect crosvm fincore metrics")
	}

	if preARC != nil {
		if err := memoryarc.DumpsysMeminfoMetrics(ctx, preARC, p, outdir, suffix); err != nil {
			return errors.Wrap(err, "failed to collect ARC dumpsys meminfo metrics")
		}
	}
	return nil
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package multivm

import (
	"context"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/memory"
	"chromiumos/tast/local/memory/arc"
)

// BaseMetrics holds initial metrics, which can be used as a baseline
// for ever-growing metrics (snapshot early in the test, subtract at the end)
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

// MemoryMetrics generates log files detailing the memory from the following:
// * smaps_rollup per process.
// * fincore per file used as a disk by crsovm.
// * zram compressed swap memory use and zram overall usage stats
// * Android adb dumpsys meminfo.
// If p != nil, summaries are added as perf.Values.
func MemoryMetrics(ctx context.Context, base *BaseMetrics, pre *PreData, p *perf.Values, outdir, suffix string) error {
	if err := memory.SmapsMetrics(ctx, p, outdir, suffix); err != nil {
		return errors.Wrap(err, "failed to collect smaps_rollup metrics")
	}
	if err := memory.CrosvmFincoreMetrics(ctx, p, outdir, suffix); err != nil {
		return errors.Wrap(err, "failed to collect crosvm fincore metrics")
	}
	if err := memory.ZramMmStatMetrics(ctx, p, outdir, suffix); err != nil {
		return errors.Wrap(err, "failed to collect zram mm_stats metrics")
	}
	if err := memory.ZramStatMetrics(ctx, base.memstats, p, outdir, suffix); err != nil {
		return errors.Wrap(err, "failed to collect zram stats metrics")
	}

	preARC := ARCFromPre(pre)
	if preARC != nil {
		if err := arc.DumpsysMeminfoMetrics(ctx, preARC, p, outdir, suffix); err != nil {
			return errors.Wrap(err, "failed to collect ARC dumpsys meminfo metrics")
		}
	}
	return nil
}

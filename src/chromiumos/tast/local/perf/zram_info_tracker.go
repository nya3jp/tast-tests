// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perf

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	zramDevPath             = "/dev/zram0"
	zramMmStatPath          = "/sys/block/zram0/mm_stat"
	zramResetMemUsedMaxPath = "/sys/block/zram0/mem_used_max"
)

var zramFieldNames = []string{
	"OrigDataSize",
	"ComprDataSize",
	"MemUsedTotal",
	"MemLimit",
	"Max",
	"SamePages",
	"PagesCompacted",
	"HugePages",
	"HugePagesSince",
}

// ZramInfoTracker is a helper to collect zram info.
type ZramInfoTracker struct {
	hasZram         bool
	prefix          string
	stats           []float64
	memUsedMaxStart float64
}

// NewZramInfoTracker creates a new instance of ZramInfoTracker. If zram is not
// used on the device, hasZram flag is set to false and makes track a no-op.
func NewZramInfoTracker(metricPrefix string) (*ZramInfoTracker, error) {
	hasZram := false

	if fi, err := os.Stat(zramDevPath); err == nil {
		m := fi.Mode() &^ 07777
		hasZram = m == os.ModeDevice
	}

	return &ZramInfoTracker{
		prefix:  metricPrefix,
		hasZram: hasZram,
	}, nil
}

func getMMStat(ctx context.Context) ([]float64, error) {
	out, err := testexec.CommandContext(ctx,
		"cat", zramMmStatPath).Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to dump zram mm_stat")
	}

	// File /sys/block/zram<id>/mm_stat
	//
	// The stat file represents device's mm statistics. It consists of a single
	// line of text and contains the stats separated by whitespace. The order of
	// the fields are as follows:
	// 1. orig_data_size
	// 2. compr_data_size
	// 3. mem_used_total
	// 4. mem_limit
	// 5. mem_used_max
	// 6. same_pages
	// 7. pages_compacted
	// 8. huge_pages (optional)
	// 9. huge_pages_since (optional)
	//
	// See https://www.kernel.org/doc/html/latest/admin-guide/blockdev/zram.html

	statsRaw := strings.Fields(string(out))
	numFields := len(statsRaw)
	stats := make([]float64, numFields)

	testing.ContextLog(ctx, string(out))
	testing.ContextLogf(ctx, "stats length: %d", len(statsRaw))
	for i := 0; i < numFields; i++ {
		stats[i], err = strconv.ParseFloat(statsRaw[i], 64)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse %s", zramFieldNames[i])
		}
	}

	return stats, nil
}

// Start indicates that the zram tracking should start. It resets the mem_used_max
// counter and captures the value after reset.
func (t *ZramInfoTracker) Start(ctx context.Context) error {
	if !t.hasZram {
		return nil
	}

	// Reset "mem_used_max" counter.
	if err := testexec.CommandContext(ctx,
		"sh", "-c", fmt.Sprintf("echo 0 > %q", zramResetMemUsedMaxPath)).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to reset mem_used_max counter")
	}

	stats, err := getMMStat(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to read mm_stat")
	}

	t.memUsedMaxStart = stats[4]

	return nil
}

// Stop indicates that the zram tracking should stop. It reads the current
// mm_stat and store relevant info.
func (t *ZramInfoTracker) Stop(ctx context.Context) error {
	if !t.hasZram {
		return nil
	}

	stats, err := getMMStat(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to read mm_stat")
	}

	// Update the current mem_used_max if the previous max was higher
	maxIdx := 4
	stats[maxIdx] = math.Max(t.memUsedMaxStart, stats[maxIdx])

	t.stats = stats

	if t.stats[maxIdx] == t.memUsedMaxStart {
		testing.ContextLog(ctx, "Zram mem_used_max is not changed")
	}

	return nil
}

// Record stores the collected data into pv for further processing.
func (t *ZramInfoTracker) Record(pv *perf.Values) {
	if !t.hasZram {
		return
	}

	for i, stat := range t.stats {
		pv.Set(perf.Metric{
			Name:      t.prefix + "RAM.Zram." + zramFieldNames[i],
			Unit:      "bytes",
			Direction: perf.SmallerIsBetter,
		}, stat)
	}
}

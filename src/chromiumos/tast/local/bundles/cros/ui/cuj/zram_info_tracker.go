// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	zramDevPath             = "/dev/zram0"
	zramMmStatPath          = "/sys/block/zram0/mm_stat"
	zramResetMemUsedMaxPath = "/sys/block/zram0/mem_used_max"
)

// ZramInfoTracker is a helper to collect zram info.
type ZramInfoTracker struct {
	hasZram         bool
	prefix          string
	memUsedMaxStart float64
	memUsedMaxEnd   float64
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

// getMmUsedMax gets the "mem_used_max" in zram's mm_stat.
func getMmUsedMax(ctx context.Context) (float64, error) {
	out, err := testexec.CommandContext(ctx,
		"cat", zramMmStatPath).Output(testexec.DumpLogOnError)
	if err != nil {
		return 0, errors.Wrap(err, "failed to dump zram mm_stat")
	}

	// File /sys/block/zram<id>/mm_stat
	//
	// The stat file represents device's mm statistics. It consists of a single
	// line of text and contains the stats separated by whitespace. "mm_used_max"
	// is the 5th field.
	//
	// mem_used_max     the maximum amount of memory zram have consumed to
	//                  store the data
	//
	// See https://www.kernel.org/doc/html/latest/admin-guide/blockdev/zram.html
	memUsedMax, err := strconv.ParseFloat(strings.Fields(string(out))[4], 64)
	if err != nil {
		return 0, errors.Wrap(err, "failed to parse signal strength field")
	}
	return memUsedMax, nil
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

	memUsedMax, err := getMmUsedMax(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to read mem_used_max counter")
	}
	t.memUsedMaxStart = memUsedMax

	return nil
}

// Stop indicates that the zram tracking should stop. It reads the current
// mm_stat and store relevant info.
func (t *ZramInfoTracker) Stop(ctx context.Context) error {
	if !t.hasZram {
		return nil
	}

	memUsedMax, err := getMmUsedMax(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to read mem_used_max counter")
	}
	t.memUsedMaxEnd = math.Max(t.memUsedMaxEnd, memUsedMax)

	if t.memUsedMaxEnd == t.memUsedMaxStart {
		testing.ContextLog(ctx, "Zram mem_used_max is not changed")
	}

	return nil
}

// Record stores the collected data into pv for further processing.
func (t *ZramInfoTracker) Record(pv *perf.Values) {
	if !t.hasZram {
		return
	}

	pv.Set(perf.Metric{
		Name:      "TPS.RAM.Zram.Max",
		Unit:      "bytes",
		Direction: perf.SmallerIsBetter,
	}, float64(t.memUsedMaxEnd))
}

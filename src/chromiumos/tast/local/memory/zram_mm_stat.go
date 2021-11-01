// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
)

// ZramMmStat holds statistics from /sys/block/zram*/mm_stat.
type ZramMmStat struct {
	OrigDataSize, ComprDataSize, MemUsedTotal, MemLimit, MemUsedMax, SamePages, PagesCompacted uint64
	// Some fields have been introduced by newer kernels, make them optional.
	HugePages, HugePagesSince *uint64 `json:",omitempty"`
}

// NewZramMmStat parses /sys/block/zram*/mm_stat to create a ZramMmStat.
func NewZramMmStat() (*ZramMmStat, error) {
	files, err := filepath.Glob("/sys/block/zram*/mm_stat")
	if err != nil {
		return nil, errors.Wrap(err, "failed to find zram<id>/mm_stat file")
	} else if len(files) != 1 {
		return nil, errors.Errorf("expected 1 zram device, got %d", len(files))
	}
	mmStat, err := ioutil.ReadFile(files[0])
	if err != nil {
		return nil, errors.Wrap(err, "failed to read mm_stat")
	}
	fields := strings.Fields(string(mmStat))
	if len(fields) < 7 {
		return nil, errors.Errorf("expected at least 7 fields in mm_stat file, got %d", len(fields))
	}

	parsedFields := make([]uint64, len(fields))
	for i, stat := range fields {
		parsed, err := strconv.ParseUint(stat, 10, 64)
		if err != nil {
			return nil, errors.Errorf("failed to parse field %d in mm_stat %q", i, stat)
		}
		parsedFields[i] = parsed
	}

	stats := &ZramMmStat{
		OrigDataSize:   parsedFields[0],
		ComprDataSize:  parsedFields[1],
		MemUsedTotal:   parsedFields[2],
		MemLimit:       parsedFields[3],
		MemUsedMax:     parsedFields[4],
		SamePages:      parsedFields[5],
		PagesCompacted: parsedFields[6],
	}
	if len(parsedFields) > 7 {
		stats.HugePages = &parsedFields[7]
	}
	if len(parsedFields) > 8 {
		stats.HugePagesSince = &parsedFields[8]
	}

	return stats, nil
}

// ZramMmStatMetrics writes a JSON file containing statistics from
// /sys/block/zram/mm_stat. If outdir is "", then no logs are written.
func ZramMmStatMetrics(ctx context.Context, p *perf.Values, outdir, suffix string) error {
	stat, err := NewZramMmStat()
	if err != nil {
		return err
	}
	if len(suffix) > 0 {
		fmt.Sprintf("blah")
	}
	if stat != nil {
		fmt.Sprintf("blah")
	}

	if len(outdir) > 0 {
		statJSON, err := json.MarshalIndent(stat, "", "  ")
		if err != nil {
			return errors.Wrap(err, "failed to serialize mm_stat metrics to JSON")
		}
		filename := fmt.Sprintf("zram_mm_stat%s.json", suffix)
		if err := ioutil.WriteFile(path.Join(outdir, filename), statJSON, 0644); err != nil {
			return errors.Wrapf(err, "failed to write zram mm_stats to %s", filename)
		}
	}

	if p == nil {
		// No perf.Values, so exit without computing the metrics.
		return nil
	}

	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("zram_original%s", suffix),
			Unit:      "MiB",
			Direction: perf.SmallerIsBetter,
		},
		float64(stat.OrigDataSize)/MiB,
	)
	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("zram_compressed%s", suffix),
			Unit:      "MiB",
			Direction: perf.SmallerIsBetter,
		},
		float64(stat.ComprDataSize)/MiB,
	)
	return nil
}

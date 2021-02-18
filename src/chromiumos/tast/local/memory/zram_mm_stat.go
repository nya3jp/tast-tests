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
	OrigDataSize, ComprDataSize, MemUsedTotal, MemLimit, MemUsedMax, SamePages, PagesCompacted, HugePages uint64
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
	stats := strings.Fields(string(mmStat))
	if len(stats) != 8 {
		return nil, errors.Errorf("expected 8 fields in mm_stat file, got %d", len(stats))
	}

	parsedStats := make([]uint64, len(stats))
	for i, stat := range stats {
		parsed, err := strconv.ParseUint(stat, 10, 64)
		if err != nil {
			return nil, errors.Errorf("failed to parse field %d in mm_stat %q", i, stat)
		}
		parsedStats[i] = parsed
	}

	return &ZramMmStat{
		OrigDataSize:   parsedStats[0],
		ComprDataSize:  parsedStats[1],
		MemUsedTotal:   parsedStats[2],
		MemLimit:       parsedStats[3],
		MemUsedMax:     parsedStats[4],
		SamePages:      parsedStats[5],
		PagesCompacted: parsedStats[6],
		HugePages:      parsedStats[7],
	}, nil
}

// ZramMmStatMetrics writes a JSON file containing statistics from
// /sys/block/zram/mm_stat. TODO: add stats to perf.Values if they are passed.
func ZramMmStatMetrics(ctx context.Context, p *perf.Values, outdir, suffix string) error {
	stat, err := NewZramMmStat()
	if err != nil {
		return err
	}
	statJSON, err := json.MarshalIndent(stat, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to serialize mm_stat metrics to JSON")
	}
	filename := fmt.Sprintf("zram_mm_stat%s.json", suffix)
	if err := ioutil.WriteFile(path.Join(outdir, filename), statJSON, 0644); err != nil {
		return errors.Wrapf(err, "failed to write zram mm_stats to %s", filename)
	}

	if p == nil {
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

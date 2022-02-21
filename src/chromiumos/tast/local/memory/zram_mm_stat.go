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

// ZramSummary holds a summary of ZRAM usage by the host.
// All values in Kilobytes.
type ZramSummary struct {
	OrigDataSize  uint64
	ComprDataSize uint64
	MemUsedTotal  uint64
}

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

// GetZramMmStatMetrics parses statistics from
// /sys/block/zram/mm_stat and returns a summary of them as ZramSummary.
// If outdir is provided, intermediate files are dumped to that directory.
func GetZramMmStatMetrics(ctx context.Context, outdir, suffix string) (*ZramSummary, error) {
	stat, err := NewZramMmStat()
	if err != nil {
		return nil, err
	}
	if len(outdir) > 0 {
		statJSON, err := json.MarshalIndent(stat, "", "  ")
		if err != nil {
			return nil, errors.Wrap(err, "failed to serialize mm_stat metrics to JSON")
		}
		filename := fmt.Sprintf("zram_mm_stat%s.json", suffix)
		if err := ioutil.WriteFile(path.Join(outdir, filename), statJSON, 0644); err != nil {
			return nil, errors.Wrapf(err, "failed to write zram mm_stats to %s", filename)
		}
	}

	// Convert reported byte values to kilobytes
	return &ZramSummary{
		OrigDataSize:  stat.OrigDataSize / KiB,
		ComprDataSize: stat.ComprDataSize / KiB,
		MemUsedTotal:  stat.MemUsedTotal / KiB,
	}, nil
}

// ReportZramMmStatMetrics outputs a set of representative metrics
// into the supplied performance data dictionary.
func ReportZramMmStatMetrics(summary *ZramSummary, p *perf.Values, suffix string) {
	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("zram_original%s", suffix),
			Unit:      "MiB",
			Direction: perf.SmallerIsBetter,
		},
		float64(summary.OrigDataSize)/KiBInMiB,
	)
	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("zram_compressed%s", suffix),
			Unit:      "MiB",
			Direction: perf.SmallerIsBetter,
		},
		float64(summary.ComprDataSize)/KiBInMiB,
	)
}

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

// ZramOpStats holds stats for one transaction type (read, write, discard).
type ZramOpStats struct {
	Ops, MergedOps, Sectors, Wait uint64
}

// discardStatColumns is the number of primitive fields in ZramStats,
// on board where discard stat information is available
const discardStatColumns = 15

// minStatColumns is the number of primitive fields in ZramStats,
// on older boards, which didn't have the last few columns
const minStatColumns = 11

// ZramStats holds statistics from /sys/block/zram*/stat.
type ZramStats struct {
	Read                              ZramOpStats
	Write                             ZramOpStats
	InFlightOps, IoTicks, TimeInQueue uint64
	Discard                           *ZramOpStats
}

// NewZramStats parses /sys/block/zram*/stat to create a ZramStats.
func NewZramStats() (*ZramStats, error) {
	files, err := filepath.Glob("/sys/block/zram*/stat")
	if err != nil {
		return nil, errors.Wrap(err, "failed to find zram<id>/stat file")
	} else if len(files) != 1 {
		return nil, errors.Errorf("expected 1 zram device, got %d", len(files))
	}
	statBlob, err := ioutil.ReadFile(files[0])
	if err != nil {
		return nil, errors.Wrap(err, "failed to read zram stat")
	}
	strFields := strings.Fields(string(statBlob))
	numFields := len(strFields)
	hasDiscard := false
	if numFields < discardStatColumns {
		if numFields < minStatColumns {
			return nil, errors.Errorf("expected %d or %d fields in stat file, got %d", discardStatColumns, minStatColumns, numFields)
		}
	} else {
		hasDiscard = true
	}

	fields := make([]uint64, numFields)
	for i, stat := range strFields {
		parsed, err := strconv.ParseUint(stat, 10, 64)
		if err != nil {
			return nil, errors.Errorf("failed to parse field %d in stat %q", i, stat)
		}
		fields[i] = parsed
	}

	stats := &ZramStats{
		Read:        ZramOpStats{Ops: fields[0], MergedOps: fields[1], Sectors: fields[2], Wait: fields[3]},
		Write:       ZramOpStats{Ops: fields[4], MergedOps: fields[5], Sectors: fields[6], Wait: fields[7]},
		InFlightOps: fields[8],
		IoTicks:     fields[9],
		TimeInQueue: fields[10],
		// intentionally omit Discard, set optionally below
	}
	if hasDiscard {
		stats.Discard = &ZramOpStats{Ops: fields[11], MergedOps: fields[12], Sectors: fields[13], Wait: fields[14]}
	}

	return stats, nil
}

func subtractBaseOps(ops, base *ZramOpStats) {
	// subtract the base from all ever-growing metrics.
	ops.Ops -= base.Ops
	ops.MergedOps -= base.MergedOps
	ops.Sectors -= base.Sectors
	ops.Wait -= base.Wait
}

func subtractBaseStat(stat, base *ZramStats) {
	subtractBaseOps(&(stat.Read), &(base.Read))
	subtractBaseOps(&(stat.Write), &(base.Write))
	stat.IoTicks -= base.IoTicks
	stat.TimeInQueue -= base.TimeInQueue
	if stat.Discard != nil && base.Discard != nil {
		subtractBaseOps(stat.Discard, base.Discard)
	}
}

// ZramStatMetrics writes a JSON file containing statistics from
// /sys/block/zram*/stat. If outdir is "", then no logs are written.
func ZramStatMetrics(ctx context.Context, base *ZramStats, p *perf.Values, outdir, suffix string) error {
	stat, err := NewZramStats()
	if err != nil {
		return err
	}
	if base != nil {
		subtractBaseStat(stat, base)
	}

	if len(outdir) > 0 {
		statJSON, err := json.MarshalIndent(stat, "", "  ")
		if err != nil {
			return errors.Wrap(err, "failed to serialize mm_stat metrics to JSON")
		}
		filename := fmt.Sprintf("zram_stat%s.json", suffix)
		if err := ioutil.WriteFile(path.Join(outdir, filename), statJSON, 0644); err != nil {
			return errors.Wrapf(err, "failed to write zram stats to %s", filename)
		}
	}

	if p == nil {
		// No perf.Values, so exit without computing the metrics.
		return nil
	}

	// Q: which ones to pick for perf metric?
	// A: decided set based on suggestion in b/184892486#comment7 .
	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("zram_reads%s", suffix),
			Unit:      "Requests",
			Direction: perf.BiggerIsBetter, // really, it depends.
		},
		float64(stat.Read.Ops),
	)
	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("zram_writes%s", suffix),
			Unit:      "Requests",
			Direction: perf.BiggerIsBetter, // really, it depends.
		},
		float64(stat.Write.Ops),
	)
	if stat.Discard != nil {
		p.Set(
			perf.Metric{
				Name:      fmt.Sprintf("zram_discards%s", suffix),
				Unit:      "Requests",
				Direction: perf.SmallerIsBetter, // really, it depends.
			},
			float64(stat.Discard.Ops),
		)
	}
	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("zram_wait%s", suffix),
			Unit:      "ms",
			Direction: perf.SmallerIsBetter,
		},
		float64(stat.TimeInQueue),
	)
	return nil
}

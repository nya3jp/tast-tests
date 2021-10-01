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
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
)

const (
	psiFilename   = "/proc/pressure/memory"
	psiLineFormat = " avg10=%f avg60=%f avg300=%f total=%d"
	psiNItems     = 4
	psiSomeTag    = "some"
	psiFullTag    = "full"
)

// PSIDetail holds one line of statistics from memory pressure dumps.
type PSIDetail struct {
	Avg10, Avg60, Avg300 float64
	Total                uint64
}

// PSIOneSystemStats holds statistics from memory pressure dumps for one system.
type PSIOneSystemStats struct {
	Some PSIDetail
	Full PSIDetail
}

// PSIStats holds statistics from memory pressure dumps.
type PSIStats struct {
	Host      *PSIOneSystemStats
	Arc       *PSIOneSystemStats
	Timestamp time.Time
}

// newPSISystemStats parses PSI text output into a struct for one system.
func newPSISystemStats(statBlob []byte) (*PSIOneSystemStats, error) {
	stats := &PSIOneSystemStats{}
	blocks := []struct {
		tag  string
		data *PSIDetail
	}{
		{psiSomeTag, &(stats.Some)},
		{psiFullTag, &(stats.Full)},
	}
	statString := string(statBlob)
	lines := strings.SplitN(statString, "\n", len(blocks))
	if len(lines) != len(blocks) {
		return nil, errors.Errorf("PSI metrics file[%s] should have %d lines, found %d", statString, len(blocks), len(lines))
	}
	for i, line := range lines {
		tag := blocks[i].tag
		d := blocks[i].data
		if nitems, err := fmt.Sscanf(line, tag+psiLineFormat, &(d.Avg10), &(d.Avg60), &(d.Avg300), &(d.Total)); nitems != psiNItems {
			return nil, errors.Wrapf(err, "found %d PSI fields, expected %d, file[%s]", nitems, psiNItems, statString)
		}
	}
	return stats, nil
}

// NewPSIStats parses /proc/pressure/memory to create a PSIStats.
func NewPSIStats(ctx context.Context, a *arc.ARC) (*PSIStats, error) {
	stats := &PSIStats{Timestamp: time.Now()}

	// Gather PSI from the host first.
	statBlob, err := ioutil.ReadFile(psiFilename)
	if err == nil {
		stats.Host, err = newPSISystemStats(statBlob)
		if err != nil {
			return nil, errors.Wrap(err, "error parsing Host PSI")
		}
	} // Otherwise, it is a host that does not support PSI.

	if a != nil && ctx != nil {
		out, err := a.Command(ctx, "cat", psiFilename).Output(testexec.DumpLogOnError)
		if err == nil {
			stats.Arc, err = newPSISystemStats(out)
			if err != nil {
				return nil, errors.Wrap(err, "error parsing Arc PSI")
			}
		} // Otherwise, this ARC does not allow access to PSI yet.
	}

	return stats, nil
}

func psiDetailMetrics(tag, sysname, suffix string, detail *PSIDetail, p *perf.Values, includeTotal bool) {
	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("%spsi_%s_avg10%s", sysname, tag, suffix),
			Unit:      "Percentage",
			Direction: perf.SmallerIsBetter,
		},
		detail.Avg10,
	)
	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("%spsi_%s_avg60%s", sysname, tag, suffix),
			Unit:      "Percentage",
			Direction: perf.SmallerIsBetter,
		},
		detail.Avg60,
	)
	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("%spsi_%s_avg300%s", sysname, tag, suffix),
			Unit:      "Percentage",
			Direction: perf.SmallerIsBetter,
		},
		detail.Avg300,
	)

	if includeTotal {
		p.Set(
			perf.Metric{
				Name:      fmt.Sprintf("%spsi_%s_total%s", sysname, tag, suffix),
				Unit:      "Microseconds",
				Direction: perf.SmallerIsBetter,
			},
			float64(detail.Total),
		)
	}
}

// psiDeltaMetrics dumps PSI metrics for one system (host or guest).
func psiDeltaMetrics(base, stat *PSIOneSystemStats, elapsedMicroseconds int64, p *perf.Values, sysname, suffix string) {

	// Ignore inverted timings, which would generate noise.
	// Inversion is the result of incorrect calling or
	// (rare but normal) total counter wrap-arounds.
	if elapsedMicroseconds <= 0 {
		return
	}
	if stat.Some.Total >= base.Some.Total {
		diff := float64(stat.Some.Total - base.Some.Total)
		rate := diff / float64(elapsedMicroseconds)
		rate *= 100.0
		p.Set(
			perf.Metric{
				Name:      fmt.Sprintf("%spsi_some_custom%s", sysname, suffix),
				Unit:      "Percentage",
				Direction: perf.SmallerIsBetter,
			},
			rate,
		)
		p.Set(
			perf.Metric{
				Name:      fmt.Sprintf("%spsi_some_delta%s", sysname, suffix),
				Unit:      "Microseconds",
				Direction: perf.SmallerIsBetter,
			},
			diff,
		)
	}
	if stat.Full.Total >= base.Full.Total {
		diff := float64(stat.Full.Total - base.Full.Total)
		rate := diff / float64(elapsedMicroseconds)
		rate *= 100.0
		p.Set(
			perf.Metric{
				Name:      fmt.Sprintf("%spsi_full_custom%s", sysname, suffix),
				Unit:      "Percentage",
				Direction: perf.SmallerIsBetter,
			},
			rate,
		)
		p.Set(
			perf.Metric{
				Name:      fmt.Sprintf("%spsi_full_delta%s", sysname, suffix),
				Unit:      "Microseconds",
				Direction: perf.SmallerIsBetter,
			},
			diff,
		)
	}
}

// PSIMetrics writes a JSON file containing statistics from PSI metrics.
// Parameter base is optional:
// * if base is set, it defines the starting point for metrics;
// * if base is nil, metrics are averaged since boot.
// If outdir is "", then no logs are written.
func PSIMetrics(ctx context.Context, a *arc.ARC, base *PSIStats, p *perf.Values, outdir, suffix string) error {
	stat, err := NewPSIStats(ctx, a)
	if err != nil {
		return err
	}
	if stat == nil {
		return nil
	}

	if len(outdir) > 0 {
		statJSON, err := json.MarshalIndent(stat, "", "  ")
		if err != nil {
			return errors.Wrap(err, "failed to serialize psi metrics to JSON")
		}
		filename := fmt.Sprintf("psi%s.json", suffix)
		if err := ioutil.WriteFile(path.Join(outdir, filename), statJSON, 0644); err != nil {
			return errors.Wrapf(err, "failed to write psi stats to %s", filename)
		}
	}

	if p == nil {
		// No perf.Values, so exit without computing metrics.
		return nil
	}

	includeTotalSinceBoot := true
	if base != nil {
		elapsedMicroseconds := stat.Timestamp.Sub(base.Timestamp).Microseconds()

		// We will log blocked times during a custom interval, so no need for these,
		// which are less useful.
		includeTotalSinceBoot = false
		if base.Host != nil && stat.Host != nil {
			psiDeltaMetrics(base.Host, stat.Host, elapsedMicroseconds, p, "", suffix)
		}
		if base.Arc != nil && stat.Arc != nil {
			psiDeltaMetrics(base.Arc, stat.Arc, elapsedMicroseconds, p, "", suffix)
		}
	}

	if stat.Host != nil {
		psiDetailMetrics(psiSomeTag, "", suffix, &(stat.Host.Some), p, includeTotalSinceBoot)
		psiDetailMetrics(psiFullTag, "", suffix, &(stat.Host.Full), p, includeTotalSinceBoot)
	}
	if stat.Arc != nil {
		psiDetailMetrics(psiSomeTag, "arc_", suffix, &(stat.Arc.Some), p, includeTotalSinceBoot)
		psiDetailMetrics(psiFullTag, "arc_", suffix, &(stat.Arc.Full), p, includeTotalSinceBoot)
	}

	return nil
}

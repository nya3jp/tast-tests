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
	"chromiumos/tast/errors"
)

const psiFilename = "/proc/pressure/memory"
const psiLineFormat = " avg10=%f avg60=%f avg300=%f total=%d"
const psiNItems = 4
const psiSomeTag = "some"
const psiFullTag = "full"

// PSIDetail holds one line of statistics from memory pressure dumps,
type PSIDetail struct {
	Avg10, Avg60, Avg300 float64
	Total                uint64
}

// PSIStats holds statistics from memory pressure dumps
type PSIStats struct {
	Some      PSIDetail
	Full      PSIDetail
	Timestamp time.Time
}

// NewPSIStats parses /proc/pressure/memory to create a PSIStats.
func NewPSIStats() (*PSIStats, error) {
	statBlob, err := ioutil.ReadFile(psiFilename)
	if err != nil {
		// this must be a kernel that has NO psi - not an error
		return nil, nil
	}
	stats := &PSIStats{Timestamp: time.Now()}
	blocks := []struct {
		tag  string
		data *PSIDetail
	}{
		{psiSomeTag, &(stats.Some)},
		{psiFullTag, &(stats.Full)},
	}
	lines := strings.SplitN(string(statBlob), "\n", len(blocks))
	if len(lines) != len(blocks) {
		return nil, errors.Wrapf(err, "PSI metrics file should have %d lines, found %d", len(blocks), len(lines))
	}
	for i, line := range lines {
		tag := blocks[i].tag
		d := blocks[i].data
		if nitems, err := fmt.Sscanf(line, tag+psiLineFormat, &(d.Avg10), &(d.Avg60), &(d.Avg300), &(d.Total)); nitems != psiNItems {
			return nil, errors.Wrapf(err, "found %d PSI fields, expected %d", nitems, psiNItems)
		}
	}
	return stats, nil
}

func psiDetailMetrics(tag, suffix string, detail *PSIDetail, p *perf.Values, includeTotal bool) {
	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("psi_%s_avg10%s", tag, suffix),
			Unit:      "Percentage",
			Direction: perf.SmallerIsBetter,
		},
		detail.Avg10,
	)
	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("psi_%s_avg60%s", tag, suffix),
			Unit:      "Percentage",
			Direction: perf.SmallerIsBetter,
		},
		detail.Avg60,
	)
	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("psi_%s_avg300%s", tag, suffix),
			Unit:      "Percentage",
			Direction: perf.SmallerIsBetter,
		},
		detail.Avg300,
	)

	if includeTotal {
		p.Set(
			perf.Metric{
				Name:      fmt.Sprintf("psi_%s_total%s", tag, suffix),
				Unit:      "Microseconds",
				Direction: perf.SmallerIsBetter,
			},
			float64(detail.Total),
		)
	}
}

// PSIMetrics writes a JSON file containing statistics from PSI metrics
// base is optional: if set, defines the starting point for metrics; if nil, metrics are avg since boot
// If outdir is "", then no logs are written.
func PSIMetrics(ctx context.Context, base *PSIStats, p *perf.Values, outdir, suffix string) error {
	stat, err := NewPSIStats()
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
		// No perf.Values, so exit without computing the metrics.
		return nil
	}

	includeTotalSinceBoot := true
	if base != nil {
		elapsedMicroseconds := stat.Timestamp.Sub(base.Timestamp).Microseconds()

		// we will log blocked times during custom interval, so no need for these,
		// which are less useful
		includeTotalSinceBoot = false

		// ignore inverted timings, which would generate noise.
		// inversion is the result of incorrect calling or
		// (rare but normal) total counter wrap-arounds
		if elapsedMicroseconds > 0 {
			if stat.Some.Total >= base.Some.Total {
				diff := float64(stat.Some.Total - base.Some.Total)
				rate := diff / float64(elapsedMicroseconds)
				rate *= 100.0
				p.Set(
					perf.Metric{
						Name:      fmt.Sprintf("psi_some_custom%s", suffix),
						Unit:      "Percentage",
						Direction: perf.SmallerIsBetter,
					},
					rate,
				)
				p.Set(
					perf.Metric{
						Name:      fmt.Sprintf("psi_some_delta%s", suffix),
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
						Name:      fmt.Sprintf("psi_full_custom%s", suffix),
						Unit:      "Percentage",
						Direction: perf.SmallerIsBetter,
					},
					rate,
				)
				p.Set(
					perf.Metric{
						Name:      fmt.Sprintf("psi_full_delta%s", suffix),
						Unit:      "Microseconds",
						Direction: perf.SmallerIsBetter,
					},
					diff,
				)
			}
		}
	}

	psiDetailMetrics(psiSomeTag, suffix, &(stat.Some), p, includeTotalSinceBoot)
	psiDetailMetrics(psiFullTag, suffix, &(stat.Full), p, includeTotalSinceBoot)

	return nil
}

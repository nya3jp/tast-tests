// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cuj has utilities for CUJ-style UI performance tests.
package cuj

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/perf"
)

// MetricCategory is the category for the metrics tracked by Recorder.
type MetricCategory string

const (
	// CategorySmoothness is the category of animation smoothness metrics.
	CategorySmoothness MetricCategory = "AnimationSmoothness"
	// CategoryLatency is the category of input latency metrics.
	CategoryLatency MetricCategory = "InputLatency"
)

// Direction returns the perf.Direction for the category.
func (c MetricCategory) Direction() perf.Direction {
	if c == CategorySmoothness {
		return perf.BiggerIsBetter
	}
	return perf.SmallerIsBetter
}

// Smoothness less than 50% (i.e. 30fps) will be considered to be janky. 20%
// (i.e. 12fps) will be considered to be extremely janky.
var smoothnessJankCriteria = []int64{50, 20}

// Latency longer than 100 msecs will be considered to be janky. 250 msecs will
// be considered to be extremely janky.
var latencyJankCriteria = []int64{100, 250}

// MetricConfig is the configuration for the recorder.
type MetricConfig struct {
	// The name of the histogram to be recorded.
	HistogramName string

	// The category of the histogram.
	Category MetricCategory

	// The unit of the histogram, like "percent" or "ms".
	Unit string

	// The criteria to be considered jank, used to aggregated rate of janky
	// instances. This can be empty, in that case the defualt criteria will be
	// used.
	JankCriteria []int64
}

type record struct {
	config     MetricConfig
	totalCount int64
	jankCounts [2]float64
	sum        int64
}

// Recorder is a utility to measure various metrics for CUJ-style tests.
type Recorder struct {
	names   []string
	records map[string]*record
}

func getJankCounts(hist *metrics.Histogram, direction perf.Direction, criteria int64) float64 {
	var count float64
	if direction == perf.BiggerIsBetter {
		for _, bucket := range hist.Buckets {
			if bucket.Max < criteria {
				count += float64(bucket.Count)
			} else if bucket.Min <= criteria {
				// Estimate the count with assuming uniform distribution.
				count += float64(bucket.Count) * float64(criteria-bucket.Min) / float64(bucket.Max-bucket.Min)
			}
		}
	} else {
		for _, bucket := range hist.Buckets {
			if bucket.Min > criteria {
				count += float64(bucket.Count)
			} else if bucket.Max > criteria {
				count += float64(bucket.Count) * float64(bucket.Max-criteria) / float64(bucket.Max-bucket.Min)
			}
		}
	}
	return count
}

// NewRecorder creates a Recorder based on the configs. It also aggregates the
// metrics of each category (animation smoothness and input latency) and creates
// the aggregated reports.
func NewRecorder(configs ...MetricConfig) (*Recorder, error) {
	// TODO(mukai): also introduce memory collector and power data collector.
	r := &Recorder{
		names:   make([]string, 0, len(configs)),
		records: make(map[string]*record, len(configs)+2)}
	for _, config := range configs {
		if config.HistogramName == string(CategoryLatency) || config.HistogramName == string(CategorySmoothness) {
			return nil, errors.Errorf("invalid histogram name: %s", config.HistogramName)
		}
		r.names = append(r.names, config.HistogramName)
		r.records[config.HistogramName] = &record{config: config}
		// Use the default criteria if JankCriteria is not specified explicitly.
		if len(config.JankCriteria) == 0 {
			switch config.Category {
			case CategorySmoothness:
				copy(r.records[config.HistogramName].config.JankCriteria, smoothnessJankCriteria)
			case CategoryLatency:
				copy(r.records[config.HistogramName].config.JankCriteria, latencyJankCriteria)
			default:
				return nil, errors.Errorf("unsupported category: %v", config.Category)
			}
		} else if len(config.JankCriteria) != 2 {
			return nil, errors.Errorf("jank criteria for %s has %d element, it must be exactly 2", config.HistogramName, len(config.JankCriteria))
		}
	}
	r.records[string(CategoryLatency)] = &record{config: MetricConfig{
		HistogramName: string(CategoryLatency),
		Unit:          "ms",
		Category:      CategoryLatency,
	}}
	r.records[string(CategorySmoothness)] = &record{config: MetricConfig{
		HistogramName: string(CategorySmoothness),
		Unit:          "percent",
		Category:      CategorySmoothness,
	}}
	return r, nil
}

// Run conducts the test scenario f, and collects the related metrics for the
// test scenario, and updates the internal data.
func (r *Recorder) Run(ctx context.Context, cr *chrome.Chrome, f func() error) error {
	// TODO(mukai): takes care of memory collector.
	hists, err := metrics.Run(ctx, cr, f, r.names...)
	if err != nil {
		return err
	}
	for _, hist := range hists {
		if hist.TotalCount() == 0 {
			continue
		}
		record := r.records[hist.Name]
		record.totalCount += hist.TotalCount()
		record.sum += hist.Sum
		jankCounts := []float64{
			getJankCounts(hist, record.config.Category.Direction(), record.config.JankCriteria[0]),
			getJankCounts(hist, record.config.Category.Direction(), record.config.JankCriteria[1]),
		}
		record.jankCounts[0] += jankCounts[0]
		record.jankCounts[1] += jankCounts[1]
		totalRecord := r.records[string(record.config.Category)]
		totalRecord.totalCount += hist.TotalCount()
		totalRecord.sum += hist.Sum
		totalRecord.jankCounts[0] += jankCounts[0]
		totalRecord.jankCounts[1] += jankCounts[1]
	}
	return nil
}

// Record creates the reporting values from the currently stored data points and
// sets the values into pv.
func (r *Recorder) Record(pv *perf.Values) error {
	if r.records[string(CategoryLatency)].totalCount == 0 && r.records[string(CategorySmoothness)].totalCount == 0 {
		return errors.New("no data points for both latency and smoothness")
	}

	for name, record := range r.records {
		if record.totalCount == 0 {
			continue
		}
		pv.Set(perf.Metric{
			Name:      name,
			Unit:      record.config.Unit,
			Variant:   "average",
			Direction: record.config.Category.Direction(),
		}, float64(record.sum)/float64(record.totalCount))
		pv.Set(perf.Metric{
			Name:      name,
			Unit:      "percent",
			Variant:   "jank_rate",
			Direction: perf.SmallerIsBetter,
		}, record.jankCounts[0]/float64(record.totalCount)*100)
		pv.Set(perf.Metric{
			Name:      name,
			Unit:      "percent",
			Variant:   "very_jank_rate",
			Direction: perf.SmallerIsBetter,
		}, record.jankCounts[1]/float64(record.totalCount)*100)
	}
	return nil
}

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
	CategoryLatency = "InputLatency"
)

// Direction returns the perf.Direction for the category.
func (c MetricCategory) Direction() perf.Direction {
	if c == CategorySmoothness {
		return perf.BiggerIsBetter
	}
	return perf.SmallerIsBetter
}

// Smoothness less than 50% (i.e. 30fps) will be considered a jank.
var smoothnessJankCriteria = []int64{50, 20}

// Latency longer than 500 msecs will be considered a jank.
var latencyJankCriteria = []int64{100, 250}

// MetricConfig is the configuration for the recorder.
type MetricConfig struct {
	Name         string
	Category     MetricCategory
	Unit         string
	JankCriteria []int64
}

type record struct {
	totalCount int64
	jankCounts [2]float64
	sum        int64
}

// Recorder is a utility to measure various metrics for CUJ-style tests.
type Recorder struct {
	names   []string
	configs map[string]MetricConfig
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

// NewRecorder creates a Recorder based on the configs.
func NewRecorder(configs ...MetricConfig) (*Recorder, error) {
	// TODO(mukai): also introduce memory collector and power data collector.
	r := &Recorder{
		names:   make([]string, 0, len(configs)),
		configs: make(map[string]MetricConfig, len(configs)),
		records: make(map[string]*record, len(configs))}
	for _, config := range configs {
		if config.Name == string(CategoryLatency) || config.Name == string(CategorySmoothness) {
			return nil, errors.Errorf("invalid histogram name: %s", config.Name)
		}
		r.names = append(r.names, config.Name)
		r.records[config.Name] = &record{}
		r.configs[config.Name] = config
		if len(config.JankCriteria) != 2 {
			switch config.Category {
			case CategorySmoothness:
				copy(r.configs[config.Name].JankCriteria, smoothnessJankCriteria)
			case CategoryLatency:
				copy(r.configs[config.Name].JankCriteria, latencyJankCriteria)
			default:
				return nil, errors.Errorf("unsupported category: %v", config.Category)
			}
		}
	}
	r.configs[string(CategoryLatency)] = MetricConfig{
		Name:     string(CategoryLatency),
		Unit:     "ms",
		Category: CategoryLatency,
	}
	r.configs[string(CategorySmoothness)] = MetricConfig{
		Name:     string(CategorySmoothness),
		Unit:     "percent",
		Category: CategorySmoothness,
	}
	r.records[string(CategoryLatency)] = &record{}
	r.records[string(CategorySmoothness)] = &record{}
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
		config := r.configs[hist.Name]
		jankCounts := []float64{
			getJankCounts(hist, config.Category.Direction(), config.JankCriteria[0]),
			getJankCounts(hist, config.Category.Direction(), config.JankCriteria[1]),
		}
		record.jankCounts[0] += jankCounts[0]
		record.jankCounts[1] += jankCounts[1]
		totalRecord := r.records[string(config.Category)]
		totalRecord.totalCount += hist.TotalCount()
		totalRecord.sum += hist.Sum
		totalRecord.jankCounts[0] += jankCounts[0]
		totalRecord.jankCounts[1] += jankCounts[1]
	}
	return nil
}

// Record creates the reporting values for perf based on the currently stored
// data recorded through Run invocations.
func (r *Recorder) Record(outDir string) error {
	if r.records[string(CategoryLatency)].totalCount == 0 && r.records[string(CategorySmoothness)].totalCount == 0 {
		return errors.New("no data points for both latency and smoothness")
	}

	pv := perf.NewValues()
	for name, record := range r.records {
		if record.totalCount == 0 {
			continue
		}
		config := r.configs[name]
		pv.Set(perf.Metric{
			Name:      name,
			Unit:      config.Unit,
			Variant:   "average",
			Direction: config.Category.Direction(),
		}, float64(record.sum)/float64(record.totalCount))
		pv.Set(perf.Metric{
			Name:      name,
			Unit:      "percent",
			Variant:   "jank_rate",
			Direction: perf.SmallerIsBetter,
		}, record.jankCounts[0]/float64(record.totalCount))
		pv.Set(perf.Metric{
			Name:      name,
			Unit:      "percent",
			Variant:   "very_jank_rate",
			Direction: perf.SmallerIsBetter,
		}, record.jankCounts[1]/float64(record.totalCount))
	}
	return pv.Save(outDir)
}

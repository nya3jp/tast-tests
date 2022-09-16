// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package perfutil provides utilities of storing performance data for UI tests.
package perfutil

import (
	"context"
	"sort"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Values keeps the reporting values for multiple runs.
type Values struct {
	metrics map[string]perf.Metric
	values  map[string][]float64
}

// NewValues creates a new Values instance.
func NewValues() *Values {
	return &Values{
		metrics: map[string]perf.Metric{},
		values:  map[string][]float64{},
	}
}

// Append adds a new data points to values.
func (v *Values) Append(metric perf.Metric, value float64) {
	name := metric.Name
	if _, ok := v.metrics[name]; !ok {
		v.metrics[name] = metric
	}
	v.values[name] = append(v.values[name], value)
}

// MergeWithSuffix merges all data points of vs into this Values structure
// optionally adding suffix to the value name.
func (v *Values) MergeWithSuffix(suffix string, newValues map[perf.Metric][]float64) {
	for metric, vs := range newValues {
		if len(vs) == 0 {
			continue
		}
		suffixedK := metric
		suffixedK.Name += suffix
		for _, val := range vs {
			v.Append(suffixedK, val)
		}
	}
}

func minMaxIndices(vs []float64) (minIndex, maxIndex int) {
	max := vs[0]
	min := vs[0]
	for i, v := range vs {
		if v > max {
			max = v
			maxIndex = i
		}
		if v < min {
			min = v
			minIndex = i
		}
	}
	return minIndex, maxIndex
}

// Values creates a new perf.Values for its data points.
func (v *Values) Values(ctx context.Context) *perf.Values {
	pv := perf.NewValues()

	// Ensure that the iteration order is sorted by the name of the metrics, as
	// this iteration also logs the name/results.
	names := make([]string, 0, len(v.metrics))
	for name := range v.metrics {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		metric := v.metrics[name]
		vs := v.values[name]
		if len(vs) == 0 {
			continue
		}
		// Report the first value
		firstMetric := metric
		firstMetric.Variant = "first"
		pv.Set(firstMetric, vs[0])
		// Average metrics; just reporting individual values, as crosbolt makes
		// the average calculation.
		otherMetric := metric
		otherMetric.Variant = "average"
		otherMetric.Multiple = true

		minIndex, maxIndex := minMaxIndices(vs)
		var sum float64
		var count int
		for i, v := range vs {
			if len(vs) < 3 || (i != maxIndex && i != minIndex) {
				pv.Append(otherMetric, v)
				sum += v
				count++
			}
		}
		testing.ContextLogf(ctx, "Average %s = %v", name, sum/float64(count))
	}
	return pv
}

// Verify verifies the stored values with the numbers in expects and returns a
// list of misses of the expectations. If no problem, it will return empty.
func (v *Values) Verify(ctx context.Context, expects map[string]float64) []error {
	var errs []error
	for name, metric := range v.metrics {
		vs := v.values[name]
		if len(vs) == 0 {
			continue
		}
		exp, ok := expects[name]
		if !ok {
			testing.ContextLogf(ctx, "Skipping %s", name)
			continue
		}
		var sum float64
		var count int
		if len(vs) < 3 {
			count = len(vs)
			for _, v := range vs {
				sum += v
			}
		} else {
			minIndex, maxIndex := minMaxIndices(vs)
			for i, v := range vs {
				if i != minIndex && i != maxIndex {
					sum += v
					count++
				}
			}
		}
		avg := sum / float64(count)
		var isGood bool
		var operator string
		if metric.Direction == perf.BiggerIsBetter {
			isGood = exp <= avg
			operator = ">="
		} else {
			isGood = avg <= exp
			operator = "<="
		}
		if !isGood {
			errs = append(errs, errors.Errorf("%s: got %v want %s%v", name, avg, operator, exp))
		}
	}
	return errs
}

// Save is a shortcut of Values().Save(outdir). Helpful when the test does not
// have to combine with other data points.
func (v *Values) Save(ctx context.Context, outdir string) error {
	return v.Values(ctx).Save(outdir)
}

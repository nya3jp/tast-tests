// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package perfutil provides utilities of storing performance data for UI tests.
package perfutil

import (
	"chromiumos/tast/common/perf"
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

// Values creates a new perf.Values for its data points.
func (v *Values) Values() *perf.Values {
	pv := perf.NewValues()

	for name, metric := range v.metrics {
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

		max := vs[0]
		min := vs[0]
		maxIndex := 0
		minIndex := 0
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
		for i, v := range vs {
			if len(vs) < 3 || (i != maxIndex && i != minIndex) {
				pv.Append(otherMetric, v)
			}
		}
	}
	return pv
}

// Save is a shortcut of Values().Save(outdir). Helpful when the test does not
// have to combine with other data points.
func (v *Values) Save(outdir string) error {
	return v.Values().Save(outdir)
}

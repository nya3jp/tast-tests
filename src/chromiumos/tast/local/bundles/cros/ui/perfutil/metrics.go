// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perfutil

import (
	"context"
	"strings"

	"chromiumos/tast/testing"
)

type metricType int

const (
	metricSmoothness metricType = iota
	metricLatency
)

// estimateMetricType checks the name of a histogram and returns type of the
// histogram (i.e. smoothness or latency).
func estimateMetricType(ctx context.Context, histname string) metricType {
	// PresentationTime is latency metric.
	if strings.Contains(histname, "PresentationTime") {
		return metricLatency
	}
	// Measuring duration is latency metric (e.g. Ash.InteractiveWindowResize.TimeToPresent).
	if strings.Contains(histname, ".TimeTo") {
		return metricLatency
	}
	// Having smoothness is smoothness metric.
	if strings.Contains(histname, "Smoothness") {
		return metricSmoothness
	}
	// Otherwise, we're not sure. Assuming unknown pattern of animation smoothness.
	testing.ContextLogf(ctx, "Can't decide histogram %s is either of smoothness or latency metric. Assuming smoothness metric", histname)
	return metricSmoothness
}

// CreateExpectations creates the map of expected values for the given histogram
// names. It estimates the type of histogram (either of smoothness or latency)
// from the name.
func CreateExpectations(ctx context.Context, histnames ...string) map[string]float64 {
	const (
		// By default, smoothness metrics should be never lower than 30% (i.e. 18FPS).
		smoothnessExpectation = 30
		// By default, latency metrics should be never bigger than 150msecs.
		latencyExpectation = 150
	)
	result := make(map[string]float64, len(histnames))
	for _, histname := range histnames {
		switch estimateMetricType(ctx, histname) {
		case metricSmoothness:
			result[histname] = smoothnessExpectation
		case metricLatency:
			result[histname] = latencyExpectation
		}
	}
	return result
}

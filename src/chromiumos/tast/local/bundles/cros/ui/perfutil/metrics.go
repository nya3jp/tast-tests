// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perfutil

import (
	"context"
	"strings"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/testing"
	"chromiumos/tast/local/chrome/metrics"
)

type metricType int

const (
	metricSmoothness metricType = iota
	metricLatency
	metricJank
)

// estimateMetricType checks the name of a histogram and returns type of the
// histogram (i.e. smoothness or latency).
func estimateMetricType(ctx context.Context, histname string) metricType {
	// PresentationTime is latency metric.
	if strings.Contains(histname, "PresentationTime") {
		return metricLatency
	}
	if strings.Contains(histname, "Duration") {
		return metricLatency
	}
	// Jank is the opposite of smoothness, so should be treated as "smaller is better".
	if strings.Contains(histname, ".Jank") {
		return metricJank
	}
	// Measuring duration is latency metric (e.g. Ash.InteractiveWindowResize.TimeToPresent, Ash.LoginAnimation.Duration)
	if strings.Contains(histname, ".TimeTo") || strings.Contains(histname, ".Duration") {
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

// estimateMetricPresenattionType checks the name of a histogram and returns
// presentation parameters (direction, data type)
func estimateMetricPresenattionType(ctx context.Context, histname string) (perf.Direction, string) {
	switch estimateMetricType(ctx, histname) {
	case metricSmoothness:
		return perf.BiggerIsBetter, "percent"
	case metricLatency:
		return perf.SmallerIsBetter, "ms"
	case metricJank:
		return perf.SmallerIsBetter, "percent"
	}
	testing.ContextLogf(ctx, "Can't guess histogram %s presentation type. Assuming 'percent' and 'BiggerIsBetter'", histname)
	return perf.BiggerIsBetter, "percent"
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
		case metricJank:
			continue // No expectation for Junk now.
		}
	}
	return result
}

func HistogramMean(hist *metrics.Histogram) (float64, error) {
  return hist.Mean()
}

func HistogramMax(hist *metrics.Histogram) (float64, error) {
  return hist.Max()
}

// StoreFunc is a function to be used for RunMultiple.
type StoreFunc func(ctx context.Context, pv *Values, hists []*metrics.Histogram) error

type aggregationFunction func(*metrics.Histogram) (float64, error)

type histogramSpecification struct {
  name string
  direction perf.Direction
  unit string
  aggregator aggregationFunction
}

type HistogramSpecifications []histogramSpecification

func (specifications* HistogramSpecifications)Names() []string {
  val :=[]string{}
  for _, spec := range *specifications {
    val = append(val, spec.name)
  }
  return val
}

func HistogramSpecification(name string, direction perf.Direction, unit string, aggregator aggregationFunction) histogramSpecification {
  return histogramSpecification{name, direction, unit, aggregator};
}

func HistogramSpecificationWithHeuristics(ctx context.Context, histname string) histogramSpecification {
  direction, unit := estimateMetricPresenattionType(ctx, histname)
  aggregator := func(histogram *metrics.Histogram) (float64, error) { return histogram.Mean() }
  return histogramSpecification{histname, direction, unit, aggregator}
}

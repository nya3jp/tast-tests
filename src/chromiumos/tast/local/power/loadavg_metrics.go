// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"io/ioutil"
	"strconv"
	"strings"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
)

// LoadAvgMetrics records load averages.
type LoadAvgMetrics struct {
	metrics map[string]perf.Metric
}

// Assert that LoadAvgMetrics can be used in perf.Timeline.
var _ perf.TimelineDatasource = &LoadAvgMetrics{}

// NewLoadAvgMetrics creates a timeline metric to collect load averages.
func NewLoadAvgMetrics() *LoadAvgMetrics {
	return &LoadAvgMetrics{make(map[string]perf.Metric)}
}

// Setup does nothing.
func (cs *LoadAvgMetrics) Setup(ctx context.Context, prefix string) error {
	return nil
}

// Start initalizes metrics.
func (cs *LoadAvgMetrics) Start(ctx context.Context) error {

	cs.metrics["loadavg_1min"] = perf.Metric{Name: "loadavg_1min", Unit: "process",
		Direction: perf.SmallerIsBetter, Multiple: true}
	cs.metrics["loadavg_5min"] = perf.Metric{Name: "loadavg_5min", Unit: "process",
		Direction: perf.SmallerIsBetter, Multiple: true}
	cs.metrics["loadavg_15min"] = perf.Metric{Name: "loadavg_15min", Unit: "process",
		Direction: perf.SmallerIsBetter, Multiple: true}
	cs.metrics["runnables"] = perf.Metric{Name: "runnables", Unit: "process",
		Direction: perf.SmallerIsBetter, Multiple: true}
	cs.metrics["entities"] = perf.Metric{Name: "entities", Unit: "process",
		Direction: perf.SmallerIsBetter, Multiple: true}
	return nil
}

// Snapshot gets the loadavg values.
func (cs *LoadAvgMetrics) Snapshot(ctx context.Context, values *perf.Values) error {

	content, err := ioutil.ReadFile("/proc/loadavg")
	if err != nil {
		return errors.Wrap(err, "failed to read /proc/loadavg")
	}

	loadavgValues := strings.Split(string(content), " ")

	if len(loadavgValues) != 5 {
		return errors.Wrap(err, "unexpected format of /proc/loadavg")
	}

	if f, err := strconv.ParseFloat(loadavgValues[0], 64); err == nil {
		values.Append(cs.metrics["loadavg_1min"], f)
	} else {
		return errors.Wrap(err, "unexpected format of /proc/loadavg")
	}

	if f, err := strconv.ParseFloat(loadavgValues[1], 64); err == nil {
		values.Append(cs.metrics["loadavg_5min"], f)
	} else {
		return errors.Wrap(err, "unexpected format of /proc/loadavg")
	}

	if f, err := strconv.ParseFloat(loadavgValues[2], 64); err == nil {
		values.Append(cs.metrics["loadavg_15min"], f)
	} else {
		return errors.Wrap(err, "unexpected format of /proc/loadavg")
	}

	runnablesEntities := strings.Split(loadavgValues[3], "/")

	if len(runnablesEntities) != 2 {
		return errors.Wrap(err, "unexpected format of /proc/loadavg")
	}

	if f, err := strconv.ParseFloat(runnablesEntities[0], 64); err == nil {
		values.Append(cs.metrics["runnables"], f)
	} else {
		return errors.Wrap(err, "unexpected format of /proc/loadavg")
	}

	if f, err := strconv.ParseFloat(runnablesEntities[1], 64); err == nil {
		values.Append(cs.metrics["entities"], f)
	} else {
		return errors.Wrap(err, "unexpected format of /proc/loadavg")
	}

	return nil
}

// Stop does nothing.
func (cs *LoadAvgMetrics) Stop(ctx context.Context, values *perf.Values) error {
	return nil
}

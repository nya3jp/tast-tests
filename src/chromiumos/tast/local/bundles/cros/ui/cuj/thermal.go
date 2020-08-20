// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

// CPU's temperature > 75C is considered to be thermal throttling.
const throttlingTemp = 75

type thermalMetric struct {
	paths  []string
	metric perf.Metric
}

type thermalDataSource struct {
	metrics          map[string]*thermalMetric
	throttlingStatus perf.Metric
}

func newThermalDataSource(ctx context.Context) *thermalDataSource {
	return &thermalDataSource{}
}

// Setup implements perf.TimelineDatasource.Setup.
func (ds *thermalDataSource) Setup(ctx context.Context, prefix string) error {
	thermalSensors, err := power.ListSysfsThermalSensors(ctx)
	if err != nil {
		return err
	}
	if len(thermalSensors) == 0 {
		testing.ContextLog(ctx, "No thermal sensors found")
		return nil
	}

	// Only CPU thermal data is cared currently.
	namePatterns := []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{"CPU", regexp.MustCompile("x86_pkg_temp")}, // Intel devices
		{"CPU", regexp.MustCompile("B0D4")},         // Intel devices
		{"CPU", regexp.MustCompile("acpitz")},       // AMD devices
		{"CPU", regexp.MustCompile("[cC][pP][uU]")}, // several arm devices, typically named cpu_thermal
	}

	ds.metrics = map[string]*thermalMetric{}
	for name, path := range thermalSensors {
		var groupName string
		for _, data := range namePatterns {
			if data.pattern.MatchString(name) {
				groupName = data.name
				break
			}
		}
		if groupName == "" {
			testing.ContextLogf(ctx, "Name %s (path %s) does not match to known patterns, skipping", name, path)
			continue
		}
		testing.ContextLogf(ctx, "Group: %s, Name: %q Path: %s", groupName, name, path)
		metric, ok := ds.metrics[groupName]
		if !ok {
			metric = &thermalMetric{metric: perf.Metric{
				Name:      prefix + "Thermal." + groupName,
				Unit:      "deg_C",
				Direction: perf.SmallerIsBetter,
				Multiple:  true}}
			ds.metrics[groupName] = metric
		}
		metric.paths = append(metric.paths, filepath.Join(path, "temp"))
	}

	// Metrics for CPU's thermal throttling status.
	ds.throttlingStatus = perf.Metric{
		Name:      prefix + "Thermal." + "ThrottlingStatus",
		Unit:      "Boolean",
		Direction: perf.SmallerIsBetter,
		Multiple:  true}

	return nil
}

// Start implements perf.TimelineDatasource.Start.
func (ds *thermalDataSource) Start(ctx context.Context) error {
	return nil
}

// Snapshot implements perf.TimelineDatasource.Snapshot.
func (ds *thermalDataSource) Snapshot(ctx context.Context, values *perf.Values) error {
	for _, metric := range ds.metrics {
		var sum float64
		for _, path := range metric.paths {
			bs, err := ioutil.ReadFile(path)
			if err != nil {
				return errors.Wrapf(err, "failed to read %s", path)
			}
			temp, err := strconv.ParseFloat(strings.TrimSpace(string(bs)), 64)
			if err != nil {
				return errors.Wrapf(err, "failed to parse data %s", string(bs))
			}
			sum += temp
		}
		temp := sum / 1000 / float64(len(metric.paths))
		var throttled float64
		if temp > throttlingTemp {
			throttled = 1
		} else {
			throttled = 0
		}
		values.Append(metric.metric, temp)
		values.Append(ds.throttlingStatus, throttled)
	}
	return nil
}

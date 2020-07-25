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

type metricData struct {
	paths  []string
	metric perf.Metric
}

type thermalDataSource struct {
	metrics map[string]*metricData
}

func newThermalDataSource(ctx context.Context) *thermalDataSource {
	return &ThermalDataSource{}
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

	namePatterns := []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{"cpu", regexp.MustCompile("[cC][pP][uU]")},
		{"cpu", regexp.MustCompile("x86")},
		{"gpu", regexp.MustCompile("[gG][pP][uU]")},
	}

	ds.metrics = map[string]*metricData{}
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
			metric = &metricData{metric: perf.Metric{
				Name:      prefix + groupName,
				Unit:      "deg_C",
				Direction: perf.SmallerIsBetter,
				Multiple:  true}}
			ds.metrics[groupName] = metric
		}
		metric.paths = append(metric.paths, filepath.Join(path, "temp"))
	}
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
		values.Append(metric.metric, sum/1000/float64(len(metric.paths)))
	}
	return nil
}

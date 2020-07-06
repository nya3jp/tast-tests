// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"io/ioutil"
	"path"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// listSysfsThermalSensors lists names and paths of thermal sensors which can be read through sysfs.
func listSysfsThermalSensors(ctx context.Context) (map[string]string, error) {
	// TODO(springerm): Remove ContextLogf()s after checking this function works on all platforms
	thermalSensors := make(map[string]string)
	const sysfsThermalPath = "/sys/class/thermal"
	testing.ContextLog(ctx, "Listing thermal sensors in ", sysfsThermalPath)
	files, err := ioutil.ReadDir(sysfsThermalPath)
	if err != nil {
		return thermalSensors, errors.Wrap(err, "failed to read sysfs dir")
	}
	// Avoid duplicate metric names by adding a counter suffix if necessary.
	typeCounter := make(map[string]int)
	for _, file := range files {
		if !strings.HasPrefix(file.Name(), "thermal_zone") {
			testing.ContextLogf(ctx, "%v is not a thermal sensor", file.Name())
			continue
		}

		devPath := path.Join(sysfsThermalPath, file.Name())
		_, err := readInt64(path.Join(devPath, "temp"))
		if err != nil {
			testing.ContextLogf(ctx, "%v is not readable", devPath)
			continue
		}

		name, err := readLine(path.Join(devPath, "type"))
		if err != nil {
			testing.ContextLogf(ctx, "%v is not readable", devPath)
			continue
		}

		typeCounter[name] = typeCounter[name] + 1
		if typeCounter[name] > 1 {
			name = name + "_" + strconv.Itoa(typeCounter[name])
		}

		thermalSensors[name] = devPath
	}
	return thermalSensors, nil
}

// ThermalMetric holds the name, sysfs path and perf.Metric object of a thermal sensor.
type ThermalMetric struct {
	name   string
	path   string
	metric perf.Metric
}

// SysfsThermalMetrics holds the metrics to read from sysfs.
type SysfsThermalMetrics struct {
	metrics []ThermalMetric
}

// Assert that SysfsThermalMetrics can be used in perf.Timeline.
var _ perf.TimelineDatasource = &SysfsThermalMetrics{}

// NewSysfsThermalMetrics creates a struct to capture thermal metrics with sysfs.
func NewSysfsThermalMetrics() *SysfsThermalMetrics {
	return &SysfsThermalMetrics{}
}

// Setup checks which thermal sensors are available.
func (b *SysfsThermalMetrics) Setup(ctx context.Context, prefix string) error {
	b.metrics = []ThermalMetric{}

	thermalSensors, err := listSysfsThermalSensors(ctx)
	if err != nil {
		return err
	}
	if len(thermalSensors) == 0 {
		testing.ContextLog(ctx, "No thermal metrics found")
		return nil
	}
	testing.ContextLogf(ctx, "SysfsThermalMetrics uses %v sensors:", len(thermalSensors))
	for name, path := range thermalSensors {
		testing.ContextLogf(ctx, "%s (%s)", name, path)
		// Some sensor names contain characters that are not allowed in metric names.
		reg := regexp.MustCompile("[^a-zA-Z0-9]+")
		metricName := prefix + reg.ReplaceAllString(name, "_")
		perfMetric := perf.Metric{Name: metricName, Unit: "deg_C", Direction: perf.SmallerIsBetter, Multiple: true}
		thermalMetric := ThermalMetric{name: name, path: path, metric: perfMetric}
		b.metrics = append(b.metrics, thermalMetric)
	}
	return nil
}

// Start is not required for SysfsThermalMetrics.
func (b *SysfsThermalMetrics) Start(ctx context.Context) error {
	return nil
}

// Snapshot takes a snapshot of thermal metrics.
// If there are no thermal sensors, Snapshot does nothing and returns without error.
func (b *SysfsThermalMetrics) Snapshot(ctx context.Context, values *perf.Values) error {
	if len(b.metrics) == 0 {
		return nil
	}
	for _, metric := range b.metrics {
		tempFile := path.Join(metric.path, "temp")
		temp, err := readInt64(tempFile)
		if err != nil {
			return errors.Wrapf(err, "cannot read temperature from %s", tempFile)
		}
		values.Append(metric.metric, float64(temp)/1000)
	}

	return nil
}

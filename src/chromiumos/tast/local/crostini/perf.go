// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
)

// RestartStages maps Chrome UMA metrics to corresponding crosbolt metrics.
var RestartStages = map[string]string{
	"Crostini.RestarterTimeInState2.Start":              "Start",
	"Crostini.RestarterTimeInState2.InstallImageLoader": "InstallImageLoader",
	"Crostini.RestarterTimeInState2.CreateDiskImage":    "CreateDiskImage",
	"Crostini.RestarterTimeInState2.StartTerminaVm":     "StartTerminaVm",
	"Crostini.RestarterTimeInState2.StartLxd":           "StartLxd",
	"Crostini.RestarterTimeInState2.CreateContainer":    "CreateContainer",
	"Crostini.RestarterTimeInState2.SetupContainer":     "SetupContainer",
	"Crostini.RestarterTimeInState2.StartContainer":     "StartContainer",
	"Crostini.RestarterTimeInState2.ConfigureContainer": "ConfigureContainer",
}

// Recording holds data for sampling Chrome metrics and emitting corresponding
// perf.Values to a file which can be consumed by crosbolt.
type Recording struct {
	metricBaseName string
	variants       map[string]string
	recorder       *metrics.Recorder
	ctx            context.Context
	tconn          *chrome.TestConn
}

// NewRecording creates a Recording object from a map of Chrome metric to
// perf.Metric variant names, and
func NewRecording(metricBaseName string, variants map[string]string) *Recording {
	return &Recording{metricBaseName: metricBaseName, variants: variants}
}

// Start samples the initial chrome metric values from which to record changes
// using UpdatePerf (below)
func (r *Recording) Start(ctx context.Context, tconn *chrome.TestConn) error {
	names := make([]string, len(r.variants))
	for n := range r.variants {
		names = append(names, n)
	}
	recorder, err := metrics.StartRecorder(ctx, tconn, names...)
	if err != nil {
		return err
	}
	r.recorder = recorder
	r.ctx = ctx
	r.tconn = tconn
	return nil
}

// UpdatePerf records changes in the Recording's Chrome metrics, appending the
// bucket mid-points to |values|, then saves them in outDir when outDir is not
// an empty string.
func (r *Recording) UpdatePerf(values *perf.Values, outDir string) error {
	if r.recorder == nil {
		return errors.New("There is no recorder for perf metrics")
	}
	diffs, err := r.recorder.Histogram(r.ctx, r.tconn)
	if err != nil {
		return err
	}
	for _, hist := range diffs {
		if len(hist.Buckets) > 0 {
			variant, ok := r.variants[hist.Name]
			if !ok {
				continue
			}
			metric := perf.Metric{
				Name:      r.metricBaseName,
				Variant:   variant,
				Unit:      "ms",
				Direction: perf.SmallerIsBetter,
				Multiple:  true,
			}
			for _, bucket := range hist.Buckets {
				mid := float64(bucket.Max+bucket.Min) * 0.5
				for i := 0; i < int(bucket.Count); i++ {
					values.Append(metric, mid)
				}
			}
		}
	}
	if len(outDir) > 0 {
		err = values.Save(outDir)
	}
	return err
}

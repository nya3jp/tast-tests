// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perfutil

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/golang/protobuf/proto"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/testing"
)

// Runs provides the number of iteration for a perftest conducts.
const Runs = 10

// ScenarioFunc is the function to conduct the test operation and returns the
// metric value.
type ScenarioFunc func(context.Context) ([]*metrics.Histogram, error)

// RunAndWaitAll is a utility function to create ScenarioFunc which conducts
// f with metrics.RunAndWaitAll.
func RunAndWaitAll(tconn *chrome.TestConn, f func() error, names ...string) ScenarioFunc {
	return func(ctx context.Context) ([]*metrics.Histogram, error) {
		return metrics.RunAndWaitAll(ctx, tconn, time.Second, f, names...)
	}
}

// StoreFunc is a function to be used for RunMultiple.
type StoreFunc func(ctx context.Context, pv *Values, hists []*metrics.Histogram) error

// StoreAll is a function to store all histograms into values.
func StoreAll(direction perf.Direction, unit, suffix string) StoreFunc {
	return func(ctx context.Context, pv *Values, hists []*metrics.Histogram) error {
		for _, hist := range hists {
			mean, err := hist.Mean()
			if err != nil {
				return errors.Wrapf(err, "failed to get mean for histogram %s", hist.Name)
			}
			name := hist.Name
			if suffix != "" {
				name = name + "." + suffix
			}
			testing.ContextLog(ctx, name, " = ", mean)

			pv.Append(perf.Metric{
				Name:      name,
				Unit:      unit,
				Direction: direction,
			}, mean)
		}
		return nil
	}
}

// StoreSmoothness is a utility function to store animation smoothness metrics.
func StoreSmoothness(ctx context.Context, pv *Values, hists []*metrics.Histogram) error {
	return StoreAll(perf.BiggerIsBetter, "percent", "")(ctx, pv, hists)
}

// StoreLatency is a utility function to store input-latency metrics.
func StoreLatency(ctx context.Context, pv *Values, hists []*metrics.Histogram) error {
	return StoreAll(perf.SmallerIsBetter, "ms", "")(ctx, pv, hists)
}

// Runner is an entity to manage multiple runs of the test scenario.
type Runner struct {
	cr *chrome.Chrome
	pv *Values
}

// NewRunner creates a new instance of Runner.
func NewRunner(cr *chrome.Chrome) *Runner {
	return &Runner{cr: cr, pv: NewValues()}
}

// Values returns the values in the runner.
func (r *Runner) Values() *Values {
	return r.pv
}

// RunMultiple runs scenario multiple times and store the data through store function.
// At the end of the runs, it invokes scenario once more with recording the trace.
func (r *Runner) RunMultiple(ctx context.Context, s *testing.State, name string, scenario ScenarioFunc, store StoreFunc) {
	runPrefix := name
	if name == "" {
		runPrefix = "run"
	}
	for i := 0; i < Runs; i++ {
		if !s.Run(ctx, fmt.Sprintf("%s-%d", runPrefix, i), func(ctx context.Context, s *testing.State) {
			hists, err := scenario(ctx)
			if err != nil {
				s.Fatal("Failed to run the test scenario: ", err)
			}
			if err = store(ctx, r.pv, hists); err != nil {
				s.Fatal("Failed to store the histogram data: ", err)
			}
		}) {
			return
		}
	}
	s.Run(ctx, fmt.Sprintf("%s-tracing", runPrefix), func(ctx context.Context, s *testing.State) {
		sctx, cancel := ctxutil.Shorten(ctx, time.Second)
		defer cancel()
		if err := r.cr.StartTracing(sctx, []string{"benchmark", "cc", "gpu", "input", "toplevel", "ui", "views", "viz"}); err != nil {
			s.Fatal("Failed to start tracing: ", err)
		}
		if _, err := scenario(sctx); err != nil {
			s.Error("Failed to run the test scenario: ", err)
		}
		tr, err := r.cr.StopTracing(ctx)
		if err != nil {
			s.Fatal("Failed to stop tracing: ", err)
		}
		data, err := proto.Marshal(tr)
		if err != nil {
			s.Fatal("Failed to marshal the tracing data: ", err)
		}
		filename := "trace.data"
		if name != "" {
			filename = name + "-" + filename
		}
		if err := ioutil.WriteFile(filepath.Join(s.OutDir(), filename), data, 0644); err != nil {
			s.Fatal("Failed to save the trace file: ", err)
		}
	})
}

// RunMultiple is a utility to create a new runner, conduct runs multiple times,
// and returns the recorded values.
func RunMultiple(ctx context.Context, s *testing.State, cr *chrome.Chrome, scenario func(context.Context) ([]*metrics.Histogram, error), store StoreFunc) *Values {
	r := NewRunner(cr)
	r.RunMultiple(ctx, s, "", scenario, store)
	return r.Values()
}

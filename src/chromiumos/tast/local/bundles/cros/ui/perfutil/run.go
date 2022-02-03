// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perfutil

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/testing"
)

// DefaultRuns provides the default number of iteration for a perftest conducts.
const DefaultRuns = 10

// ScenarioFunc is the function to conduct the test operation and returns the
// metric value.
type ScenarioFunc func(context.Context) ([]*metrics.Histogram, error)

// RunAndWaitAll is a utility function to create ScenarioFunc which conducts
// f with metrics.RunAndWaitAll.
func RunAndWaitAll(tconn *chrome.TestConn, f func(ctx context.Context) error, names ...string) ScenarioFunc {
	return func(ctx context.Context) ([]*metrics.Histogram, error) {
		return metrics.RunAndWaitAll(ctx, tconn, time.Minute, f, names...)
	}
}

// RunAndWaitAny is a utility function to create ScenarioFunc which conducts
// f with metrics.RunAndWaitAny.
func RunAndWaitAny(tconn *chrome.TestConn, f func(ctx context.Context) error, names ...string) ScenarioFunc {
	return func(ctx context.Context) ([]*metrics.Histogram, error) {
		return metrics.RunAndWaitAny(ctx, tconn, time.Minute, f, names...)
	}
}

// StoreFunc is a function to be used for RunMultiple.
type StoreFunc func(ctx context.Context, pv *Values, hists []*metrics.Histogram) error

// StoreAllWithHeuristics is a utility function to store all metrics. It
// determines the direction of perf (bigger is better or smaller is better)
// and unit through heuristics from the name of metrics.
func StoreAllWithHeuristics(suffix string) StoreFunc {
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
			direction, unit := estimateMetricPresenattionType(ctx, name)
			pv.Append(perf.Metric{
				Name:      name,
				Unit:      unit,
				Direction: direction,
			}, mean)
		}
		return nil
	}
}

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
	br         *browser.Browser
	pv         *Values
	Runs       int
	RunTracing bool
}

// NewRunner creates a new instance of Runner.
func NewRunner(br *browser.Browser) *Runner {
	return &Runner{br: br, pv: NewValues(), Runs: DefaultRuns, RunTracing: (br != nil)}
}

// Values returns the values in the runner.
func (r *Runner) Values() *Values {
	return r.pv
}

// RunMultiple runs scenario multiple times and store the data through store
// function. It invokes scenario+store 10 times, and then invokes scenario only
// with tracing enabled. If one of the runs fails, it quits immediately and
// reports an error. The name parameter is used for the prefix of subtest names
// for calling scenario/store function and the prefix for the trace data file.
// The name can be empty, in which case the runner uses default prefix values.
// Returns false when it has an error.
func (r *Runner) RunMultiple(ctx context.Context, s *testing.State, name string, scenario ScenarioFunc, store StoreFunc) bool {
	runPrefix := name
	if name == "" {
		runPrefix = "run"
	}
	for i := 0; i < r.Runs; i++ {
		if !s.Run(ctx, fmt.Sprintf("%s-%d", runPrefix, i), func(ctx context.Context, s *testing.State) {
			hists, err := scenario(ctx)
			if err != nil {
				s.Fatal("Failed to run the test scenario: ", err)
			}
			if err = store(ctx, r.pv, hists); err != nil {
				s.Fatal("Failed to store the histogram data: ", err)
			}
		}) {
			return false
		}
	}
	if !r.RunTracing {
		return true
	}

	const traceCleanupDuration = 2 * time.Second
	if deadline, ok := ctx.Deadline(); ok && deadline.Sub(time.Now()) < traceCleanupDuration {
		testing.ContextLog(ctx, "There are no time to conduct a tracing run. Skipping")
		return true
	}

	defer r.br.StopTracing(ctx)
	return s.Run(ctx, fmt.Sprintf("%s-tracing", runPrefix), func(ctx context.Context, s *testing.State) {
		sctx, cancel := ctxutil.Shorten(ctx, traceCleanupDuration)
		defer cancel()
		// At this time, systrace causes kernel crash on dedede devices. Because of
		// that and data points from systrace isn't actually helpful to most of
		// UI tests, disable systraces for the time being.
		// TODO(https://crbug.com/1162385, b/177636800): enable it.
		if err := r.br.StartTracing(sctx, []string{"benchmark", "cc", "gpu", "input", "toplevel", "ui", "views", "viz"}, browser.DisableSystrace()); err != nil {
			s.Log("Failed to start tracing: ", err)
			return
		}
		if _, err := scenario(sctx); err != nil {
			s.Error("Failed to run the test scenario: ", err)
		}
		tr, err := r.br.StopTracing(ctx)
		if err != nil {
			s.Log("Failed to stop tracing: ", err)
			return
		}
		if tr == nil || len(tr.Packet) == 0 {
			s.Log("No trace data is collected")
			return
		}
		filename := "trace.data.gz"
		if name != "" {
			filename = name + "-" + filename
		}
		if err := chrome.SaveTraceToFile(ctx, tr, filepath.Join(s.OutDir(), filename)); err != nil {
			s.Log("Failed to save trace to file: ", err)
			return
		}
	})
}

// RunMultiple is a utility to create a new runner, conduct runs multiple times,
// and returns the recorded values.
func RunMultiple(ctx context.Context, s *testing.State, br *browser.Browser, scenario ScenarioFunc, store StoreFunc) *Values {
	r := NewRunner(br)
	r.RunMultiple(ctx, s, "", scenario, store)
	return r.Values()
}

// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package perf contains helper functions to ease perfutil package usage.
package perf

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/perfutil"
	"chromiumos/tast/testing"
)

// RunMultiple is used by perfutil runner to run the performance scenario. It
// wraps the run function around the scenario function. RunMultiple is used in
// conjunction with perfutil runner, usually right after a new instance is
// created.
func RunMultiple(ctx context.Context, r *perfutil.Runner, run func(context.Context, string, func(context.Context, *testing.State)) bool, name string, scenario perfutil.ScenarioFunc, store perfutil.StoreFunc) error {
	// scenarioWrapper wraps scenario around the run function.
	scenarioWrapper := func(ctx context.Context, name string) ([]*metrics.Histogram, error) {
		var hists []*metrics.Histogram
		var err error
		run(ctx, name, func(ctx context.Context, s *testing.State) {
			hists, err = scenario(ctx, name)
			if err != nil {
				testing.ContextLog(ctx, "Failed to run the test scenario")

			}
		})
		return hists, err
	}

	return r.RunMultiple(ctx, name, scenarioWrapper, store)
}

// RunAndWaitAll is used by tests to run the performance scenario. It builds up
// the scenario function which gets wrapped around the run one. RunAndWaitAll
// is used within perfutil.RunMultiple.
func RunAndWaitAll(run func(context.Context, string, func(context.Context, *testing.State)) bool, tconn *chrome.TestConn, f func(ctx context.Context) error, names ...string) perfutil.ScenarioFunc {
	scenario := func(ctx context.Context, name string) ([]*metrics.Histogram, error) {
		var hists []*metrics.Histogram
		var err error
		run(ctx, name, func(ctx context.Context, s *testing.State) {
			hists, err = metrics.RunAndWaitAll(ctx, tconn, time.Minute, f, names...)
			if err != nil {
				testing.ContextLog(ctx, "Failed to run the test scenario")
			}
		})
		return hists, err
	}
	return scenario
}

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

// Run is.
// TODO(tvignatti): Update description. Change name to RunMultiple.
func Run(ctx context.Context, r *perfutil.Runner, run func(context.Context, string, func(context.Context, *testing.State)) bool, name string, scenario perfutil.ScenarioFunc, store perfutil.StoreFunc) error {
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

// RunAndWaitAll is.
// TODO(tvignatti): Update description.
func RunAndWaitAll(ctx context.Context, run func(context.Context, string, func(context.Context, *testing.State)) bool, tconn *chrome.TestConn, f func(ctx context.Context) error, names ...string) perfutil.ScenarioFunc {
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

	// TODO(tvignatti): Why I can't do this below?
	// a := func(ctx context.Context, name string) perfutil.ScenarioFunc {
	// 	var ret perfutil.ScenarioFunc
	// 	run(ctx, name, func(ctx context.Context, s *testing.State) {
	// 		ret = perfutil.RunAndWaitAll(tconn, f, names...)
	// 		// if err != nil {
	// 		// 	testing.ContextLog(ctx, "Failed to run the test scenario")
	// 		// }
	// 	})
	// 	return ret
	// }
	// return a

}

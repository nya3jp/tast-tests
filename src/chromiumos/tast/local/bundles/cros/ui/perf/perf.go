// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package perf contains helper functions to ease perfutil package usage.
package perf

import (
	"context"

	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/perfutil"
	"chromiumos/tast/testing"
)

// Run is a utility function to pass only s.Run instead using the whole 'testing.State' to
// chromiumos/tast/local/perfutil. It has the purpose of information hiding in that package. Use it
// together with perfutil RunMultiple fn.
func Run(run func(context.Context, string, func(context.Context, *testing.State)) bool) perfutil.RunFunc {
	return func(ctx context.Context, name string) {
		run(ctx, name, func(context.Context, *testing.State) {})
	}
}

// RunMultiple is a wrapper around perfutil RunMultiple fn, and likewise, serves to create a new
// runner, conduct runs multiple times, and returns the recorded values.
func RunMultiple(ctx context.Context, br *browser.Browser, run func(context.Context, string, func(context.Context, *testing.State)) bool, scenario perfutil.ScenarioFunc, store perfutil.StoreFunc) *perfutil.Values {
	return perfutil.RunMultiple(ctx, Run(run), br, scenario, store)
}

// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package perf contains helper functions to ease perfutil package usage.
package perf

import (
	"context"

	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/perfutil"
	"chromiumos/tast/testing"
)

// Run is used by perfutil runner to run the performance scenario. It wraps
// s.Run around the scenario function, returning a closure to be used by tests.
func Run(s *testing.State, scenario perfutil.ScenarioFunc) func(ctx context.Context, name string) ([]*metrics.Histogram, error) {
	return func(ctx context.Context, name string) ([]*metrics.Histogram, error) {
		var hists []*metrics.Histogram
		var err error
		s.Run(ctx, name, func(ctx context.Context, s *testing.State) {
			hists, err = scenario(ctx, name)
			if err != nil {
				testing.ContextLog(ctx, "Failed to run the test scenario")
			}
		})
		return hists, err
	}
}

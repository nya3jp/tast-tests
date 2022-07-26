// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package perf contains helper functions to ease perfutil package usage.
package perf

import (
	"context"

	"chromiumos/tast/local/perfutil"
	"chromiumos/tast/testing"
)

// Run is a utility function to pass only s.Run instead using the whole 'testing.State' to
// chromiumos/tast/local/perfutil. It has the purpose of information hiding in that package. Use it
// together with perfutil RunMultiple().
func Run(run func(context.Context, string, func(context.Context, *testing.State)) bool) perfutil.RunFunc {
	return func(ctx context.Context, name string, r *perfutil.Runner) {
		run(ctx, name, func(context.Context, *testing.State) {
			if err := r.RunSubtest(ctx); err != nil {
				testing.ContextLogf(ctx, "Failed to run subtest %s", name)
			}
		})
	}
}

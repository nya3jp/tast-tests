// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package profiler supports capture various kind of system profiler data
// while running test.
//
// Usage
//
//  p, err := profiler.Start(ctx, s, "cros_perf", "some", "profiler")
//  if err != nil {
// 		s.Fatal(err)
//  }
//	defer p.End()
package profiler

import (
	"context"

	"chromiumos/tast/local/profiler/controller"
	"chromiumos/tast/local/profiler/crosperf"
	"chromiumos/tast/testing"
)

// Start creates a profile controller and begins the profiling
// process by calling Start from the controller.
func Start(ctx context.Context, s *testing.State, pnames ...string) (*controller.ProfilerController, error) {
	return controller.Start(ctx, s, pnames)
}

// init registers all the profilers.
func init() {
	crosperf.Register()
}

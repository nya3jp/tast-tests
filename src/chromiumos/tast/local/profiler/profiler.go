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
//  	s.Fatal(err)
//  }
//  defer p.End()
package profiler

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/profiler/crosperf"
)

type profiler interface {
	Start(ctx context.Context, outDir string) error
	End() error
}

// RunningProf is the list of all running profilers.
type RunningProf []profiler

// Profiler is a string represents each of the profiler in the library.
type Profiler int

// Profiler names available in the library.
const (
	CrosPerf Profiler = 1 + iota
)

// profilers map each profiler's representation with the
// appropriate implementation.
var profilers = map[Profiler]profiler{
	CrosPerf: &crosperf.CrosPerf{},
}

// Start initialize all the profilers specified and start the process
// running each of it.
func Start(ctx context.Context, outDir string, pnames ...Profiler) (*RunningProf, error) {
	// Create list of profilers to run.
	var profs RunningProf
	for _, name := range pnames {
		// Find and start each profiler specified.
		prof, ok := profilers[name]
		if !ok {
			return nil, errors.Errorf("unrecognized profiler: %v", name)
		}
		err := prof.Start(ctx, outDir)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to start profiler %v", name)
		}

		// Add running profiler to the controller.
		profs = append(profs, prof)
	}
	return &profs, nil
}

// End terminates all the profilers currently running.
func (p *RunningProf) End() error {
	for _, prof := range *p {
		err := prof.End()
		if err != nil {
			return errors.Wrap(err, "failed to end profiler")
		}
	}
	return nil
}

// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package profiler supports capture various kind of system profiler data
// while running test.
//
// Usage
//
//  p, err := profiler.Start(ctx, s, Profiler.Perf(nil), ...)
//  if err != nil {
//  	// Error handling...
//  }
//  defer func() {
//  	if err := p.End(); err != nil {
//  		// Error handling...
//  	}
//  }()
package profiler

import (
	"context"

	"chromiumos/tast/errors"
)

// Output holds the output of a profiler.
type Output struct {
	Props map[string]interface{}
}

// OutputNull returns a default value for Output.
// This is used when a profiler does not need to output metrics.
func OutputNull() Output {
	return Output{Props: map[string]interface{}{}}
}

type instance interface {
	end() (Output, error)
}

// Profiler is a function construct a profiler instance
// and start the profiler.
type Profiler func(ctx context.Context, outDir string) (instance, error)

// RunningProf is the list of all running profilers.
type RunningProf []instance

// Start uses the set of input profiler constructors to start
// running each of it.
func Start(ctx context.Context, outDir string, profs ...Profiler) (*RunningProf, error) {
	// Create list of profilers to run.
	var rp RunningProf

	// Ends all profilers if one of them fail to start.
	success := false
	defer func() {
		if !success {
			rp.End()
		}
	}()

	for _, prof := range profs {
		// Find and start each profiler specified.
		ins, err := prof(ctx, outDir)
		if err != nil {
			return nil, errors.Wrap(err, "failed to start profiler")
		}
		// Add running profiler to the controller.
		rp = append(rp, ins)
	}
	success = true
	return &rp, nil
}

// End terminates all the profilers currently running and returns output of each profiler in an array of Output.
func (p *RunningProf) End() ([]Output, error) {
	// Ends all profilers, return the first error encountered.
	var firstErr error
	var res []Output
	for _, prof := range *p {
		if output, err := prof.end(); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			res = append(res, OutputNull())
		} else {
			res = append(res, output)
		}
	}
	return res, firstErr
}

// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package profiler supports capture various kind of system profiler data
// while running test.
//
// Usage
//
//  p, err := profiler.Start(ctx, s, Profiler.Perf, ...)
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

type instance interface {
	end() error
}

// Profiler is a function construct a profiler instance
// and start the profiler.
type Profiler func(ctx context.Context, outDir string) (instance, error)

// ProfOpts is a function construct a profiler instance
// with user's specify options for the profiler.
type ProfOpts func(ctx context.Context, outDir string, opts interface{}) (instance, error)

// Profiler's constructors available in the library.
var (
	Perf           Profiler = newPerf
	VMStat         Profiler = newVMStat
	Top            Profiler = newTop
	PerfWithOpts   ProfOpts = newPerfOpts
	VMStatWithOpts ProfOpts = newVMStatOpts
)

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

// StartWithOpts will start a customized profiler with user's specified options.
// This method only instantiates one profiler with the options given.
//
// The options specified varies depending on what profiler is using.
// More detail about each profiler's option can be found on its file.
func StartWithOpts(ctx context.Context, outDir string, prof ProfOpts, opts interface{}) (*RunningProf, error) {
	// Create list of profilers to run.
	var rp RunningProf
	ins, err := prof(ctx, outDir, opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start profiler")
	}
	rp = append(rp, ins)
	return &rp, nil
}

// End terminates all the profilers currently running.
func (p *RunningProf) End() error {
	// Ends all profilers, return the first error encountered.
	var firstErr error
	for _, prof := range *p {
		if err := prof.end(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

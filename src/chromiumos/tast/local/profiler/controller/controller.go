// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package controller helps registering and managing all profilers
// running by the users.
package controller

import (
	"context"
	"sync"
	"sync/atomic"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

var (
	profilersMu     sync.Mutex
	atomicProfilers atomic.Value
)

// ProfilerController manages all the profilers, it can find the correct
// profiler given user's specified profiler's name and performs profiling.
type ProfilerController []profiler

type profiler struct {
	name  string
	start func(context.Context, *testing.State) error
	end   func() error
}

// RegisterProfiler registers a profiler type with a name, start and end
// function to the controller.
func RegisterProfiler(name string, start func(context.Context, *testing.State) error, end func() error) {
	profilersMu.Lock()
	profilers, _ := atomicProfilers.Load().([]profiler)
	atomicProfilers.Store(append(profilers, profiler{name, start, end}))
	profilersMu.Unlock()
}

// Start initialize all the profilers specified and start the process
// running each of it.
func Start(ctx context.Context, s *testing.State, pnames []string) (*ProfilerController, error) {
	// Create list of profilers to run.
	var profs ProfilerController
	for _, name := range pnames {
		// Find and start each profiler specified.
		prof := sniff(name)
		if prof.start == nil || prof.end == nil {
			return nil, errors.Errorf("unrecognized profiler: %v", name)
		}
		err := prof.start(ctx, s)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to start profiler %v", name)
		}
		// Add running profiler to the controller.
		profs = append(profs, prof)
	}
	return &profs, nil
}

// sniff return the profiler that match the name given.
func sniff(name string) profiler {
	profilers, _ := atomicProfilers.Load().([]profiler)
	for _, f := range profilers {
		if name == f.name {
			return f
		}
	}
	return profiler{}
}

// End terminates all the profilers currently running.
func (p *ProfilerController) End() error {
	for _, prof := range *p {
		err := prof.end()
		if err != nil {
			return errors.Wrap(err, "failed to end profiler")
		}
	}
	return nil
}

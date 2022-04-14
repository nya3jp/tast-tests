// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package startstop

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v3/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// TestMidis verifies midis daemon starts correctly.
type TestMidis struct{}

// Name returns the subtest name.
func (*TestMidis) Name() string { return "Midis" }

type midisExpect int

const (
	midisStopped midisExpect = iota
	midisRunning
)

// PreStart makes sure that midis daemon is not running in login screen.
func (*TestMidis) PreStart(ctx context.Context, s *testing.State) {
	if err := waitForMidis(ctx, midisStopped); err != nil {
		s.Error("Midis should not be running in login screen: ", err)
	}
}

// PostStart makes sure that midis daemon is running.
func (*TestMidis) PostStart(ctx context.Context, s *testing.State) {
	if err := waitForMidis(ctx, midisRunning); err != nil {
		s.Error("Midis should run: ", err)
	}
}

// PostStop makes sure that midis deamon is stopped.
func (*TestMidis) PostStop(ctx context.Context, s *testing.State) {
	if err := waitForMidis(ctx, midisStopped); err != nil {
		s.Error("Midis does not stop on Chrome logout: ", err)
	}
}

// waitForMidis waits for midis either running or stopped depending on the
// passed expectation. Returns an error if midis does not get into the state.
func waitForMidis(ctx context.Context, e midisExpect) error {
	const midisExe = "/usr/bin/midis"
	return testing.Poll(ctx, func(ctx context.Context) error {
		all, err := process.Pids()
		if err != nil {
			return testing.PollBreak(err)
		}
		found := false
		for _, pid := range all {
			p, err := process.NewProcess(int32(pid))
			if err != nil {
				// Process is terminated after listing all PIDs.
				continue
			}

			exe, err := p.Exe()
			if err != nil {
				// As same as above, process may be terminated.
				continue
			}
			if exe == midisExe {
				found = true
				break
			}
		}

		// Check if midis is in the expected state, and if not, wait for the next cycle.
		if e == midisStopped && found {
			return errors.New("midis is unexpectly running")
		}
		if e == midisRunning && !found {
			return errors.New("midis is not running")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second})
}

// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
)

// Action is implemented by all setup tasks that need to restore the DUT to its
// previous state.
type Action interface {
	Setup() error
	Cleanup() error
}

// Setup builds a list of Actions and then processes them all at once.
type Setup struct {
	actions []Action
}

// NewSetup creates a new list of Actions, taking the testing.State to
// simplify error handling in tests.
func NewSetup() *Setup {
	return &Setup{actions: []Action{}}
}

// Append adds another Action to be processed.
func (s *Setup) Append(action Action) {
	s.actions = append(s.actions, action)
}

// FailureReporter is used to report failures in Setup or Cleanup methods.
type FailureReporter interface {
	Error(args ...interface{})
	Fatal(args ...interface{})
}

// Setup runs all setup actions and returns a callback that cleans everything
// up. The test will immediately fail if any Actions fails in Setup, and
// will also fail if there are any Cleanup failures.
func (s *Setup) Setup(f FailureReporter) func(FailureReporter) {
	actions := s.actions
	s.actions = []Action{}
	for i, action := range actions {
		if err := action.Setup(); err != nil {
			for j := i - 1; j >= 0; j-- {
				actions[j].Cleanup()
			}
			f.Fatal("Failed to set up: ", err)
		}
	}
	return func(f FailureReporter) {
		for i := len(actions) - 1; i >= 0; i-- {
			action := actions[i]
			if err := action.Cleanup(); err != nil {
				f.Error("Failed to clean up: ", err)
			}
		}
	}
}

// DefaultPowerSetup prepares a DUT to have a power test run by consistently
// configuring power draining components and disabling sources of variance.
func DefaultPowerSetup(ctx context.Context, s *Setup) {
	s.Append(DisableService(ctx, "powerd"))
	s.Append(DisableService(ctx, "update-engine"))
	s.Append(DisableService(ctx, "vnc"))
	s.Append(DisableService(ctx, "dptf"))

	// TODO: backlight
	// TODO: keyboard light
	// TODO: audio
	// TODO: WiFi
	// TODO: Battery discharge
	// TODO: bluetooth
	// TODO: SetLightbarBrightness
	// TODO: nightlight off
}

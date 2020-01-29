// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package setup provides helpers to manage test setup actions that need
// cleanup. Actions are test specific configurations that are cheap, so don't
// need a Precondition.
// Setup is called in the order that actions are Appended. Cleanup is called in
// the reverse order of setup.
package setup

import "chromiumos/tast/testing"

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

// Setup runs all setup actions and returns a callback that cleans everything
// up. The test will immediately fail if any Actions fails in Setup, and
// will also fail if there are any Cleanup failures.
func (s *Setup) Setup(state *testing.State) func(*testing.State) {
	actions := s.actions
	s.actions = []Action{}
	for i, action := range actions {
		if err := action.Setup(); err != nil {
			for j := i - 1; j >= 0; j-- {
				actions[j].Cleanup()
			}
			state.Fatal("Failed to set up: ", err)
		}
	}
	return func(state *testing.State) {
		for i := len(actions) - 1; i >= 0; i-- {
			action := actions[i]
			if err := action.Cleanup(); err != nil {
				state.Error("Failed to clean up: ", err)
			}
		}
	}
}

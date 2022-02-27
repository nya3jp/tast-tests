// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacrosfixt contains tools for working with lacros fixtures.
package lacrosfixt

import (
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/internal/config"
	"chromiumos/tast/testing"
)

// The FixtValue object is made available to users of this fixture via:
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		d := s.FixtValue().(lacros.FixtValue)
//		...
//	}
type FixtValue interface {
	Chrome() *chrome.Chrome        // The CrOS-chrome instance.
	TestAPIConn() *chrome.TestConn // The CrOS-chrome test connection.
	Mode() SetupMode               // Mode used to get the lacros binary.
	LacrosPath() string            // Root directory for lacros-chrome.
	UserTmpDir() string            // Path to the directory that can store user data.
	Options() []config.Option
}

// NewFixture creates a new fixture that can launch Lacros chrome with the given setup mode and
// Chrome options.
func NewFixture(mode SetupMode, fOpt chrome.OptionsCallback) testing.FixtureImpl {
	return NewComposedFixture(mode, func(v FixtValue, pv interface{}) interface{} {
		return v
	}, fOpt)
}

// NewComposedFixture is similar to NewFixture but allows tests to customise the FixtValue
// used. This lets tests compose fixtures via struct embedding.
func NewComposedFixture(mode SetupMode, makeValue func(v FixtValue, pv interface{}) interface{},
	fOpt chrome.OptionsCallback) testing.FixtureImpl {
	return &fixtImpl{
		mode:      mode,
		fOpt:      fOpt,
		makeValue: makeValue,
	}
}

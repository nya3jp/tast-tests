// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacrosfixt contains tools for working with lacros fixtures.
package lacrosfixt

import (
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/testing"
)

// LacrosLogPath is where the lacros log file ought to be.
// N.B. If lacros is launched multiple times (via command line launch, the current default),
// then this logfile will be overwritten. Launching means starting a new lacros process entirely,
// not just creating a new window or tab.
const LacrosLogPath = "/home/chronos/user/lacros/lacros.log"

// The FixtValue object is made available to users of this fixture via:
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		d := s.FixtValue().(lacros.FixtValue)
//		...
//	}
type FixtValue interface {
	chrome.HasChrome
	TestAPIConn() *chrome.TestConn
}

// NewFixture creates a new fixture that can launch Lacros chrome with the given selection and
// Chrome options.
func NewFixture(selection lacros.Selection, fOpt chrome.OptionsCallback) testing.FixtureImpl {
	return NewComposedFixture(selection, func(v FixtValue, pv interface{}) interface{} {
		return v
	}, fOpt)
}

// NewComposedFixture is similar to NewFixture but allows tests to customise the FixtValue
// used. This lets tests compose fixtures via struct embedding.
func NewComposedFixture(selection lacros.Selection, makeValue func(v FixtValue, pv interface{}) interface{},
	fOpt chrome.OptionsCallback) testing.FixtureImpl {
	return &fixtImpl{
		selection: selection,
		fOpt:      fOpt,
		makeValue: makeValue,
	}
}

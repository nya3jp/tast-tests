// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wpr

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// FixtValue is an interface for accessing WPR FixtValue members.
type FixtValue interface {
	FOpt() chrome.OptionsCallback
}

// fixtValueImpl holds values necessary for composed fixtures to interoperate
// with WPR.
type fixtValueImpl struct {
	fOpt chrome.OptionsCallback
}

// FOpt gets the Chrome options necessary to run with WPR.
func (f *fixtValueImpl) FOpt() chrome.OptionsCallback {
	return f.fOpt
}

type fixtImpl struct {
	archiveName string // Specifies archive name for WPR. Must be non-empty.
	mode        Mode
	wpr         *WPR // WPR instance.
}

// NewFixture creates a new fixture that can launch Lacros chrome with the given
// wpr archive and mode.
func NewFixture(wprAchiveName string, mode Mode) testing.FixtureImpl {
	return &fixtImpl{
		archiveName: wprAchiveName,
		mode:        mode,
	}
}

// SetUp is called by tast before each test is run. We use this method to initialize
// the fixture data, or return early if the fixture is already active.
func (f *fixtImpl) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	// Use fixture scoped context so that the WPR server isn't killed early.
	var err error
	f.wpr, err = New(s.FixtContext(), f.mode, s.DataPath(f.archiveName))
	if err != nil {
		s.Fatal("Cannot start WPR: ", err)
	}

	return &fixtValueImpl{
		fOpt: func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return f.wpr.ChromeOptions, nil
		},
	}
}

// TearDown is called after all tests involving this fixture have been run,
// (or failed to be run if the fixture itself fails).
func (f *fixtImpl) TearDown(ctx context.Context, s *testing.FixtState) {
	if f.wpr != nil {
		f.wpr.Close(ctx)
		f.wpr = nil
	}
}

func (f *fixtImpl) Reset(ctx context.Context) error {
	return nil
}

func (f *fixtImpl) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *fixtImpl) PostTest(ctx context.Context, s *testing.FixtTestState) {}

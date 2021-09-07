// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/wpr"
	"chromiumos/tast/testing"
)

const (
	lacrosWPRArchiveDefaultName = "lacros.wprgo"
)

// TODO: Move to WPR directory

type WPRFixtValueImpl struct {
	fOpt chrome.OptionsCallback
}

type WPRFixtValue interface {
	FOpt() chrome.OptionsCallback
}

// wprFixtImpl is a fixture that allows Lacros chrome to be launched.
type wprFixtImpl struct {
	wprArchiveName string   // Specifies archive name for WPR. Must be non-empty.
	wpr            *wpr.WPR // WPR instance.
}

// NewWPRFixture creates a new fixture that can launch Lacros chrome with the given setup mode,
// Chrome options, and WPR archive.
func NewWPRFixture(mode SetupMode, wprAchiveName string) testing.FixtureImpl {
	return &wprFixtImpl{
		wprArchiveName: wprAchiveName,
	}
}

// NewLacrosWPRFixture creates a new fixture that can launch Lacros chrome with the given setup mode,
// Chrome options, and WPR archive. This should be the child of a WPR fixture.
func NewLacrosWPRFixture(mode SetupMode, fOpt chrome.OptionsCallback) testing.FixtureImpl {
	return NewComposedFixture(mode, func(v *FixtValueImpl, pv interface{}) interface{} {
		return v
	}, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
		opts, err := s.ParentValue().(WPRFixtValue).FOpt()(ctx, s)
		if err != nil {
			return nil, err
		}

		optsExtra, err := fOpt(ctx, s)
		if err != nil {
			return nil, err
		}

		opts = append(opts, optsExtra...)
		return opts, nil
	})
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "wpr",
		Desc:            "Fixture using WPR",
		Contacts:        []string{"edcourtney@chromium.org", "hidehiko@chromium.org"},
		Impl:            NewWPRFixture(External, lacrosWPRArchiveDefaultName),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Data:            []string{lacrosWPRArchiveDefaultName},
	})

	// lacrosWPR is the same as lacros but
	// camera/microphone permissions are enabled by default.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosWPR",
		Desc:     "Lacros Chrome from a pre-built image using WPR",
		Contacts: []string{"hidehiko@chromium.org", "edcourtney@chromium.org"},
		Impl: NewLacrosWPRFixture(External, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return nil, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Data:            []string{DataArtifact},
		Vars:            []string{LacrosDeployedBinary},
		Parent:          "wpr",
	})
}

// SetUp is called by tast before each test is run. We use this method to initialize
// the fixture data, or return early if the fixture is already active.
func (f *wprFixtImpl) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	// Use fixture scoped context so that the WPR server isn't killed early.
	var err error
	f.wpr, err = wpr.New(s.FixtContext(), wpr.Record, f.wprArchiveName)
	if err != nil {
		s.Fatal("Cannot start WPR")
	}

	return WPRFixtValueImpl{
		fOpt: func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return f.wpr.ChromeOptions, nil
		},
	}
}

// TearDown is called after all tests involving this fixture have been run,
// (or failed to be run if the fixture itself fails).
func (f *wprFixtImpl) TearDown(ctx context.Context, s *testing.FixtState) {
	if f.wpr != nil {
		f.wpr.Close(ctx)
		f.wpr = nil
	}
}

func (f *wprFixtImpl) Reset(ctx context.Context) error {
	return nil
}

func (f *wprFixtImpl) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *wprFixtImpl) PostTest(ctx context.Context, s *testing.FixtTestState) {}

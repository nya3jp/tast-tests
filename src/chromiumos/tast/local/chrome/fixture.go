// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:     "chromeLoggedIn",
		Desc:     "Logged into a user session",
		Contacts: []string{"nya@chromium.org", "oka@chromium.org"},
		Impl: NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]Option, error) {
			return nil, nil
		}),
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeLoggedInWith100DummyApps",
		Desc:     "Logged into a user session with 100 dummy apps",
		Contacts: []string{"mukai@chromium.org"},
		Impl: NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]Option, error) {
			return nil, nil
		}),
		Parent:          "install100Apps",
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeLoggedInWith100DummyAppsSkiaRenderer",
		Desc:     "Logged into a user session with 100 dummy apps",
		Contacts: []string{"mukai@chromium.org"},
		Impl: NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]Option, error) {
			return []Option{EnableFeatures("UseSkiaRenderer")}, nil
		}),
		Parent:          "install100Apps",
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})
}

// OptionsCallback is the function used to set up the fixture by returning Chrome options.
type OptionsCallback func(ctx context.Context, s *testing.FixtState) ([]Option, error)

// loggedInFixture is a fixture to start Chrome with the given options.
// If the parent is specified, and the parent returns a value of []Option, it
// will also add those options when starting Chrome.
type loggedInFixture struct {
	cr   *Chrome
	fOpt OptionsCallback // Function to generate Chrome Options
}

// NewLoggedInFixture returns a FixtureImpl with a OptionsCallback function to provide Chrome options.
func NewLoggedInFixture(fOpt OptionsCallback) testing.FixtureImpl {
	return &loggedInFixture{fOpt: fOpt}
}

func (f *loggedInFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	var opts []Option
	// If there's a parent fixture and the fixture supplies extra options, use them.
	if extraOpts, ok := s.ParentValue().([]Option); ok {
		opts = append(opts, extraOpts...)
	}

	crOpts, err := f.fOpt(ctx, s)
	if err != nil {
		s.Fatal("Failed to obtain Chrome options: ", err)
	}
	opts = append(opts, crOpts...)

	cr, err := New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	Lock()
	f.cr = cr
	return cr
}

func (f *loggedInFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	Unlock()
	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	f.cr = nil
}

func (f *loggedInFixture) Reset(ctx context.Context) error {
	if err := f.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}
	return nil
}

func (f *loggedInFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *loggedInFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

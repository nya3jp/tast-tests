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
		Name:            "chromeLoggedIn",
		Desc:            "Logged into a user session",
		Contacts:        []string{"nya@chromium.org", "oka@chromium.org"},
		Impl:            NewLoggedInFixture(),
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "chromeLoggedInWith100FakeApps",
		Desc:            "Logged into a user session with 100 fake apps",
		Contacts:        []string{"mukai@chromium.org"},
		Impl:            NewLoggedInFixture(),
		Parent:          "install100Apps",
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "chromeLoggedInWith100FakeAppsSkiaRenderer",
		Desc:            "Logged into a user session with 100 fake apps",
		Contacts:        []string{"mukai@chromium.org"},
		Impl:            NewLoggedInFixture(EnableFeatures("UseSkiaRenderer")),
		Parent:          "install100Apps",
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "chromeRunning",
		Desc:            "There is a Chrome process running, but no user session",
		Contacts:        []string{"nya@chromium.org", "oka@chromium.org"},
		Impl:            NewRunningFixture(),
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})

}

// chromeFixture is a fixture to start Chrome with the given options.
// If the parent is specified, and the parent returns a value of []Option, it
// will also add those options when starting Chrome.
type chromeFixture struct {
	cr   *Chrome
	opts []Option
}

// NewLoggedInFixture returns a FixtureImpl of creating a Chrome instance with
// the given options. By default, this Chrome instance has a user session.
func NewLoggedInFixture(opts ...Option) testing.FixtureImpl {
	return &chromeFixture{opts: opts}
}

// NewRunningFixture returns a FixtureImpl of creating a Chrome instance with
// the given options. This instance does not have a logged in user session.
func NewRunningFixture(opts ...Option) testing.FixtureImpl {
	opts = append(opts, NoLogin())
	return &chromeFixture{opts: opts}
}

func (f *chromeFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	opts := f.opts
	// If there's a parent fixture and the fixture supplies extra options, use them.
	if extraOpts, ok := s.ParentValue().([]Option); ok {
		opts = append(opts, extraOpts...)
	}
	cr, err := New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	Lock()
	f.cr = cr
	return cr
}

func (f *chromeFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	Unlock()
	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	f.cr = nil
}

func (f *chromeFixture) Reset(ctx context.Context) error {
	if err := f.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}
	return nil
}

func (f *chromeFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *chromeFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

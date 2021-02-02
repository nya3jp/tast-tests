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
		Impl:            NewFixture(),
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "chromeLoggedInWith100FakeApps",
		Desc:            "Logged into a user session with 100 fake apps",
		Contacts:        []string{"mukai@chromium.org"},
		Impl:            NewFixture(),
		Parent:          "install100Apps",
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "chromeLoggedInWith100FakeAppsSkiaRenderer",
		Desc:            "Logged into a user session with 100 fake apps",
		Contacts:        []string{"mukai@chromium.org"},
		Impl:            NewFixture(EnableFeatures("UseSkiaRenderer")),
		Parent:          "install100Apps",
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "chromeLoggedOut",
		Desc:            "There is a Chrome process running, but no user session",
		Contacts:        []string{"nya@chromium.org", "oka@chromium.org"},
		Impl:            NewFixture(NoLogin()),
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})

}

// fixture is a fixture to start Chrome with the given options. If the parent is
// specified, and the parent returns a value of []Option, it will also add those
// options when starting Chrome.
type fixture struct {
	cr   *Chrome
	opts []Option
}

// NewFixture returns a FixtureImpl of creating a Chrome instance with
// the given options. By default, this Chrome instance has a user session.
func NewFixture(opts ...Option) testing.FixtureImpl {
	return &fixture{opts: opts}
}

func (f *fixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
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

func (f *fixture) TearDown(ctx context.Context, s *testing.FixtState) {
	Unlock()
	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	f.cr = nil
}

func (f *fixture) Reset(ctx context.Context) error {
	if err := f.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}
	return nil
}

func (f *fixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *fixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

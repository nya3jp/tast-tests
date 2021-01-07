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
		Name:            "chromeLoggedInWith100DummyApps",
		Desc:            "Logged into a user session with 100 dummy apps",
		Contacts:        []string{"mukai@chromium.org"},
		Impl:            NewLoggedInFixture(),
		Parent:          "install100Apps",
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "chromeLoggedInWith100DummyAppsSkiaRenderer",
		Desc:            "Logged into a user session with 100 dummy apps",
		Contacts:        []string{"mukai@chromium.org"},
		Impl:            NewLoggedInFixture(EnableFeatures("UseSkiaRenderer")),
		Parent:          "install100Apps",
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})
}

// FixtureOption is the function used to set up the fixture by returning Chrome option.
type FixtureOption func(ctx context.Context, s *testing.FixtState) (Option, error)

// LoggedInUser returns a fixture option to set up Chrome user.
func LoggedInUser(userVar, passwdVar string) FixtureOption {
	return func(ctx context.Context, s *testing.FixtState) (Option, error) {
		var userID, userPasswd string
		var ok bool
		userID, ok = s.Var(userVar)
		if !ok {
			s.Fatalf("create new Chrome - %s not provided. Please specify it in your test vars configuration", userVar)
		}
		userPasswd, ok = s.Var(passwdVar)
		if !ok {
			s.Fatalf("create new Chrome - %s not provided. Please specify it in your test vars configuration", passwdVar)
		}
		return Auth(userID, userPasswd, ""), nil
	}
}

// loggedInFixture is a fixture to start Chrome with the given options.
// If the parent is specified, and the parent returns a value of []Option, it
// will also add those options when starting Chrome.
type loggedInFixture struct {
	cr    *Chrome
	opts  []Option        // Chrome Options
	fOpts []FixtureOption // Fixture Options
}

// NewLoggedInFixture returns a FixtureImpl of creating a Chrome instance with the given options.
func NewLoggedInFixture(opts ...Option) testing.FixtureImpl {
	return &loggedInFixture{opts: opts}
}

// NewLoggedInFixtureWithOptions returns a FixtureImpl with both Chrome Options and Fixture Options given.
func NewLoggedInFixtureWithOptions(opts []Option, fOpts []FixtureOption) testing.FixtureImpl {
	return &loggedInFixture{opts: opts, fOpts: fOpts}
}

func (f *loggedInFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	opts := f.opts
	// If there's a parent fixture and the fixture supplies extra options, use them.
	if extraOpts, ok := s.ParentValue().([]Option); ok {
		opts = append(opts, extraOpts...)
	}

	for _, fOpt := range f.fOpts {
		crOpt, err := fOpt(ctx, s)
		if err != nil {
			s.Fatal("Failed to call fixture option: ", err)
		}
		opts = append(opts, crOpt)
	}

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

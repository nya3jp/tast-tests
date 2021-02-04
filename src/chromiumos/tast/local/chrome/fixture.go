// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/log"
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
}

// loggedInFixture is a fixture to start Chrome with the given options.
// If the parent is specified, and the parent returns a value of []Option, it
// will also add those options when starting Chrome.
type loggedInFixture struct {
	cr   *Chrome
	opts []Option
	s    *log.Splitter
}

// NewLoggedInFixture returns a FixtureImpl of creating a Chrome instance with the given options.
func NewLoggedInFixture(opts ...Option) testing.FixtureImpl {
	return &loggedInFixture{opts: opts}
}

func (f *loggedInFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
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
	f.s = log.NewSplitter()
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

func (f *loggedInFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	if err := f.s.Start(ctx); err != nil {
		s.Error("Failed to check the chrome log: ", err)
	}
}

func (f *loggedInFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	if err := f.s.SaveLogIntoFile(ctx, filepath.Join(s.OutDir(), "chrome.log")); err != nil {
		s.Error("Failed to save the chrome log for the test: ", err)
	}
}

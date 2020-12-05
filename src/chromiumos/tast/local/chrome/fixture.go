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
		Impl:            newLoggedInFixture(),
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "chromeLoggedInWithEnhancedDeskAnimations",
		Desc:            "Logged into a user session with EnhancedDeskAnimations feature",
		Impl:            newLoggedInFixture(EnableFeatures("EnhancedDeskAnimations")),
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
	})
}

type loggedInFixture struct {
	cr   *Chrome
	opts []Option
}

func newLoggedInFixture(opts ...Option) testing.FixtureImpl {
	return &loggedInFixture{opts: opts}
}

func (f *loggedInFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	cr, err := New(ctx, f.opts...)
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

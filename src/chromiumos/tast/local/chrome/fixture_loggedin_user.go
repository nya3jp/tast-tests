// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// loggedInUserFixture is a fixture to start Chrome with the given user and options.
// Different with loggedInFixture, this fixture will obtain the user credentials
// using the given user and password runtime variable name.
type loggedInUserFixture struct {
	userVar   string
	passwdVar string
	cr        *Chrome
	opts      []Option
}

// NewLoggedInUserFixture returns a FixtureImpl of creating a Chrome instance with the given
// runtime user/password variables and Chrome options.
func NewLoggedInUserFixture(userVar, passwdVar string, opts ...Option) testing.FixtureImpl {
	return &loggedInUserFixture{
		userVar:   userVar,
		passwdVar: passwdVar,
		opts:      opts,
	}
}

func (f *loggedInUserFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	opts := f.opts
	// If there's a parent fixture and the fixture supplies extra options, use them.
	if extraOpts, ok := s.ParentValue().([]Option); ok {
		opts = append(opts, extraOpts...)
	}

	var userID, userPasswd string
	var ok bool
	if userID, ok = s.Var(f.userVar); !ok {
		s.Fatalf("create new Chrome - %s not provided. Please specify it in your test vars configuration", f.userVar)
	}
	if userPasswd, ok = s.Var(f.passwdVar); !ok {
		s.Fatalf("create new Chrome - %s not provided. Please specify it in your test vars configuration", f.passwdVar)
	}
	opts = append(opts, Auth(userID, userPasswd, ""))

	cr, err := New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	Lock()
	f.cr = cr
	return cr
}

func (f *loggedInUserFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	Unlock()
	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	f.cr = nil
}

func (f *loggedInUserFixture) Reset(ctx context.Context) error {
	if err := f.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}
	return nil
}

func (f *loggedInUserFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *loggedInUserFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

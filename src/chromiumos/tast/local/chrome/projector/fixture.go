// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package projector provides Projector login functions.
package projector

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// resetTimeout is the timeout duration of trying to reset the current fixture.
const resetTimeout = 30 * time.Second

// NewProjectorFixture creates a new implementation of the Projector fixture.
func NewProjectorFixture(opts ...chrome.Option) testing.FixtureImpl {
	return &projectorFixture{
		opts: opts,
	}
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:     "projectorLogin",
		Desc:     "Regular user login with Projector feature flag enabled",
		Contacts: []string{"tobyhuang@chromium.org", "cros-projector@google.com"},
		// We need to disable extended features because we don't know if SODA is enabled on the test device.
		Impl: NewProjectorFixture(chrome.EnableFeatures("Projector"), chrome.ExtraArgs("--projector-extended-features-disabled")),
		Vars: []string{
			"ui.gaiaPoolDefault",
		},
		SetUpTimeout:    chrome.GAIALoginTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})
}

type projectorFixture struct {
	cr   *chrome.Chrome
	opts []chrome.Option
}

// FixtData holds information made available to tests that specify this Fixture.
type FixtData struct {
	// Chrome is the running chrome instance.
	Chrome *chrome.Chrome
	// TestConn is a connection to the test extension.
	TestConn *chrome.TestConn
}

func (f *projectorFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	cr, err := chrome.New(ctx, append(f.opts, chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")))...)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	f.cr = cr
	fixtData := &FixtData{
		Chrome:   cr,
		TestConn: tconn,
	}

	// Lock chrome after all Setup is complete so we don't block other fixtures.
	chrome.Lock()

	return fixtData
}

func (f *projectorFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()
	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	f.cr = nil
}

func (f *projectorFixture) Reset(ctx context.Context) error {
	if err := f.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}

	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}

	return nil
}

func (f *projectorFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *projectorFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

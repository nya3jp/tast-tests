// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package projector provides Projector login functions.
package projector

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/testing"
)

// resetTimeout is the timeout duration of trying to reset the current fixture.
const resetTimeout = 30 * time.Second

// NewProjectorFixture creates a new implementation of the Projector fixture.
func NewProjectorFixture(fOpts chrome.OptionsCallback) testing.FixtureImpl {
	return &projectorFixture{fOpts: fOpts}
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:     "projectorLogin",
		Desc:     "Regular user login with Projector feature flag enabled",
		Contacts: []string{"tobyhuang@chromium.org", "cros-projector@google.com"},
		Impl: NewProjectorFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.EnableFeatures("Projector, ProjectorAppDebug, ProjectorAnnotator, ProjectorTutorialVideoView"),
				chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
			}, nil
		}),
		Vars: []string{
			"ui.gaiaPoolDefault",
		},
		SetUpTimeout:    chrome.GAIALoginTimeout + time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosProjectorLogin",
		Desc:     "Regular user login to lacros with Projector feature flag enabled",
		Contacts: []string{"hyungtaekim@chromium.org", "cros-projector@google.com"},
		Impl: NewProjectorFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return lacrosfixt.NewConfig(lacrosfixt.ChromeOptions(
				chrome.EnableFeatures("Projector, ProjectorAppDebug, ProjectorAnnotator, ProjectorTutorialVideoView"),
				chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
			)).Opts()
		}),
		Vars: []string{
			"ui.gaiaPoolDefault",
		},
		SetUpTimeout:    chrome.GAIALoginTimeout + time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})
}

type projectorFixture struct {
	cr    *chrome.Chrome
	fOpts chrome.OptionsCallback
}

// FixtData holds information made available to tests that specify this Fixture.
type FixtData struct {
	// Chrome is the running chrome instance.
	Chrome *chrome.Chrome
	// TestConn is a connection to the test extension.
	TestConn *chrome.TestConn
}

func (f *projectorFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	opts, err := f.fOpts(ctx, s)
	if err != nil {
		s.Fatal("Failed to obtain Chrome options: ", err)
	}
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	f.cr = cr
	// SWA installation is not guaranteed during startup.
	// Wait for installation finished before starting test.
	s.Log("Wait for Screencast app to be installed")
	if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.Projector.ID, 2*time.Minute); err != nil {
		s.Fatal("Failed to wait for installed app: ", err)
	}
	// Lock chrome after all Setup is complete so we don't block other fixtures.
	chrome.Lock()
	return &FixtData{
		Chrome:   cr,
		TestConn: tconn,
	}
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

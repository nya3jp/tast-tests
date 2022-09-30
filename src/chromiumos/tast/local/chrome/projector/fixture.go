// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package projector provides Projector login functions.
package projector

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/familylink"
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
				chrome.EnableFeatures("Projector, ProjectorAppDebug, ProjectorAnnotator, ProjectorTutorialVideoView, ProjectorLocalPlayback"),
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
				chrome.EnableFeatures("Projector, ProjectorAppDebug, ProjectorAnnotator, ProjectorTutorialVideoView, ProjectorLocalPlayback"),
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

	// Identical to the familyLinkUnicornLogin fixture, but uses a
	// different test account and isolates sessions for Projector
	// tests.
	testing.AddFixture(&testing.Fixture{
		Name:     "projectorUnicornLogin",
		Desc:     "Supervised Family Link user login with Unicorn account for Projector tests",
		Contacts: []string{"tobyhuang@chromium.org", "cros-families-eng+test@google.com"},
		Impl:     familylink.NewFamilyLinkFixture("projector.parentEmail", "projector.parentPassword", "projector.childEmail", "projector.childPassword", true /*isOwner*/),
		Vars: []string{
			"projector.parentEmail",
			"projector.parentPassword",
			"projector.childEmail",
			"projector.childPassword",
		},
		SetUpTimeout:    chrome.GAIALoginChildTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "projectorEduLogin",
		Desc:     "Managed EDU user login with fakeDMS policy setup for Projector tests",
		Contacts: []string{"tobyhuang@chromium.org", "cros-families-eng+test@google.com"},
		Impl:     familylink.NewFamilyLinkFixture("projector.eduEmail", "projector.eduPassword", "", "", true /*isOwner*/),
		Vars: []string{
			"projector.eduEmail",
			"projector.eduPassword",
		},
		SetUpTimeout:    chrome.ManagedUserLoginTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
		Parent:          fixture.PersistentProjectorEDU,
	})
}

type projectorFixture struct {
	cr    *chrome.Chrome
	fOpts chrome.OptionsCallback
}

// FixtData holds information made available to tests that specify this Fixture.
type FixtData struct {
	// Chrome is the running chrome instance.
	chrome *chrome.Chrome
	// TestConn is a connection to the test extension.
	testConn *chrome.TestConn
}

// Chrome implements the HasChrome interface.
func (f FixtData) Chrome() *chrome.Chrome {
	if f.chrome == nil {
		panic("Chrome is called with nil chrome instance")
	}
	return f.chrome
}

// TestConn implements the HasTestConn interface.
func (f FixtData) TestConn() *chrome.TestConn {
	if f.testConn == nil {
		panic("TestConn is called with nil testConn instance")
	}
	return f.testConn
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
		chrome:   cr,
		testConn: tconn,
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

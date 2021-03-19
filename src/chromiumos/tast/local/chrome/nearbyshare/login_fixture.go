// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// NewNearbyShareLogin creates a fixture that logs in and enables Nearby Share.
func NewNearbyShareLogin(gaiaLogin bool) testing.FixtureImpl {
	defaultNearbyOpts := []chrome.Option{
		chrome.EnableFeatures("IntentHandlingSharing", "NearbySharing", "Sharesheet"),
		chrome.ExtraArgs("--nearby-share-verbose-logging"),
	}
	return &nearbyShareLoginFixture{
		opts:      defaultNearbyOpts,
		gaiaLogin: gaiaLogin,
	}
}

func init() {
	const (
		// These are the default GAIA credentials that will be used to sign in on CrOS. Use the optional "custom" vars below to specify you'd like to specify your own credentials while running locally on personal devices.
		defaultCrOSUsername = "nearbyshare.cros_username"
		defaultCrOSPassword = "nearbyshare.cros_password"

		// These vars can be used from the command line when running tests locally to configure the tests to run on personal GAIA accounts.
		// Use these vars to log in with your own GAIA credentials. If running in-contacts tests with an Android device, it is expected that the CrOS user and Android user are already mutual contacts.
		customCrOSUsername = "cros_username"
		customCrOSPassword = "cros_password"
	)

	testing.AddFixture(&testing.Fixture{
		Name:            "nearbyShareTestUserLogin",
		Desc:            "CrOS login with Test User and Nearby Share flags enabled.",
		Parent:          "nearbyShareAndroidSetup",
		Impl:            NewNearbyShareLogin(false),
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:   "nearbyShareGAIALogin",
		Desc:   "CrOS login with GAIA and Nearby Share flags enabled.",
		Parent: "nearbyShareAndroidSetup",
		Impl:   NewNearbyShareLogin(true),
		Vars: []string{
			defaultCrOSUsername,
			defaultCrOSPassword,
			customCrOSUsername,
			customCrOSPassword,
		},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})
}

type nearbyShareLoginFixture struct {
	opts      []chrome.Option
	gaiaLogin bool
	cr        *chrome.Chrome
}

func (f *nearbyShareLoginFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	// Android device info from parent fixture
	androidDevice := s.ParentValue().(*FixtData).AndroidDevice
	androidDeviceName := s.ParentValue().(*FixtData).AndroidDeviceName
	androidUsername := s.ParentValue().(*FixtData).AndroidUsername
	loggedIn := s.ParentValue().(*FixtData).AndroidLoggedIn

	crosUsername := chrome.DefaultUser
	if f.gaiaLogin {
		crosUsername = s.RequiredVar("nearbyshare.cros_username")
		crosPassword := s.RequiredVar("nearbyshare.cros_password")
		customUser, userOk := s.Var("cros_username")
		customPass, passOk := s.Var("cros_password")
		if userOk && passOk {
			s.Log("Logging in with user-provided credentials")
			crosUsername = customUser
			crosPassword = customPass
		} else {
			s.Log("Logging in with default GAIA credentials")
		}
		f.opts = append(f.opts, chrome.GAIALogin(chrome.Creds{User: crosUsername, Pass: crosPassword}))
	}

	cr, err := chrome.New(
		ctx,
		f.opts...,
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	f.cr = cr
	// Lock chrome after all Setup is complete so we don't block other fixtures.
	chrome.Lock()
	return &FixtData{
		Chrome:            cr,
		CrOSUsername:      crosUsername,
		AndroidDevice:     androidDevice,
		AndroidDeviceName: androidDeviceName,
		AndroidUsername:   androidUsername,
		AndroidLoggedIn:   loggedIn,
	}
}

func (f *nearbyShareLoginFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()
	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	f.cr = nil
}
func (f *nearbyShareLoginFixture) Reset(ctx context.Context) error {
	if err := f.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}
	return nil
}
func (f *nearbyShareLoginFixture) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (f *nearbyShareLoginFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

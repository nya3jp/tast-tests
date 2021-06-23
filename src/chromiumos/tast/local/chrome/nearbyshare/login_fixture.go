// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"strconv"
	"time"

	nearbycommon "chromiumos/tast/common/cros/nearbyshare"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// NewNearbyShareLogin creates a fixture that logs in and enables Nearby Share.
// Note that nearbyShareGAIALogin inherits from nearbyShareAndroidSetup.
func NewNearbyShareLogin() testing.FixtureImpl {
	defaultNearbyOpts := []chrome.Option{
		chrome.DisableFeatures("SplitSettingsSync"),
		chrome.ExtraArgs("--nearby-share-verbose-logging", "--enable-logging", "--vmodule=*blue*=1", "--vmodule=*nearby*=1"),
	}
	return &nearbyShareLoginFixture{
		opts: defaultNearbyOpts,
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

		// Set this var to True to prevent the tests from clearing existing user accounts from the DUT.
		keepState = nearbycommon.KeepStateVar
	)

	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareGAIALogin",
		Desc: "CrOS login with GAIA and Nearby Share flags enabled",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Parent: "nearbyShareAndroidSetup",
		Impl:   NewNearbyShareLogin(),
		Vars: []string{
			defaultCrOSUsername,
			defaultCrOSPassword,
			customCrOSUsername,
			customCrOSPassword,
			keepState,
		},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})
}

type nearbyShareLoginFixture struct {
	opts []chrome.Option
	cr   *chrome.Chrome
}

func (f *nearbyShareLoginFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	// Android device info from parent fixture
	androidDevice := s.ParentValue().(*FixtData).AndroidDevice
	androidDeviceName := s.ParentValue().(*FixtData).AndroidDeviceName
	androidUsername := s.ParentValue().(*FixtData).AndroidUsername
	loggedIn := s.ParentValue().(*FixtData).AndroidLoggedIn

	// Ensure we have Android logs even if setup fails.
	if err := androidDevice.ClearLogcat(ctx); err != nil {
		s.Fatal("Failed to clear logcat at start of fixture setup")
	}
	defer androidDevice.DumpLogs(ctx, s.OutDir(), "fixture_setup_logcat.txt")

	crosUsername := s.RequiredVar("nearbyshare.cros_username")
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

	if val, ok := s.Var(nearbycommon.KeepStateVar); ok {
		b, err := strconv.ParseBool(val)
		if err != nil {
			s.Fatalf("Unable to convert %v var to bool: %v", nearbycommon.KeepStateVar, err)
		}
		if b {
			f.opts = append(f.opts, chrome.KeepState())
		}
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

	// Sometimes during login the tcp connection to the snippet server on Android is lost.
	// If we cannot do a simple rpc call, reconnect to the snippet server.
	if _, err := androidDevice.GetNearbySharingVersion(ctx); err != nil {
		s.Log("Lost connection to the Snippet server. Reconnecting")
		if err := androidDevice.ReconnectToSnippet(ctx); err != nil {
			s.Fatal("Failed to reconnect to the snippet server: ", err)
		}
	}

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

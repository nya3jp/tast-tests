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
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

type mediums int

const (
	defaultMediums mediums = iota
	webRtcOnly
	wlanOnly
)

// NewNearbyShareLogin creates a fixture that logs in and enables Nearby Share.
// Note that nearbyShareGAIALogin inherits from nearbyShareAndroidSetup.
func NewNearbyShareLogin(arcEnabled, backgroundScanningEnabled bool, m mediums) testing.FixtureImpl {
	defaultNearbyOpts := []chrome.Option{
		chrome.EnableFeatures("GwpAsanMalloc", "GwpAsanPartitionAlloc"),
		chrome.DisableFeatures("SplitSettingsSync"),
		chrome.ExtraArgs("--nearby-share-verbose-logging", "--enable-logging", "--vmodule=*blue*=1", "--vmodule=*nearby*=1"),
	}
	if arcEnabled {
		defaultNearbyOpts = append(defaultNearbyOpts, chrome.ARCEnabled(), chrome.EnableFeatures("ArcNearbySharing"), chrome.ExtraArgs(arc.DisableSyncFlags()...))
	}
	if backgroundScanningEnabled {
		defaultNearbyOpts = append(defaultNearbyOpts, chrome.EnableFeatures("BluetoothAdvertisementMonitoring"),
			chrome.EnableFeatures("NearbySharingBackgroundScanning"))
	}
	switch m {
	case webRtcOnly:
		defaultNearbyOpts = append(defaultNearbyOpts, chrome.EnableFeatures("NearbySharingWebRtc"), chrome.DisableFeatures("NearbySharingWifiLan"))
	case wlanOnly:
		defaultNearbyOpts = append(defaultNearbyOpts, chrome.DisableFeatures("NearbySharingWebRtc"), chrome.EnableFeatures("NearbySharingWifiLan"))
	}

	return &nearbyShareLoginFixture{
		opts:       defaultNearbyOpts,
		arcEnabled: arcEnabled,
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
		Impl:   NewNearbyShareLogin(false, false, defaultMediums),
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

	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareGAIALoginBackgroundScanningEnabled",
		Desc: "CrOS login with GAIA; Nearby Share and Background scanning flags enabled",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Parent: "nearbyShareAndroidSetup",
		Impl:   NewNearbyShareLogin(false, true, defaultMediums),
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

	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareGAIALoginARCEnabled",
		Desc: "CrOS login with GAIA, Nearby Share flags enabled, and ARC enabled",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"arc-app-dev@google.com",
		},
		Parent: "nearbyShareAndroidSetup",
		Impl:   NewNearbyShareLogin(true, false, defaultMediums),
		Vars: []string{
			defaultCrOSUsername,
			defaultCrOSPassword,
			customCrOSUsername,
			customCrOSPassword,
			keepState,
		},
		SetUpTimeout:    3 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareGAIALoginWebRtcOnly",
		Desc: "CrOS login with GAIA; only use WebRTC upgrade medium",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Parent: "nearbyShareAndroidSetup",
		Impl:   NewNearbyShareLogin(false, false, webRtcOnly),
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

	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareGAIALoginWlanOnly",
		Desc: "CrOS login with GAIA; only use WLAN upgrade medium",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Parent: "nearbyShareAndroidSetup",
		Impl:   NewNearbyShareLogin(false, false, wlanOnly),
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
	opts       []chrome.Option
	cr         *chrome.Chrome
	arcEnabled bool
	arc        *arc.ARC
}

func (f *nearbyShareLoginFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	// Android device info from parent fixture
	androidDevice := s.ParentValue().(*FixtData).AndroidDevice
	androidDeviceName := s.ParentValue().(*FixtData).AndroidDeviceName
	androidUsername := s.ParentValue().(*FixtData).AndroidUsername
	loggedIn := s.ParentValue().(*FixtData).AndroidLoggedIn

	// Reset and save logcat so we have Android logs even if fixture setup fails.
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

	// Starting ARC restarts ADB, which kills the connection to the snippet.
	// Starting it here (before we check the connection and attempt a reconnect) will ensure the snippet connection is up.
	if f.arcEnabled {
		a, err := arc.New(ctx, s.OutDir())
		if err != nil {
			s.Fatal("Failed to start ARC: ", err)
		}
		f.arc = a
	}

	// Sometimes during login the tcp connection to the snippet server on Android is lost.
	// If we cannot do a simple rpc call, reconnect to the snippet server.
	if _, err := androidDevice.GetNearbySharingVersion(ctx); err != nil {
		s.Log("Lost connection to the Snippet server. Reconnecting")
		if err := androidDevice.ReconnectToSnippet(ctx); err != nil {
			s.Fatal("Failed to reconnect to the snippet server: ", err)
		}
	}

	// Lock chrome after all Setup is complete so we don't block other fixtures.
	chrome.Lock()

	return &FixtData{
		Chrome:            cr,
		CrOSUsername:      crosUsername,
		AndroidDevice:     androidDevice,
		AndroidDeviceName: androidDeviceName,
		AndroidUsername:   androidUsername,
		AndroidLoggedIn:   loggedIn,
		ARC:               f.arc,
	}
}

func (f *nearbyShareLoginFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()
	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	f.cr = nil
	if f.arc != nil {
		f.arc.Close(ctx)
		f.arc = nil
	}
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
func (f *nearbyShareLoginFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	if f.arcEnabled {
		if err := f.arc.ResetOutDir(ctx, s.OutDir()); err != nil {
			s.Error("Failed to to reset outDir field of ARC object: ", err)
		}
	}
}
func (f *nearbyShareLoginFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	if f.arcEnabled {
		if err := f.arc.SaveLogFiles(ctx); err != nil {
			s.Error("Failed to to save ARC-related log files: ", err)
		}
	}
}

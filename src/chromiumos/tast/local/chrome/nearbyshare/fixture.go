// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/nearbyshare/nearbysetup"
	"chromiumos/tast/local/chrome/nearbyshare/nearbysnippet"
	"chromiumos/tast/local/chrome/nearbyshare/nearbytestutils"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

// resetTimeout is the timeout duration to trying reset of the current fixture.
const resetTimeout = 30 * time.Second

// NewNearbyShareFixture creates a new implementation of the Nearby Share fixture.
func NewNearbyShareFixture(dataUsage nearbysetup.DataUsage, visibility nearbysetup.Visibility, gaiaLogin, androidSetup bool, opts ...chrome.Option) testing.FixtureImpl {
	defaultNearbyOpts := []chrome.Option{
		chrome.EnableFeatures("IntentHandlingSharing", "NearbySharing", "Sharesheet"),
		chrome.ExtraArgs("--nearby-share-verbose-logging"),
	}
	return &nearbyShareFixture{
		opts:         append(defaultNearbyOpts, opts...),
		dataUsage:    dataUsage,
		visibility:   visibility,
		gaiaLogin:    gaiaLogin,
		androidSetup: androidSetup,
	}
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "nearbyShareDataUsageOfflineAllContactsTestUserNoAndroid",
		Desc:            "CrOS Nearby Share enabled and configured with 'Data Usage' set to 'Offline' and 'Visibility' set to 'All Contacts'. No Android device setup.",
		Impl:            NewNearbyShareFixture(nearbysetup.DataUsageOffline, nearbysetup.VisibilityAllContacts, false, false),
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "nearbyShareDataUsageOfflineAllContactsTestUser",
		Desc:            "Nearby Share enabled on CrOS and Android configured with 'Data Usage' set to 'Offline' and 'Visibility' set to 'All Contacts'",
		Impl:            NewNearbyShareFixture(nearbysetup.DataUsageOffline, nearbysetup.VisibilityAllContacts, false, true),
		Vars:            []string{"rooted"},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareDataUsageOfflineAllContactsGAIA",
		Desc: "Nearby Share enabled on CrOS and Android configured with 'Data Usage' set to 'Offline',  'Visibility' set to 'All Contacts' and logged in with a real GAIA account",
		Impl: NewNearbyShareFixture(nearbysetup.DataUsageOffline, nearbysetup.VisibilityAllContacts, true, true),
		Vars: []string{
			"rooted",
			"username",
			"password",
			"nearbyshare.cros_username",
			"nearbyshare.cros_password",
		},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})
}

type nearbyShareFixture struct {
	cr            *chrome.Chrome
	opts          []chrome.Option
	dataUsage     nearbysetup.DataUsage
	visibility    nearbysetup.Visibility
	gaiaLogin     bool
	androidSetup  bool
	androidDevice *nearbysnippet.AndroidNearbyDevice
	// ChromeReader is the line reader for collecting Chrome logs.
	ChromeReader *syslog.LineReader
}

// FixtData holds information made available to tests that specify this Fixture.
type FixtData struct {
	// Chrome is the running chrome instance.
	Chrome *chrome.Chrome

	// TestConn is a connection to the test extension.
	TestConn *chrome.TestConn

	// CrOSDeviceName is the CrOS device name configured for Nearby Share.
	CrOSDeviceName string

	// AndroidDevice is an object for interacting with the connected Android device's Snippet Library.
	AndroidDevice *nearbysnippet.AndroidNearbyDevice

	// AndroidDeviceName is the Android device name configured for Nearby Share.
	AndroidDeviceName string
}

func (f *nearbyShareFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	if f.gaiaLogin {
		username := s.RequiredVar("nearbyshare.cros_username")
		password := s.RequiredVar("nearbyshare.cros_password")
		customUser, userOk := s.Var("username")
		customPass, passOk := s.Var("password")
		if userOk && passOk {
			s.Log("Logging in with user-provided credentials")
			username = customUser
			password = customPass
		} else {
			s.Log("Logging in with default GAIA credentials")
		}
		f.opts = append(f.opts, chrome.Auth(username, password, ""), chrome.GAIALogin())
	}

	cr, err := chrome.New(
		ctx,
		f.opts...,
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	chrome.Lock()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Set up Nearby Share on the CrOS device.
	const crosBaseName = "cros_test"
	crosDisplayName := nearbytestutils.RandomDeviceName(crosBaseName)
	if err := nearbysetup.CrOSSetup(ctx, tconn, cr, f.dataUsage, f.visibility, crosDisplayName); err != nil {
		s.Fatal("Failed to set up Nearby Share: ", err)
	}

	f.cr = cr
	fixData := &FixtData{
		Chrome:         cr,
		TestConn:       tconn,
		CrOSDeviceName: crosDisplayName,
	}

	if f.androidSetup {
		// Set up Nearby Share on the Android device. Don't override GMS Core flags or perform settings changes that require root access if specified in the runtime vars.
		rooted := true
		if val, ok := s.Var("rooted"); ok {
			b, err := strconv.ParseBool(val)
			if err != nil {
				s.Fatal("Unable to convert rooted var to bool: ", err)
			}
			rooted = b
		}
		const androidBaseName = "android_test"
		androidDisplayName := nearbytestutils.RandomDeviceName(androidBaseName)
		// TODO(crbug/1127165): Replace with s.DataPath(nearbysnippet.ZipName) when data is supported in Fixtures.
		apkZipPath := "/usr/local/share/tast/data_pushed/chromiumos/tast/local/bundles/cros/nearbyshare/data/nearby_snippet.zip"
		androidDevice, err := nearbysetup.AndroidSetup(
			ctx, apkZipPath, rooted,
			nearbysetup.DefaultScreenTimeout,
			nearbysnippet.DataUsageOffline,
			nearbysnippet.VisibilityAllContacts,
			androidDisplayName,
		)
		if err != nil {
			s.Fatal("Failed to prepare connected Android device for Nearby Share testing: ", err)
		}
		f.androidDevice = androidDevice
		fixData.AndroidDevice = androidDevice
		fixData.AndroidDeviceName = androidDisplayName
		if err := f.androidDevice.DumpLogs(ctx, s.OutDir(), "fixture_setup_logcat.txt"); err != nil {
			s.Fatal("Failed to save logcat for the fixture setup: ", err)
		}
	}
	return fixData
}

func (f *nearbyShareFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()
	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	f.cr = nil
	if f.androidDevice != nil {
		f.androidDevice.StopSnippet(ctx)
	}
}

func (f *nearbyShareFixture) Reset(ctx context.Context) error {
	if err := f.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}
	return nil
}

func (f *nearbyShareFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	chromeReader, err := nearbytestutils.StartLogging(ctx, syslog.ChromeLogFile)
	if err != nil {
		s.Fatal("Failed to start Chrome logging: ", err)
	}
	f.ChromeReader = chromeReader
	if f.androidSetup {
		if err := f.androidDevice.ClearLogcat(ctx); err != nil {
			s.Fatal("Failed to clear logcat before the test run: ", err)
		}
	}
}

func (f *nearbyShareFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	if f.ChromeReader == nil {
		s.Fatal("ChromeReader not defined")
	}
	if err := nearbytestutils.SaveLogs(ctx, f.ChromeReader, filepath.Join(s.OutDir(), ChromeLog)); err != nil {
		s.Fatal("Failed to save Chrome log: ", err)
	}
	if f.androidSetup {
		f.androidDevice.DumpLogs(ctx, s.OutDir(), "nearby_logcat.txt")

	}
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
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

// NewNearbyShareFixtureNoAndroid creates a new implementation of the Nearby Share fixture without an Android device.
func NewNearbyShareFixtureNoAndroid(crosDataUsage nearbysetup.DataUsage, crosVisibility nearbysetup.Visibility, gaiaLogin bool, opts ...chrome.Option) testing.FixtureImpl {
	defaultNearbyOpts := []chrome.Option{
		chrome.EnableFeatures("IntentHandlingSharing", "NearbySharing", "Sharesheet"),
		chrome.ExtraArgs("--nearby-share-verbose-logging"),
	}
	return &nearbyShareFixture{
		opts:           append(defaultNearbyOpts, opts...),
		crosDataUsage:  crosDataUsage,
		crosVisibility: crosVisibility,
		gaiaLogin:      gaiaLogin,
	}
}

// NewNearbyShareFixtureWithAndroid creates a new implementation of the Nearby Share fixture with an Android device.
func NewNearbyShareFixtureWithAndroid(crosDataUsage nearbysetup.DataUsage, crosVisibility nearbysetup.Visibility, androidDataUsage nearbysnippet.DataUsage, androidVisibility nearbysnippet.Visibility, gaiaLogin bool, opts ...chrome.Option) testing.FixtureImpl {
	defaultNearbyOpts := []chrome.Option{
		chrome.EnableFeatures("IntentHandlingSharing", "NearbySharing", "Sharesheet"),
		chrome.ExtraArgs("--nearby-share-verbose-logging"),
	}
	return &nearbyShareFixture{
		opts:              append(defaultNearbyOpts, opts...),
		crosDataUsage:     crosDataUsage,
		crosVisibility:    crosVisibility,
		androidDataUsage:  androidDataUsage,
		androidVisibility: androidVisibility,
		gaiaLogin:         gaiaLogin,
		androidSetup:      true,
	}
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "nearbyShareDataUsageOfflineAllContactsTestUserNoAndroid",
		Desc:            "CrOS Nearby Share enabled and configured with 'Data Usage' set to 'Offline' and 'Visibility' set to 'All Contacts'. No Android device setup.",
		Impl:            NewNearbyShareFixtureNoAndroid(nearbysetup.DataUsageOffline, nearbysetup.VisibilityAllContacts, false),
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareDataUsageOfflineAllContactsTestUser",
		Desc: "Nearby Share enabled on CrOS and Android configured with 'Data Usage' set to 'Offline' and 'Visibility' set to 'All Contacts'",
		Impl: NewNearbyShareFixtureWithAndroid(nearbysetup.DataUsageOffline, nearbysetup.VisibilityAllContacts, nearbysnippet.DataUsageOffline, nearbysnippet.VisibilityAllContacts, false),
		Vars: []string{
			// Specify -var=rooted=false when running on an unrooted device to skip steps that require adb root access.
			"rooted",
			// Specify -var=skipAndroidLogin=true if the Android device is logged in to a personal account.
			// Otherwise we will attempt removing all Google accounts and adding a test account to the phone.
			"skipAndroidLogin",
			"nearbyshare.android_username",
			"nearbyshare.android_password",
		},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareDataUsageOfflineAllContactsGAIA",
		Desc: "Nearby Share enabled on CrOS and Android configured with 'Data Usage' set to 'Offline',  'Visibility' set to 'All Contacts' and logged in with a real GAIA account",
		Impl: NewNearbyShareFixtureWithAndroid(nearbysetup.DataUsageOffline, nearbysetup.VisibilityAllContacts, nearbysnippet.DataUsageOffline, nearbysnippet.VisibilityAllContacts, true),
		Vars: []string{
			"rooted",
			// Specify -var=username=<your username> and -var=password=<your password> to log in to a different GAIA account on the CrOS device.
			// Combine this with -var=skipAndroidLogin=true and a manually-signed-in Android device to run the tests on your own GAIA accounts.
			// Otherwise we will log in both devices to pre-configured GAIA test accounts. These account credentials are loaded with the
			// nearbyshare.cros_{username,password} and nearbyshare.android_{username,password} vars, so these vars are not meant to be overridden by the user.
			"username",
			"password",
			"nearbyshare.cros_username",
			"nearbyshare.cros_password",
			"skipAndroidLogin",
			"nearbyshare.android_username",
			"nearbyshare.android_password",
		},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	// Online data usage requires a real GAIA login.
	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareDataUsageOnlineAllContactsGAIA",
		Desc: "Nearby Share enabled on CrOS and Android configured with 'Data Usage' set to 'Online',  'Visibility' set to 'All Contacts' and logged in with a real GAIA account",
		Impl: NewNearbyShareFixtureWithAndroid(nearbysetup.DataUsageOnline, nearbysetup.VisibilityAllContacts, nearbysnippet.DataUsageOnline, nearbysnippet.VisibilityAllContacts, true),
		Vars: []string{
			"rooted",
			"username",
			"password",
			"nearbyshare.cros_username",
			"nearbyshare.cros_password",
			"skipAndroidLogin",
			"nearbyshare.android_username",
			"nearbyshare.android_password",
		},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareDataUsageOnlineNoOneGAIA",
		Desc: "Nearby Share enabled on CrOS and Android configured with 'Data Usage' set to 'Online' and 'Visibility' set to 'No One' and logged in with a real GAIA account",
		Impl: NewNearbyShareFixtureWithAndroid(nearbysetup.DataUsageOnline, nearbysetup.VisibilityNoOne, nearbysnippet.DataUsageOnline, nearbysnippet.VisibilityNoOne, true),
		Vars: []string{
			"rooted",
			"username",
			"password",
			"nearbyshare.cros_username",
			"nearbyshare.cros_password",
			"skipAndroidLogin",
			"nearbyshare.android_username",
			"nearbyshare.android_password",
		},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})
}

type nearbyShareFixture struct {
	cr                *chrome.Chrome
	opts              []chrome.Option
	crosDataUsage     nearbysetup.DataUsage
	crosVisibility    nearbysetup.Visibility
	androidDataUsage  nearbysnippet.DataUsage
	androidVisibility nearbysnippet.Visibility
	gaiaLogin         bool
	androidSetup      bool
	androidDevice     *nearbysnippet.AndroidNearbyDevice
	crosAttributes    *nearbysetup.CrosAttributes
	androidAttributes *nearbysnippet.AndroidAttributes
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
	crosUsername := chrome.DefaultUser
	if f.gaiaLogin {
		crosUsername = s.RequiredVar("nearbyshare.cros_username")
		crosPassword := s.RequiredVar("nearbyshare.cros_password")
		customUser, userOk := s.Var("username")
		customPass, passOk := s.Var("password")
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

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Set up Nearby Share on the CrOS device.
	const crosBaseName = "cros_test"
	crosDisplayName := nearbytestutils.RandomDeviceName(crosBaseName)
	if err := nearbysetup.CrOSSetup(ctx, tconn, cr, f.crosDataUsage, f.crosVisibility, crosDisplayName); err != nil {
		s.Fatal("Failed to set up Nearby Share: ", err)
	}

	// Store CrOS test metadata for reporting.
	crosAttributes, err := nearbysetup.GetCrosAttributes(ctx, tconn, crosDisplayName, crosUsername, f.crosDataUsage, f.crosVisibility)
	if err != nil {
		s.Fatal("Failed to get CrOS attributes for reporting: ", err)
	}
	f.crosAttributes = crosAttributes

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
		// The data path changes based on whether -build=true or -build=false is supplied to `tast run`.
		// Local test runs on your workstation use -build=true by default, while lab runs use -build=false.
		const (
			prebuiltLocalDataPath = "/usr/local/share/tast/data/chromiumos/tast/local/bundles/cros/nearbyshare/data"
			builtLocalDataPath    = "/usr/local/share/tast/data_pushed/chromiumos/tast/local/bundles/cros/nearbyshare/data"
			apkZipName            = "nearby_snippet.zip"
			accountUtilZipName    = "google_account_util.zip"
		)

		// Use the built local data path if it exists, and fall back to the prebuilt data path otherwise.
		apkZipPath := filepath.Join(builtLocalDataPath, apkZipName)
		accountUtilZipPath := filepath.Join(builtLocalDataPath, accountUtilZipName)
		if _, err := os.Stat(builtLocalDataPath); os.IsNotExist(err) {
			apkZipPath = filepath.Join(prebuiltLocalDataPath, apkZipName)
			accountUtilZipPath = filepath.Join(prebuiltLocalDataPath, accountUtilZipName)
		} else if err != nil {
			s.Fatal("Failed to check if built local data path exists: ", err)
		}
		// Setup adb and connect to the Android phone.
		adbDevice, err := nearbysetup.AdbSetup(ctx)
		if err != nil {
			s.Fatal("Failed to setup an adb device: ", err)
		}
		// We want to ensure we have logs even if the Android device setup fails.
		fixtureLogcatPath := filepath.Join(s.OutDir(), "fixture_setup_logcat.txt")
		defer adbDevice.DumpLogcat(ctx, fixtureLogcatPath)

		// Skip logging in to the test account on the Android device if specified in the runtime vars.
		// This lets you run the tests on a phone that's already signed in with your own account.
		loggedIn := false
		if val, ok := s.Var("skipAndroidLogin"); ok {
			b, err := strconv.ParseBool(val)
			if err != nil {
				s.Fatal("Unable to convert skipAndroidLogin var to bool: ", err)
			}
			loggedIn = b
		}
		androidUsername := s.RequiredVar("nearbyshare.android_username")
		androidPassword := s.RequiredVar("nearbyshare.android_password")

		// Configure the Android phone and setup the Snippet library.
		androidDevice, err := nearbysetup.AndroidSetup(
			ctx, adbDevice, accountUtilZipPath, androidUsername, androidPassword, loggedIn, apkZipPath, rooted,
			nearbysetup.DefaultScreenTimeout,
			f.androidDataUsage,
			f.androidVisibility,
			androidDisplayName,
		)
		if err != nil {
			s.Fatal("Failed to prepare connected Android device for Nearby Share testing: ", err)
		}
		f.androidDevice = androidDevice
		fixData.AndroidDevice = androidDevice
		fixData.AndroidDeviceName = androidDisplayName

		// Store Android attributes for reporting.
		androidAttributes, err := androidDevice.GetAndroidAttributes(ctx)
		if err != nil {
			s.Fatal("Failed to get Android attributes for reporting: ", err)
		}
		f.androidAttributes = androidAttributes
	}

	// Lock chrome after all Setup is complete so we don't block other fixtures.
	chrome.Lock()
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
		s.Error("Failed to start Chrome logging: ", err)
	}
	f.ChromeReader = chromeReader
	if f.androidSetup {
		if err := f.androidDevice.ClearLogcat(ctx); err != nil {
			s.Fatal("Failed to clear logcat before the test run: ", err)
		}
	}
	if err := saveDeviceAttributes(f.crosAttributes, f.androidAttributes, filepath.Join(s.OutDir(), "device_attributes.json")); err != nil {
		s.Error("Failed to save device attributes: ", err)
	}
}

func (f *nearbyShareFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	if f.ChromeReader == nil {
		s.Error("ChromeReader not defined")
	}
	if err := nearbytestutils.SaveLogs(ctx, f.ChromeReader, filepath.Join(s.OutDir(), ChromeLog)); err != nil {
		s.Error("Failed to save Chrome log: ", err)
	}
	if f.androidSetup {
		f.androidDevice.DumpLogs(ctx, s.OutDir(), "nearby_logcat.txt")
	}
}

// saveDeviceAttributes saves the CrOS and Android device attributes as a formatted JSON at the specified filepath.
func saveDeviceAttributes(crosAttrs *nearbysetup.CrosAttributes, androidAttrs *nearbysnippet.AndroidAttributes, filepath string) error {
	attributes := struct {
		CrOS    *nearbysetup.CrosAttributes
		Android *nearbysnippet.AndroidAttributes
	}{CrOS: crosAttrs, Android: androidAttrs}
	crosLog, err := json.MarshalIndent(attributes, "", "\t")
	if err != nil {
		return errors.Wrap(err, "failed to format device metadata for logging")
	}
	if err := ioutil.WriteFile(filepath, crosLog, 0644); err != nil {
		return errors.Wrap(err, "failed to write CrOS attributes to output file")
	}
	return nil
}

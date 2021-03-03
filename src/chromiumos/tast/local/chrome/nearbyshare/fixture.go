// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/nearbyshare/nearbysetup"
	"chromiumos/tast/local/chrome/nearbyshare/nearbysnippet"
	"chromiumos/tast/local/chrome/nearbyshare/nearbytestutils"
	"chromiumos/tast/local/chrome/systemlogs"
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
		Name: "nearbyShareDataUsageOfflineAllContactsTestUser",
		Desc: "Nearby Share enabled on CrOS and Android configured with 'Data Usage' set to 'Offline' and 'Visibility' set to 'All Contacts'",
		Impl: NewNearbyShareFixture(nearbysetup.DataUsageOffline, nearbysetup.VisibilityAllContacts, false, true),
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
		Impl: NewNearbyShareFixture(nearbysetup.DataUsageOffline, nearbysetup.VisibilityAllContacts, true, true),
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
}

type nearbyShareFixture struct {
	cr            *chrome.Chrome
	opts          []chrome.Option
	dataUsage     nearbysetup.DataUsage
	visibility    nearbysetup.Visibility
	gaiaLogin     bool
	androidSetup  bool
	androidDevice *nearbysnippet.AndroidNearbyDevice
	crosInfo      crosMetadata
	androidInfo   androidMetadata
	// ChromeReader is the line reader for collecting Chrome logs.
	ChromeReader *syslog.LineReader
}

type crosMetadata struct {
	CrosDisplayName string
	CrosUser        string
	CrosDataUsage   string
	CrosVisibility  string
	ChromeVersion   string
}

type androidMetadata struct {
	AndroidDisplayName        string
	AndroidUser               string
	AndroidDataUsage          string
	AndroidVisibility         string
	AndroidNearbyShareVersion string
	AndroidGMSCoreVersion     int
	AndroidVersion            int
	AndroidSDKVersion         int
	AndroidProductName        string
	AndroidModelName          string
	AndroidDeviceName         string
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
		f.opts = append(f.opts, chrome.Auth(crosUsername, crosPassword, ""), chrome.GAIALogin())
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

		androidDevice, err := nearbysetup.AndroidSetup(
			ctx, accountUtilZipPath, androidUsername, androidPassword, loggedIn,
			apkZipPath, rooted,
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

		// Store Android metadata for logging.
		f.androidInfo.AndroidDisplayName = androidDisplayName
		if loggedIn {
			f.androidInfo.AndroidUser = "Custom Android user (skipped login)"
		} else {
			f.androidInfo.AndroidUser = androidUsername
		}
		f.androidInfo.AndroidDataUsage = nearbysetup.DataUsageStrings[f.dataUsage]
		f.androidInfo.AndroidVisibility = nearbysetup.VisibilityStrings[f.visibility]
		androidNearbyShareVersion, err := androidDevice.GetNearbySharingVersion(ctx)
		if err != nil {
			s.Fatal("Failed to get Android Nearby Share version: ", err)
		}
		f.androidInfo.AndroidNearbyShareVersion = androidNearbyShareVersion
		f.androidInfo.AndroidGMSCoreVersion = androidDevice.GMSCoreVersion
		f.androidInfo.AndroidVersion = androidDevice.AndroidVersion
		f.androidInfo.AndroidSDKVersion = androidDevice.SDKVersion
		f.androidInfo.AndroidProductName = androidDevice.Product
		f.androidInfo.AndroidModelName = androidDevice.Model
		f.androidInfo.AndroidDeviceName = androidDevice.Device
	}

	// Store CrOS test metadata before returning.
	f.crosInfo.CrosDisplayName = crosDisplayName
	f.crosInfo.CrosUser = crosUsername
	f.crosInfo.CrosDataUsage = nearbysetup.DataUsageStrings[f.dataUsage]
	f.crosInfo.CrosVisibility = nearbysetup.VisibilityStrings[f.visibility]
	const expectedKey = "CHROME VERSION"
	result, err := systemlogs.GetSystemLogs(ctx, tconn, expectedKey)
	if err != nil {
		s.Fatal("Error getting system logs to check Chrome version: ", err)
	}
	if result == "" {
		s.Fatal("System logs result empty")
	}
	f.crosInfo.ChromeVersion = result

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
	s.Log("CrOS Nearby Share Configuration:")
	crosLog, err := json.MarshalIndent(f.crosInfo, "", "\t")
	if err != nil {
		s.Error("Failed to format CrOS metadata for logging: ", err)
	}
	s.Log(string(crosLog))

	if f.androidSetup {
		s.Log("Android Configuration:")
		androidLog, err := json.MarshalIndent(f.androidInfo, "", "\t")
		if err != nil {
			s.Error("Failed to format Android metadata for logging: ", err)
		}
		s.Log(string(androidLog))
	}
}

func (f *nearbyShareFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	if f.ChromeReader == nil {
		s.Error("ChromeReader not defined")
	}
	if err := nearbytestutils.SaveLogs(ctx, f.ChromeReader, filepath.Join(s.OutDir(), ChromeLog)); err != nil {
		s.Error("Failed to save Chrome log: ", err)
	}
}

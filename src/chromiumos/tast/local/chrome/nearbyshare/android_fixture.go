// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/common/cros/nearbyshare/nearbysetup"
	"chromiumos/tast/common/cros/nearbyshare/nearbytestutils"
	"chromiumos/tast/local/chrome/nearbyshare/nearbysnippet"
	"chromiumos/tast/testing"
)

// NewNearbyShareAndroid creates a fixture that sets up an Android device for Nearby Share.
func NewNearbyShareAndroid(androidDataUsage nearbysnippet.DataUsage, androidVisibility nearbysnippet.Visibility) testing.FixtureImpl {
	return &nearbyShareAndroidFixture{
		androidDataUsage:  androidDataUsage,
		androidVisibility: androidVisibility,
	}
}

func init() {
	const (
		// These are the default GAIA credentials that will be used to sign in on Android.
		defaultAndroidUsername = "nearbyshare.android_username"
		defaultAndroidPassword = "nearbyshare.android_password"

		// This is the username that we'll use for non-rooted devices in the lab.
		unrootedAndroidUsername = "nearbyshare.unrooted_android_username"

		// Specify -var=skipAndroidLogin=true if the Android device is logged in to a personal account.
		// Otherwise we will attempt removing all Google accounts and adding a test account to the phone.
		// Adding/removing accounts requires ADB root access, so this will automatically be set to true if root is not available.
		skipAndroidLogin = "skipAndroidLogin"
	)
	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareAndroidSetup",
		Desc: "Set up Android device for Nearby Share with default settings (Data usage offline, All Contacts)",
		Impl: NewNearbyShareAndroid(nearbysnippet.DataUsageOffline, nearbysnippet.VisibilityAllContacts),
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Vars: []string{
			defaultAndroidUsername,
			defaultAndroidPassword,
			unrootedAndroidUsername,
			skipAndroidLogin,
		},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})
}

type nearbyShareAndroidFixture struct {
	androidDataUsage  nearbysnippet.DataUsage
	androidVisibility nearbysnippet.Visibility
	androidDevice     *nearbysnippet.AndroidNearbyDevice
}

func (f *nearbyShareAndroidFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
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
	// Set up adb, connect to the Android phone, and check if ADB root access is available.
	adbDevice, rooted, err := nearbysetup.AdbSetup(ctx)
	if err != nil {
		s.Fatal("Failed to set up an adb device: ", err)
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

	// If the device is not rooted and skipAndroidLogin was not specified, we'll assume the test is running with an unrooted phone in the lab.
	// This only affects the username that will be saved in the device_attributes.json log.
	if !rooted && !loggedIn {
		androidUsername = s.RequiredVar("nearbyshare.unrooted_android_username")
	}

	// Configure the Android phone and set up the Snippet library.
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
	return &FixtData{
		AndroidDevice:     androidDevice,
		AndroidDeviceName: androidDisplayName,
		AndroidUsername:   androidUsername,
		AndroidLoggedIn:   loggedIn,
	}

}
func (f *nearbyShareAndroidFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if f.androidDevice != nil {
		f.androidDevice.StopSnippet(ctx)
	}
}
func (f *nearbyShareAndroidFixture) Reset(ctx context.Context) error                        { return nil }
func (f *nearbyShareAndroidFixture) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (f *nearbyShareAndroidFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nearbyfixture contains fixtures for Nearby Share tests.
package nearbyfixture

import (
	"context"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/common/android"
	nearbycommon "chromiumos/tast/common/cros/nearbyshare"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/crossdevice"
	"chromiumos/tast/local/chrome/nearbyshare/nearbysnippet"
	"chromiumos/tast/testing"
)

type channel int

const (
	prod channel = iota
	modulefood
	dev
)

const (
	// These are the default GAIA credentials that will be used to sign in on Android.
	// This account is a member of the Nearby modulefood user group, which is our default
	// choice for Android version for Nearby tests.
	defaultAndroidUsername = "nearbyshare.android_username"
	defaultAndroidPassword = "nearbyshare.android_password"

	// These are GAIA credentials that will be used to sign in on Android to test the dev version of Android Nearby Share.
	devAndroidUsername = "nearbyshare.dev_android_username"
	devAndroidPassword = "nearbyshare.dev_android_password"

	// These are GAIA credentials that will be used to sign in on Android to test the production version of Android Nearby Share.
	prodAndroidUsername = "nearbyshare.prod_android_username"
	prodAndroidPassword = "nearbyshare.prod_android_password"

	// This is the username that we'll use for non-rooted devices in the lab.
	unrootedAndroidUsername = "nearbyshare.unrooted_android_username"

	// Specify -var=skipAndroidLogin=true if the Android device is logged in to a personal account.
	// Otherwise we will attempt removing all Google accounts and adding a test account to the phone.
	// Adding/removing accounts requires ADB root access, so this will automatically be set to true if root is not available.
	skipAndroidLogin = "skipAndroidLogin"
)

// NewNearbyShareAndroid creates a fixture that sets up an Android device for Nearby Share.
func NewNearbyShareAndroid(androidDataUsage nearbysnippet.DataUsage, androidVisibility nearbysnippet.Visibility, androidNearbyChannel channel) testing.FixtureImpl {
	return &nearbyShareAndroidFixture{
		androidDataUsage:     androidDataUsage,
		androidVisibility:    androidVisibility,
		androidNearbyChannel: androidNearbyChannel,
	}
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareAndroidSetup",
		Desc: "Set up Android device for Nearby Share with default settings (Data usage offline, All Contacts)",
		Impl: NewNearbyShareAndroid(nearbysnippet.DataUsageOffline, nearbysnippet.VisibilityAllContacts, modulefood),
		Data: []string{nearbysnippet.ZipName, crossdevice.AccountUtilZip},
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

	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareAndroidSetupDev",
		Desc: "Set up Android device for Nearby Share with default settings (Data usage offline, All Contacts), using the dev version of Nearby",
		Impl: NewNearbyShareAndroid(nearbysnippet.DataUsageOffline, nearbysnippet.VisibilityAllContacts, dev),
		Data: []string{nearbysnippet.ZipName, crossdevice.AccountUtilZip},
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Vars: []string{
			devAndroidUsername,
			devAndroidPassword,
			unrootedAndroidUsername,
			skipAndroidLogin,
		},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareAndroidSetupProd",
		Desc: "Set up Android device for Nearby Share with default settings (Data usage offline, All Contacts), using the prod version of Nearby",
		Impl: NewNearbyShareAndroid(nearbysnippet.DataUsageOffline, nearbysnippet.VisibilityAllContacts, prod),
		Data: []string{nearbysnippet.ZipName, crossdevice.AccountUtilZip},
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Vars: []string{
			prodAndroidUsername,
			prodAndroidPassword,
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
	androidDataUsage     nearbysnippet.DataUsage
	androidVisibility    nearbysnippet.Visibility
	androidDevice        *nearbysnippet.AndroidNearbyDevice
	androidNearbyChannel channel
}

func (f *nearbyShareAndroidFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	const androidBaseName = "android_test"
	androidDisplayName := nearbycommon.RandomDeviceName(androidBaseName)
	snippetZip := s.DataPath(nearbysnippet.ZipName)
	accountUtilZip := s.DataPath(crossdevice.AccountUtilZip)

	// Set up adb, connect to the Android phone, and check if ADB root access is available.
	adbDevice, rooted, err := crossdevice.AdbSetup(ctx)
	if err != nil {
		s.Fatal("Failed to set up an adb device: ", err)
	}
	// We want to ensure we have logs even if the Android device setup fails.
	fixtureLogcatPath := filepath.Join(s.OutDir(), "fixture_setup_logcat.txt")
	defer adbDevice.DumpLogcat(ctx, fixtureLogcatPath)

	if err := crossdevice.ConfigureDevice(ctx, adbDevice, rooted); err != nil {
		s.Fatal("Failed to do basic Android device preparation: ", err)
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

	var androidUsername string
	var androidPassword string
	switch f.androidNearbyChannel {
	case modulefood:
		androidUsername = s.RequiredVar(defaultAndroidUsername)
		androidPassword = s.RequiredVar(defaultAndroidPassword)
	case prod:
		androidUsername = s.RequiredVar(prodAndroidUsername)
		androidPassword = s.RequiredVar(prodAndroidPassword)
	case dev:
		androidUsername = s.RequiredVar(devAndroidUsername)
		androidPassword = s.RequiredVar(devAndroidPassword)
	}

	// If the device is not rooted and skipAndroidLogin was not specified, we'll assume the test is running with an unrooted phone in the lab.
	// This only affects the username that will be saved in the device_attributes.json log.
	if !rooted && !loggedIn {
		androidUsername = s.RequiredVar("nearbyshare.unrooted_android_username")
	}

	// Remove and re-add the specified account. A GAIA login is required to configure Nearby Share on the Android device.
	// Root access is required for adding and removing accounts.
	if !loggedIn && rooted {
		if err := crossdevice.GAIALogin(ctx, adbDevice, accountUtilZip, androidUsername, androidPassword); err != nil {
			s.Fatal("Failed to log in on the Android device: ", err)
		}
	}

	tags := []string{
		"Nearby",
		"NearbyMessages",
		"NearbyDiscovery",
		"NearbyConnections",
		"NearbyMediums",
		"NearbySetup",
		"NearbySharing",
		"NearbyDirect",
		"Backup",
		"SmartDevice",
		"audioModem",
	}
	if err := crossdevice.EnableVerboseLogging(ctx, adbDevice, rooted, tags...); err != nil {
		s.Fatal("Failed to enable verbose logging on Android: ", err)
	}

	// Clear the Android's default directory for receiving shares.
	if err := adbDevice.RemoveContents(ctx, android.DownloadDir); err != nil {
		s.Fatal("Failed to clear Android downloads directory: ", err)
	}

	// Launch and start the snippet server. Don't override GMS Core flags if specified in the runtime vars.
	androidDevice, err := nearbysnippet.New(ctx, adbDevice, snippetZip, rooted)
	if err != nil {
		s.Fatal("Failed to set up the Nearby snippet server: ", err)
	}

	if err := configureAndroidNearbySettings(ctx, androidDevice, f.androidDataUsage, f.androidVisibility, androidDisplayName); err != nil {
		s.Fatal("Failed to configure Android Nearby Share settings: ", err)
	}

	f.androidDevice = androidDevice
	return &FixtData{
		AndroidDevice:        androidDevice,
		AndroidDeviceName:    androidDisplayName,
		AndroidUsername:      androidUsername,
		AndroidLoggedIn:      loggedIn,
		AndroidNearbyChannel: f.androidNearbyChannel,
	}

}
func (f *nearbyShareAndroidFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if f.androidDevice != nil {
		f.androidDevice.Cleanup(ctx)
	}
}
func (f *nearbyShareAndroidFixture) Reset(ctx context.Context) error                        { return nil }
func (f *nearbyShareAndroidFixture) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (f *nearbyShareAndroidFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

// configureAndroidNearbySettings configures Nearby Share settings on an Android device.
func configureAndroidNearbySettings(ctx context.Context, androidNearby *nearbysnippet.AndroidNearbyDevice, dataUsage nearbysnippet.DataUsage, visibility nearbysnippet.Visibility, name string) error {
	// Ensure Nearby is disabled to avoid race conditions or starting up in an invalid state after the device is set up.
	androidNearby.SetEnabled(ctx, false)
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep after setting Nearby disabld via snippets")
	}

	if err := androidNearby.SetupDevice(ctx, dataUsage, visibility, name); err != nil {
		return errors.Wrap(err, "failed to configure Android Nearby Share settings")
	}

	// androidNearby.SetupDevice is asynchronous, so we need to poll until the settings changes have taken effect.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if n, err := androidNearby.GetDeviceName(ctx); err != nil {
			return testing.PollBreak(err)
		} else if n != name {
			return errors.Errorf("current device name (%v) not yet updated to %v", n, name)
		}

		if v, err := androidNearby.GetVisibility(ctx); err != nil {
			return testing.PollBreak(err)
		} else if v != visibility {
			return errors.Errorf("current visibility (%v) not yet updated to %v", v, visibility)
		}

		if d, err := androidNearby.GetDataUsage(ctx); err != nil {
			return testing.PollBreak(err)
		} else if d != dataUsage {
			return errors.Errorf("current data usage (%v) not yet updated to %v", d, dataUsage)
		}

		return nil
	}, &testing.PollOptions{Interval: 2 * time.Second, Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "timed out waiting for Nearby Share settings to update")
	}

	if visibility != nearbysnippet.VisibilityNoOne {
		// Force-sync after changing Nearby settings to ensure the phone's certificates are regenerated and uploaded.
		if err := androidNearby.Sync(ctx); err != nil {
			return errors.Wrap(err, "failed to sync contacts and certificates")
		}
	}

	return nil
}

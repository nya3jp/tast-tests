// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package multidevice

import (
	"context"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/testing"
)

// NewMultideviceAndroid creates a fixture that sets up an Android device for multidevice feature testing.
func NewMultideviceAndroid() testing.FixtureImpl {
	return &multideviceAndroidFixture{}
}

// resetTimeout is the timeout duration to trying reset of the current fixture.
const resetTimeout = 30 * time.Second

func init() {
	const (
		// These are the default GAIA credentials that will be used to sign in on Android for Multidevice suite tests.
		defaultMultideviceUsername = "nearbyshare.android_username"
		defaultMultidevicePassword = "nearbyshare.android_password"

		// This is the username that we'll use for non-rooted devices in the lab.
		unrootedAndroidUsername = "nearbyshare.unrooted_android_username"

		// Specify -var=skipAndroidLogin=true if the Android device is logged in to a personal account.
		// Otherwise we will attempt removing all Google accounts and adding a test account to the phone.
		// Adding/removing accounts requires ADB root access, so this will automatically be set to true if root is not available.
		skipAndroidLogin = "skipAndroidLogin"
	)
	testing.AddFixture(&testing.Fixture{
		Name: "multideviceAndroidSetup",
		Desc: "Set up Android device for Nearby Share with default settings (Data usage offline, All Contacts)",
		Impl: NewMultideviceAndroid(),
		Data: []string{AccountUtilZip, SnippetZipName},
		Contacts: []string{
			"kyleshima@chromium.org",
			"chromeos-sw-engprod@google.com",
		},
		Vars: []string{
			defaultMultideviceUsername,
			defaultMultidevicePassword,
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

type multideviceAndroidFixture struct {
	connectedDevice *ConnectedAndroidDevice
}

func (f *multideviceAndroidFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	accountUtilZip := s.DataPath(AccountUtilZip)
	snippetZip := s.DataPath(SnippetZipName)

	// Set up adb, connect to the Android phone, and check if ADB root access is available.
	adbDevice, rooted, err := AdbSetup(ctx)
	if err != nil {
		s.Fatal("Failed to set up an adb device: ", err)
	}
	// We want to ensure we have logs even if the Android device setup fails.
	fixtureLogcatPath := filepath.Join(s.OutDir(), "android_base_fixture_logcat.txt")
	defer adbDevice.DumpLogcat(ctx, fixtureLogcatPath)

	// Do some basic device set up like waking the screen and clearing logcat.
	if err := ConfigureDevice(ctx, adbDevice, rooted); err != nil {
		s.Fatal("Failed to prepare the Android device: ", err)
	}

	// Enable verbose logging for related modules.
	if err := EnableVerboseLogging(ctx, adbDevice, rooted); err != nil {
		s.Fatal("Failed to enable verbose logs: ", err)
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

	if !loggedIn {
		if rooted {
			if err := GAIALogin(ctx, adbDevice, accountUtilZip, androidUsername, androidPassword); err != nil {
				s.Fatal("Failed to log in on the Android device: ", err)
			}
		} else {
			// If the device is not rooted and skipAndroidLogin was not specified, we'll assume the test is running with an unrooted phone in the lab.
			// This only affects the username that will be saved in the device_attributes.json log.
			androidUsername = s.RequiredVar("nearbyshare.unrooted_android_username")
		}
	}

	// Prepare the Multidevice Snippet.
	connectedDevice, err := NewConnectedAndroidDevice(ctx, adbDevice, snippetZip)
	if err != nil {
		s.Fatal("Failed to prepare connected Android device for Multidevice testing: ", err)
	}
	f.connectedDevice = connectedDevice
	return &FixtData{
		ConnectedDevice: connectedDevice,
	}
}
func (f *multideviceAndroidFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if f.connectedDevice != nil {
		f.connectedDevice.Cleanup(ctx)
	}
}
func (f *multideviceAndroidFixture) Reset(ctx context.Context) error                        { return nil }
func (f *multideviceAndroidFixture) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (f *multideviceAndroidFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

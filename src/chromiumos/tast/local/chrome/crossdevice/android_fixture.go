// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crossdevice

import (
	"context"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/testing"
)

// NewCrossDeviceAndroid creates a fixture that sets up an Android device for crossdevice feature testing.
func NewCrossDeviceAndroid() testing.FixtureImpl {
	return &crossdeviceAndroidFixture{}
}

// resetTimeout is the timeout duration to trying reset of the current fixture.
const resetTimeout = 30 * time.Second

// Runtime variable names.
const (
	// These are the default GAIA credentials that will be used to sign in on Android for crossdevice tests.
	defaultCrossDeviceUsername = "crossdevice.username"
	defaultCrossDevicePassword = "crossdevice.password"

	// Specify -var=skipAndroidLogin=true if the Android device is logged in to a personal account.
	// Otherwise we will attempt removing all Google accounts and adding a test account to the phone.
	// Adding/removing accounts requires ADB root access, so this will automatically be set to true if root is not available.
	skipAndroidLogin = "skipAndroidLogin"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "crossdeviceAndroidSetup",
		Desc: "Set up Android device for CrOS crossdevice testing",
		Impl: NewCrossDeviceAndroid(),
		Data: []string{AccountUtilZip, MultideviceSnippetZipName},
		Contacts: []string{
			"kyleshima@chromium.org",
			"chromeos-sw-engprod@google.com",
		},
		Vars: []string{
			defaultCrossDeviceUsername,
			defaultCrossDevicePassword,
			skipAndroidLogin,
		},
		SetUpTimeout:    3 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})
}

type crossdeviceAndroidFixture struct {
	adbDevice     *adb.Device
	androidDevice *AndroidDevice
}

func (f *crossdeviceAndroidFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	accountUtilZip := s.DataPath(AccountUtilZip)
	snippetZip := s.DataPath(MultideviceSnippetZipName)

	// Set up adb, connect to the Android phone, and check if ADB root access is available.
	adbDevice, rooted, err := AdbSetup(ctx)
	if err != nil {
		s.Fatal("Failed to set up an adb device: ", err)
	}
	f.adbDevice = adbDevice
	// We want to ensure we have logs even if the Android device setup fails.
	fixtureLogcatPath := filepath.Join(s.OutDir(), "android_base_fixture_logcat.txt")
	defer adbDevice.DumpLogcat(ctx, fixtureLogcatPath)

	// Do some basic device set up like waking the screen and clearing logcat.
	if err := ConfigureDevice(ctx, adbDevice, rooted); err != nil {
		s.Fatal("Failed to prepare the Android device: ", err)
	}

	// Enable verbose logging for related modules.
	tags := []string{"ProximityAuth", "CryptauthV2", "NearbyConnections", "NearbyMediums"}
	if err := EnableVerboseLogging(ctx, adbDevice, rooted, tags...); err != nil {
		s.Fatal("Failed to enable verbose logs: ", err)
	}

	// Skip logging in to the test account on the Android device if specified in the runtime vars.
	// This lets you run the tests on a phone that's already signed in with your own account.
	loggedIn := false
	if val, ok := s.Var(skipAndroidLogin); ok {
		b, err := strconv.ParseBool(val)
		if err != nil {
			s.Fatal("Unable to convert skipAndroidLogin var to bool: ", err)
		}
		loggedIn = b
	}
	androidUsername := s.RequiredVar(defaultCrossDeviceUsername)
	androidPassword := s.RequiredVar(defaultCrossDevicePassword)

	if !loggedIn {
		if rooted {
			if err := GAIALogin(ctx, adbDevice, accountUtilZip, androidUsername, androidPassword); err != nil {
				s.Fatal("Failed to log in on the Android device: ", err)
			}
		} else {
			s.Fatal("Cannot log in on Android on an unrooted phone")
		}
	}

	// Prepare the Multidevice Snippet.
	androidDevice, err := NewAndroidDevice(ctx, adbDevice, snippetZip)
	if err != nil {
		s.Fatal("Failed to prepare connected Android device for Multidevice testing: ", err)
	}
	f.androidDevice = androidDevice
	return &FixtData{
		AndroidDevice: androidDevice,
	}
}
func (f *crossdeviceAndroidFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if f.androidDevice != nil {
		f.androidDevice.Cleanup(ctx)
	}
	if err := RemoveAccounts(ctx, f.adbDevice); err != nil {
		s.Log("Failed to remove accounts from the Android device: ", err)
	}
}
func (f *crossdeviceAndroidFixture) Reset(ctx context.Context) error                        { return nil }
func (f *crossdeviceAndroidFixture) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (f *crossdeviceAndroidFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

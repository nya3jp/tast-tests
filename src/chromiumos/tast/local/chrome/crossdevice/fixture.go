// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crossdevice

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/crossdevice/phonehub"
	"chromiumos/tast/testing"
)

// NewCrossDeviceOnboarded creates a fixture that logs in to CrOS, pairs it with an Android device,
// and ensures the features in the "Connected devices" section of OS Settings are ready to use (Smart Lock, Phone Hub, etc.).
// Note that crossdevice fixtures inherit from crossdeviceAndroidSetup.
func NewCrossDeviceOnboarded() testing.FixtureImpl {
	defaultOpts := []chrome.Option{
		chrome.ExtraArgs("--enable-logging", "--vmodule=*blue*=1", "--vmodule=*nearby*=1"),
	}
	return &crossdeviceFixture{
		opts: defaultOpts,
	}
}

// Fixture runtime variables.
const (
	// These vars can be used from the command line when running tests locally to configure the tests to run on personal GAIA accounts.
	// Use these vars to log in with your own GAIA credentials on CrOS. The Android device should be signed in with the same account.
	customCrOSUsername = "cros_username"
	customCrOSPassword = "cros_password"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "crossdeviceOnboarded",
		Desc: "User is signed in (with GAIA) to CrOS and paired with an Android phone",
		Contacts: []string{
			"kyleshima@chromium.org",
			"chromeos-sw-engprod@google.com",
		},
		Parent: "crossdeviceAndroidSetup",
		Impl:   NewCrossDeviceOnboarded(),
		Vars: []string{
			defaultCrossDeviceUsername,
			defaultCrossDevicePassword,
			customCrOSUsername,
			customCrOSPassword,
			KeepStateVar,
		},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})
}

type crossdeviceFixture struct {
	opts              []chrome.Option
	cr                *chrome.Chrome
	androidAttributes *AndroidAttributes
	crosAttributes    *CrosAttributes
}

// FixtData holds information made available to tests that specify this Fixture.
type FixtData struct {
	// Chrome is the running chrome instance.
	Chrome *chrome.Chrome

	// TestConn is a connection to the test extension.
	TestConn *chrome.TestConn

	// AndroidDevice is an object for interacting with the connected Android device's Multidevice Snippet.
	AndroidDevice *AndroidDevice
}

func (f *crossdeviceFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	// Android device from parent fixture.
	androidDevice := s.ParentValue().(*FixtData).AndroidDevice

	// Reset and save logcat so we have Android logs even if fixture setup fails.
	if err := androidDevice.ClearLogcat(ctx); err != nil {
		s.Fatal("Failed to clear logcat at start of fixture setup")
	}
	defer androidDevice.DumpLogs(ctx, s.OutDir(), "fixture_setup_logcat.txt")

	crosUsername := s.RequiredVar(defaultCrossDeviceUsername)
	crosPassword := s.RequiredVar(defaultCrossDevicePassword)
	customUser, userOk := s.Var(customCrOSUsername)
	customPass, passOk := s.Var(customCrOSPassword)
	if userOk && passOk {
		s.Log("Logging in with user-provided credentials")
		crosUsername = customUser
		crosPassword = customPass
	} else {
		s.Log("Logging in with default GAIA credentials")
	}
	f.opts = append(f.opts, chrome.GAIALogin(chrome.Creds{User: crosUsername, Pass: crosPassword}))

	if val, ok := s.Var(KeepStateVar); ok {
		b, err := strconv.ParseBool(val)
		if err != nil {
			s.Fatalf("Unable to convert %v var to bool: %v", KeepStateVar, err)
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

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Sometimes during login the tcp connection to the snippet server on Android is lost.
	// If the Connect RPC fails, reconnect to the snippet server and try again.
	if err := androidDevice.Pair(ctx); err != nil {
		s.Log("Lost connection to the Snippet server. Reconnecting")
		if err := androidDevice.ReconnectToSnippet(ctx); err != nil {
			s.Fatal("Failed to reconnect to the snippet server: ", err)
		}
		if err := androidDevice.Pair(ctx); err != nil {
			s.Fatal("Failed to connect the Android device to CrOS: ", err)
		}
	}

	// Wait for the "Smart Lock is turned on" notification to appear,
	// since it will cause Phone Hub to close if it's open before the notification pops up.
	if _, err := ash.WaitForNotification(ctx, tconn, 30*time.Second, ash.WaitTitleContains("Smart Lock is turned on")); err != nil {
		s.Log("Smart Lock notification did not appear after 30 seconds, proceeding anyways")
	}

	if err := phonehub.Enable(ctx, tconn, cr); err != nil {
		s.Fatal("Failed to enable Phone Hub: ", err)
	}

	// Store Android attributes for reporting.
	androidAttributes, err := androidDevice.GetAndroidAttributes(ctx)
	if err != nil {
		s.Fatal("Failed to get Android attributes for reporting: ", err)
	}
	f.androidAttributes = androidAttributes

	// Store CrOS test metadata for reporting.
	crosAttributes, err := GetCrosAttributes(ctx, tconn, crosUsername)
	if err != nil {
		s.Fatal("Failed to get CrOS attributes for reporting: ", err)
	}
	f.crosAttributes = crosAttributes

	return &FixtData{
		Chrome:        cr,
		TestConn:      tconn,
		AndroidDevice: androidDevice,
	}
}

func (f *crossdeviceFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()
	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	f.cr = nil
}
func (f *crossdeviceFixture) Reset(ctx context.Context) error {
	if err := f.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}
	return nil
}
func (f *crossdeviceFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	if err := saveDeviceAttributes(f.crosAttributes, f.androidAttributes, filepath.Join(s.OutDir(), "device_attributes.json")); err != nil {
		s.Error("Failed to save device attributes: ", err)
	}
}
func (f *crossdeviceFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

// saveDeviceAttributes saves the CrOS and Android device attributes as a formatted JSON at the specified filepath.
func saveDeviceAttributes(crosAttrs *CrosAttributes, androidAttrs *AndroidAttributes, filepath string) error {
	attributes := struct {
		CrOS    *CrosAttributes
		Android *AndroidAttributes
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

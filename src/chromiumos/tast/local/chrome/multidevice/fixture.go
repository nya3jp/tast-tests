// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package multidevice

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
	"chromiumos/tast/local/chrome/multidevice/phonehub"
	"chromiumos/tast/testing"
)

// NewMultidevice creates a fixture that logs in and configures a connected Android device.
// Note that multidevice fixtures inherit from multideviceAndroidSetup.
func NewMultidevice() testing.FixtureImpl {
	defaultOpts := []chrome.Option{
		chrome.ExtraArgs("--enable-logging", "--vmodule=*blue*=1", "--vmodule=*nearby*=1"),
	}
	return &multideviceFixture{
		opts: defaultOpts,
	}
}

func init() {
	const (
		// These are the default GAIA credentials that will be used to sign in on Android for Multidevice suite tests.
		defaultMultideviceUsername = "nearbyshare.android_username"
		defaultMultidevicePassword = "nearbyshare.android_password"

		// These vars can be used from the command line when running tests locally to configure the tests to run on personal GAIA accounts.
		// Use these vars to log in with your own GAIA credentials. If running in-contacts tests with an Android device, it is expected that the CrOS user and Android user are already mutual contacts.
		customCrOSUsername = "cros_username"
		customCrOSPassword = "cros_password"

		// Set this var to True to prevent the tests from clearing existing user accounts from the DUT.
		keepState = KeepStateVar
	)

	testing.AddFixture(&testing.Fixture{
		Name: "multidevice",
		Desc: "User is signed in (with GAIA) to CrOS and paired with an Android phone",
		Contacts: []string{
			"kyleshima@chromium.org",
			"chromeos-sw-engprod@google.com",
		},
		Parent: "multideviceAndroidSetup",
		Impl:   NewMultidevice(),
		Vars: []string{
			defaultMultideviceUsername,
			defaultMultidevicePassword,
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

type multideviceFixture struct {
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

	// ConnectedDevice is an object for interacting with the connected Android device's Multidevice Snippet.
	ConnectedDevice *ConnectedAndroidDevice
}

func (f *multideviceFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	// Android device info from parent fixture.
	connectedDevice := s.ParentValue().(*FixtData).ConnectedDevice

	// Reset and save logcat so we have Android logs even if fixture setup fails.
	if err := connectedDevice.ClearLogcat(ctx); err != nil {
		s.Fatal("Failed to clear logcat at start of fixture setup")
	}
	defer connectedDevice.DumpLogs(ctx, s.OutDir(), "fixture_setup_logcat.txt")

	crosUsername := s.RequiredVar("nearbyshare.android_username")
	crosPassword := s.RequiredVar("nearbyshare.android_password")
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
	const connectTimeout time.Duration = 30 * time.Second
	if err := connectedDevice.Connect(ctx, connectTimeout); err != nil {
		s.Log("Lost connection to the Snippet server. Reconnecting")
		if err := connectedDevice.ReconnectToSnippet(ctx); err != nil {
			s.Fatal("Failed to reconnect to the snippet server: ", err)
		}
		if err := connectedDevice.Connect(ctx, connectTimeout); err != nil {
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
	androidAttributes, err := connectedDevice.GetAndroidAttributes(ctx)
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
		Chrome:          cr,
		TestConn:        tconn,
		ConnectedDevice: connectedDevice,
	}
}

func (f *multideviceFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()
	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	f.cr = nil
}
func (f *multideviceFixture) Reset(ctx context.Context) error {
	if err := f.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}
	return nil
}
func (f *multideviceFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	if err := saveDeviceAttributes(f.crosAttributes, f.androidAttributes, filepath.Join(s.OutDir(), "device_attributes.json")); err != nil {
		s.Error("Failed to save device attributes: ", err)
	}
}
func (f *multideviceFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

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

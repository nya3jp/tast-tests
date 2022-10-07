// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "chromeEnterOobeHidDetection",
		Desc: "Enter Chrome OOBE HID Detection screen",
		Contacts: []string{
			"andrewdear@google.com",
			"cros-connectivity@google.com",
		},
		Vars:            []string{"ui.signinProfileTestExtensionManifestKey"},
		Impl:            newChromeOobeHidDetection(),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
}

// newChromeOobeHidDetection creates an instance of OOBE HID detection Fixture.
func newChromeOobeHidDetection() testing.FixtureImpl {
	return &ChromeOobeHidDetection{}
}

// ChromeOobeHidDetection holds fields required for this Fixture.
type ChromeOobeHidDetection struct {
	Chrome *chrome.Chrome
}

// SetUp the necessary flags while creating a Chrome instance.
func (f *ChromeOobeHidDetection) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	var chromeOpts []chrome.Option = []chrome.Option{
		chrome.NoLogin(),
		chrome.EnableHIDScreenOnOOBE(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
		chrome.EnableFeatures(
			"BluetoothRevamp",
			"OobeHidDetectionRevamp",
		),
	}
	cr, err := chrome.New(ctx, chromeOpts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	f.Chrome = cr
	fixtData := FixtData{
		Chrome: cr,
	}
	return fixtData
}

// TearDown is called by the framework to tear down the environment SetUp set up.
func (f *ChromeOobeHidDetection) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := f.Chrome.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	f.Chrome = nil
}

// Reset is called by the framework after each test (except for the last one) to do a
// light-weight reset of the environment to the original state.
func (f *ChromeOobeHidDetection) Reset(ctx context.Context) error {
	if err := f.Chrome.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	if err := f.Chrome.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}
	return nil
}

// PreTest is called by the framework before each test to do a light-weight set up for the test.
func (f *ChromeOobeHidDetection) PreTest(ctx context.Context, s *testing.FixtTestState) {}

// PostTest is called by the framework after each test to tear down changes PreTest made.
func (f *ChromeOobeHidDetection) PostTest(ctx context.Context, s *testing.FixtTestState) {}

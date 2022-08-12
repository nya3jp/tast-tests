// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "chromeLoggedInWithBluetoothRevampEnabled",
		Desc: "Logs into a user session with the BluetoothRevamp feature flag enabled",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Impl:            ChromeLoggedInWithFeatures([]string{"BluetoothRevamp"}, []string{}),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "chromeLoggedInWithFlossEnabled",
		Desc: "Logs into a user session with the BluetoothRevamp and Floss feature flags enabled",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Impl:            ChromeLoggedInWithFeatures([]string{"BluetoothRevamp", "BluetoothUseFloss"}, []string{}),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "chromeLoggedInWithBluetoothRevampDisabled",
		Desc: "Logs into a user session with the BluetoothRevamp feature flag disabled",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Impl:            ChromeLoggedInWithFeatures([]string{}, []string{"BluetoothRevamp"}),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "chromeLoggedInWithBluetoothEnabled",
		Desc: "Logs into a user session and enables Bluetooth during set up and tear down",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Impl:            &chromeLoggedInWithBluetoothEnabled{},
		Parent:          "chromeLoggedInWithBluetoothRevampEnabled",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "chromeLoggedInWithFlossAndBluetoothEnabled",
		Desc: "Logs into a user session and enables Bluetooth during set up and tear down",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Impl:            &chromeLoggedInWithBluetoothEnabled{},
		Parent:          "chromeLoggedInWithFlossEnabled",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
}

// ChromeLoggedInWithFeatures returns a fixture implementation that builds on
// the existing chromeLoggedIn fixture to also enable or disable the
// BluetoothRevamp feature flag.
func ChromeLoggedInWithFeatures(enableFeatures, disableFeatures []string) testing.FixtureImpl {
	return chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
		return []chrome.Option{
			chrome.EnableFeatures(enableFeatures...), chrome.DisableFeatures(disableFeatures...),
		}, nil
	})
}

type chromeLoggedInWithBluetoothEnabled struct {
}

func (*chromeLoggedInWithBluetoothEnabled) Reset(ctx context.Context) error {
	if err := bluetooth.Enable(ctx); err != nil {
		return errors.Wrap(err, "failed to enable Bluetooth")
	}
	return nil
}

func (*chromeLoggedInWithBluetoothEnabled) PreTest(ctx context.Context, s *testing.FixtTestState) {
}

func (*chromeLoggedInWithBluetoothEnabled) PostTest(ctx context.Context, s *testing.FixtTestState) {
}

func (*chromeLoggedInWithBluetoothEnabled) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	if err := bluetooth.Enable(ctx); err != nil {
		s.Fatal("Failed to enable Bluetooth: ", err)
	}
	return s.ParentValue()
}

func (*chromeLoggedInWithBluetoothEnabled) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := bluetooth.Enable(ctx); err != nil {
		s.Fatal("Failed to enable Bluetooth: ", err)
	}
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/bluetooth/floss"
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
		Impl:            chromeLoggedInWithFeatures([]string{"BluetoothRevamp"}, []string{}),
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
		Impl:            chromeLoggedInWithFeatures([]string{"BluetoothRevamp", "Floss"}, []string{}),
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
		Impl:            chromeLoggedInWithFeatures([]string{}, []string{"BluetoothRevamp"}),
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
		Impl:            &ChromeLoggedInWithBluetoothEnabled{Impl: &bluetooth.BlueZ{}},
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
		Impl:            &ChromeLoggedInWithBluetoothEnabled{Impl: &floss.Floss{}},
		Parent:          "chromeLoggedInWithFlossEnabled",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
}

func chromeLoggedInWithFeatures(enableFeatures, disableFeatures []string) testing.FixtureImpl {
	return chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
		return []chrome.Option{
			chrome.EnableFeatures(enableFeatures...), chrome.DisableFeatures(disableFeatures...),
		}, nil
	})
}

// ChromeLoggedInWithBluetoothEnabled provides an interface for Bluetooth tests
// that provides both access to the Chrome session and a Bluetooth
// implementation.
type ChromeLoggedInWithBluetoothEnabled struct {
	Chrome *chrome.Chrome
	Impl   bluetooth.Bluetooth
}

func (f *ChromeLoggedInWithBluetoothEnabled) Reset(ctx context.Context) error {
	if err := f.Impl.Enable(ctx); err != nil {
		return errors.Wrap(err, "failed to enable Bluetooth")
	}
	return nil
}

func (*ChromeLoggedInWithBluetoothEnabled) PreTest(ctx context.Context, s *testing.FixtTestState) {
}

func (*ChromeLoggedInWithBluetoothEnabled) PostTest(ctx context.Context, s *testing.FixtTestState) {
}

func (f *ChromeLoggedInWithBluetoothEnabled) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	f.Chrome = s.ParentValue().(*chrome.Chrome)
	if err := f.Impl.Enable(ctx); err != nil {
		s.Fatal("Failed to enable Bluetooth: ", err)
	}
	return f
}

func (f *ChromeLoggedInWithBluetoothEnabled) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := f.Impl.Enable(ctx); err != nil {
		s.Fatal("Failed to enable Bluetooth: ", err)
	}
}

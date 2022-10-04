// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bluetooth/bluez"
	"chromiumos/tast/local/bluetooth/floss"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "chromeLoggedInWithBlueZ",
		Desc: "Logs into a user session with the Floss feature flag disabled",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Impl:            chromeLoggedInWithFeatures([]string{}, []string{"Floss"}),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "chromeLoggedInWithFloss",
		Desc: "Logs into a user session with the Floss feature flag enabled",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Impl:            chromeLoggedInWithFeatures([]string{"Floss"}, []string{}),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "bluetoothEnabledWithBlueZ",
		Desc: "Logs into Chrome with Floss disabled, and enables Bluetooth during set up and tear down",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Impl:            &ChromeLoggedInWithBluetoothEnabled{Impl: &bluez.BlueZ{}},
		Parent:          "chromeLoggedInWithBlueZ",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "bluetoothEnabledWithFloss",
		Desc: "Logs into Chrome with Floss enabled, and enables Bluetooth during set up and tear down",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Impl:            &ChromeLoggedInWithBluetoothEnabled{Impl: &floss.Floss{}},
		Parent:          "chromeLoggedInWithFloss",
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
	Impl   Bluetooth
}

// Reset is called between tests to reset state.
func (f *ChromeLoggedInWithBluetoothEnabled) Reset(ctx context.Context) error {
	if err := f.Impl.Enable(ctx); err != nil {
		return errors.Wrap(err, "failed to enable Bluetooth")
	}
	return nil
}

// PreTest is called before each test to perform required setup.
func (*ChromeLoggedInWithBluetoothEnabled) PreTest(ctx context.Context, s *testing.FixtTestState) {
}

// PostTest is called after each test to perform required cleanup.
func (*ChromeLoggedInWithBluetoothEnabled) PostTest(ctx context.Context, s *testing.FixtTestState) {
}

// SetUp is called before any tests using this fixture are run to perform fixture setup.
func (f *ChromeLoggedInWithBluetoothEnabled) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	f.Chrome = s.ParentValue().(*chrome.Chrome)
	if err := f.Impl.Enable(ctx); err != nil {
		s.Fatal("Failed to enable Bluetooth: ", err)
	}
	return f
}

// TearDown is called after all tests using this fixture have run to perform fixture cleanup.
func (f *ChromeLoggedInWithBluetoothEnabled) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := f.Impl.Enable(ctx); err != nil {
		s.Fatal("Failed to enable Bluetooth: ", err)
	}
}

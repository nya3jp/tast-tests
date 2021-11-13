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
		Name: "chromeLoggedInWithBluetoothRevamp",
		Desc: "Logs into a user session with the BluetoothRevamp feature flag enabled",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Impl:            ChromeLoggedInWithBluetoothRevampEnabled(),
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
		Parent:          "chromeLoggedInWithBluetoothRevamp",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
}

// ChromeLoggedInWithBluetoothRevampEnabled returns a fixture implementation
// that builds on the existing chromeLoggedIn fixture to also enable the
// BluetoothRevamp feature flag.
func ChromeLoggedInWithBluetoothRevampEnabled() testing.FixtureImpl {
	return chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
		return []chrome.Option{chrome.EnableFeatures("BluetoothRevamp")}, nil
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

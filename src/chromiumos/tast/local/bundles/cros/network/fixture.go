// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "chromeLoggedInWithNetworkRevampEnabled",
		Desc: "Logs into a user session with the QuickSettingsNetworkRevamp and BluetoothRevamp feature flag enabled",
		Contacts: []string{
			"tjohnsonkanu@google.com",
			"cros-connectivity@google.com",
		},
		Impl:            ChromeLoggedInWithNetworkRevampEnabled(),
		Parent:          "chromeLoggedInWithBluetoothEnabled",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
}

// ChromeLoggedInWithNetworkRevampEnabled returns a fixture implementation
// that builds on the existing chromeLoggedInWithBluetoothEnabled fixture to
// enable QuickSettingsNetworkRevamp and BluetoothRevamp flags and turn
// Bluetooth device on.
func ChromeLoggedInWithNetworkRevampEnabled() testing.FixtureImpl {
	return chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
		return []chrome.Option{chrome.EnableFeatures("QuickSettingsNetworkRevamp")}, nil
	})
}

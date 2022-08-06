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
			"tjohnsonkanu@chromium.org",
			"cros-connectivity@google.com",
		},
		Impl:            ChromeLoggedInWithNetworkRevampEnabled(),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
}

// ChromeLoggedInWithNetworkRevampEnabled returns a fixture
// implementation that builds on the existing chromeLoggedIn fixture to enable
// QuickSettingsNetworkRevamp and BluetoothRevamp flags.
func ChromeLoggedInWithNetworkRevampEnabled() testing.FixtureImpl {
	return chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
		return []chrome.Option{chrome.EnableFeatures("QuickSettingsNetworkRevamp", "BluetoothRevamp")}, nil
	})
}

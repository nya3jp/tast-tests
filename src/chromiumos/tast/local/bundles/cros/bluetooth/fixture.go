// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

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
}

// ChromeLoggedInWithBluetoothRevampEnabled returns a fixture implementation
// that builds on the existing chromeLoggedIn fixture to also enable the
// BluetoothRevamp feature flag.
func ChromeLoggedInWithBluetoothRevampEnabled() FixtureImpl {
	return chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
		return []chrome.Option{chrome.EnableFeatures("BluetoothRevamp")}, nil
	})
}

// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package floss

import (
	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "bluetoothEnabledWithFloss",
		Desc: "Logs into Chrome with Floss enabled, and enables Bluetooth during set up and tear down",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Impl:            &bluetooth.ChromeLoggedInWithBluetoothEnabled{Impl: &Floss{}},
		Parent:          "chromeLoggedInWithFloss",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
}

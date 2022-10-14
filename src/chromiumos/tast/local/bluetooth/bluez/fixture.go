// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluez

import (
	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "bluetoothEnabledWithBlueZ",
		Desc: "Logs into Chrome with Floss disabled, and enables Bluetooth during set up and tear down",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Impl:            &bluetooth.ChromeLoggedInWithBluetoothEnabled{Impl: &BlueZ{}},
		Parent:          "chromeLoggedInWithBlueZ",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
}

// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetoothmojo

import (
	"context"

	//	"chromiumos/tast/local/bluetooth"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "bluetoothMojoJSObject",
		Desc: "Get JS object for bluetooth mojo interface",
		Contacts: []string{
			"shijinabraham@google.com",
			"cros-conn-test-team@m",
		},
		Impl:            &bluetoothMojoJSObject{},
		Parent:          "chromeLoggedInWithBluetoothEnabled",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
}

type bluetoothMojoJSObject struct {
	cr chrome.Chrome
	js chrome.JSObject
}

func (*bluetoothMojoJSObject) Reset(ctx context.Context) error {
	return nil
}

func (*bluetoothMojoJSObject) PreTest(ctx context.Context, s *testing.FixtTestState) {
}

func (*bluetoothMojoJSObject) PostTest(ctx context.Context, s *testing.FixtTestState) {
}

func (*bluetoothMojoJSObject) SetUp(ctx context.Context, s *testing.FixtState) interface{} {

	return s.ParentValue()
}

func (*bluetoothMojoJSObject) TearDown(ctx context.Context, s *testing.FixtState) {
}

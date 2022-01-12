// Copyright 2021 The Chromium OS Authors. All rights reserved.
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

const BTJS2 = `
                function () {
                this.foo = 'one'
                this.bluetoothConfig = chromeos.bluetoothConfig.mojom.CrosBluetoothConfig.getRemote();
                this.foo = 'two'
                this.bluetoothConfig.setBluetoothEnabledState(false);
                this.foo = 'three'
               return this

                }`

func (*bluetoothMojoJSObject) SetUp(ctx context.Context, s *testing.FixtState) interface{} {

	return s.ParentValue()
	/*
		cr := s.ParentValue().(*chrome.Chrome)

		/*
			_, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to create Test API connection: ", err)
			}


		//Open OS settings App Bluetooth Subpage
		url := "chrome://os-settings/bluetooth"
		crconn, err := apps.LaunchOSSettings(ctx, cr, url)
		if err != nil {
			s.Fatal("Failed to open settings app bluetooth page ", err)
		}

		var bluetoothMojo chrome.JSObject

		if err := crconn.Call(ctx, &bluetoothMojo, BTJS2); err != nil {
			s.Fatal(errors.Wrap(err, "failed to create mojo JS object"))

		}
		return bluetoothMojoJSObject{cr, bluetoothMojo}
	*/
}

func (*bluetoothMojoJSObject) TearDown(ctx context.Context, s *testing.FixtState) {
}

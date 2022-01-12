// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetoothmojo

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bluetooth/mojo"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "bluetoothMojoJSObject",
		Desc: "Get JS object for Bluetooth mojo interface via OS Settings App, with Bluetooth Revamp flag enabled",
		Contacts: []string{
			"shijinabraham@google.com",
			"cros-conn-test-team@google.com",
		},
		Impl:            &bluetoothMojoJSObject{},
		Parent:          "chromeLoggedInWithBluetoothEnabled",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
}

type bluetoothMojoJSObject struct {
	Crconn *chrome.Conn
	Js     chrome.JSObject
}

func (m *bluetoothMojoJSObject) Reset(ctx context.Context) error {

	if err := m.Crconn.Call(ctx, &(m.Js), mojo.BTConfigJS); err != nil {
		return errors.Wrap(err, "failed to create Bluetooth mojo JS")
	}

	if err := m.Js.Call(ctx, nil, `function init(){ this.initSysPropObs()}`); err != nil {
		return errors.Wrap(err, "failed to initailize the observer")
	}

	return nil
}

func (*bluetoothMojoJSObject) PreTest(ctx context.Context, s *testing.FixtTestState) {
}

func (*bluetoothMojoJSObject) PostTest(ctx context.Context, s *testing.FixtTestState) {
}

func (*bluetoothMojoJSObject) SetUp(ctx context.Context, s *testing.FixtState) interface{} {

	cr := s.ParentValue().(*chrome.Chrome)

	//Open OS settings App Bluetooth Subpage
	url := "chrome://os-settings/bluetooth"
	crconn, err := apps.LaunchOSSettings(ctx, cr, url)
	if err != nil {
		s.Fatal("Failed to open settings app: ", err)
	}

	var js chrome.JSObject

	if err := crconn.Call(ctx, &js, mojo.BTConfigJS); err != nil {
		s.Fatal(errors.Wrap(err, "failed to create Bluetooth mojo JS"))
	}

	if err := js.Call(ctx, nil, `function init(){ this.initSysPropObs()}`); err != nil {
		s.Fatal(errors.Wrap(err, "failed to initailize the observer"))
	}

	return &bluetoothMojoJSObject{crconn, js}
}

func (*bluetoothMojoJSObject) TearDown(ctx context.Context, s *testing.FixtState) {
}

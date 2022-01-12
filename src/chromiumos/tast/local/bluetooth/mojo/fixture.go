// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mojo

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
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
		Impl:            &BTConn{},
		Parent:          "chromeLoggedInWithBluetoothEnabled",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
}

// BTConn hold the chrome connection and the Mojo JS object
type BTConn struct {
	Crconn *chrome.Conn
	Js     *chrome.JSObject
}

func (m *BTConn) Reset(ctx context.Context) error {

	if err := m.Js.Release(ctx); err != nil {
		return errors.Wrap(err, "failed to release Bluetooth Mojo JS object")
	}

	if err := m.Crconn.Call(ctx, &(m.Js), BTConfigJS); err != nil {
		return errors.Wrap(err, "failed to create Bluetooth mojo JS")
	}

	if err := m.Js.Call(ctx, nil, `function init(){ this.initSysPropObs()}`); err != nil {
		return errors.Wrap(err, "failed to initailize the observer")
	}

	return nil
}

func (*BTConn) PreTest(ctx context.Context, s *testing.FixtTestState) {
}

func (*BTConn) PostTest(ctx context.Context, s *testing.FixtTestState) {
}

func (m *BTConn) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	cr := s.ParentValue().(*chrome.Chrome)

	// Open OS settings App Bluetooth Subpage
	const url = "chrome://os-settings/bluetooth"
	crConn, err := apps.LaunchOSSettings(ctx, cr, url)
	if err != nil {
		s.Fatal("Failed to open settings app: ", err)
	}

	var js chrome.JSObject

	if err := crConn.Call(ctx, &js, BTConfigJS); err != nil {
		s.Fatal(errors.Wrap(err, "failed to create Bluetooth mojo JS"))
	}

	if err := js.Call(ctx, nil, `function init(){ this.initSysPropObs()}`); err != nil {
		s.Fatal(errors.Wrap(err, "failed to initailize the observer"))
	}

	m.Crconn = crConn
	m.Js = &js
	return m
}

func (m *BTConn) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := m.Js.Release(ctx); err != nil {
		s.Fatal(errors.Wrap(err, "failed to release Bluetooth Mojo JS object"))
	}

}

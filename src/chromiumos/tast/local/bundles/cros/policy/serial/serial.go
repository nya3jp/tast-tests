// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package serial provides the implementation and helper functions for serial-related policy tests.
package serial

import (
	"context"
	"fmt"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// Values for the DefaultSerialGuardSetting policy
const (
	DefaultSerialGuardSettingBlock = 2
	DefaultSerialGuardSettingAsk   = 3
)

const (
	// SerialTestPage contains the path to the HTML page that needs to be added to
	// a test's `Data` property for serial testing to work
	SerialTestPage = "serial_policies_index.html"
)

// TestSerialPortRequest executes navigator.serial.requestPort() and checks whether
// the port selection dialog opens according to the `wantSerialDialog` argument.
func TestSerialPortRequest(ctx context.Context, cr *chrome.Chrome, br *browser.Browser, url string, wantSerialDialog bool) error {
	conn, err := br.NewConn(ctx, fmt.Sprintf("%s/%s", url, SerialTestPage))
	if err != nil {
		return errors.Wrap(err, "failed to open website")
	}
	defer conn.Close()

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}

	ui := uiauto.New(tconn)

	// Attempt to open the serial port dialog by clicking the HTML link that
	// triggers navigator.serial.requestPort(). We cannot use conn.Eval() for
	// this, because opening the serial port dialog must be triggered by a
	// user gesture for security reasons.
	if err := ui.LeftClick(nodewith.Role(role.Link).Name("requestSerialPort"))(ctx); err != nil {
		return errors.Wrap(err, "failed to request a serial port")
	}

	if wantSerialDialog {
		if err := ui.WaitUntilExists(nodewith.Role(role.Window).NameContaining("connect to a serial port"))(ctx); err != nil {
			return errors.Wrap(err, "serial port selection dialog did not open")
		}
	} else {
		if err := conn.WaitForExpr(ctx, "isBlocked"); err != nil {
			return errors.Wrap(err, "failed to wait for the serial port selection dialog to be blocked")
		}
	}

	return nil
}

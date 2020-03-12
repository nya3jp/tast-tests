// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package settings

import (
	"context"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	mtbfchrome "chromiumos/tast/local/mtbf/chrome"
)

const blankPage = `about:blank`

const osSettingURL = `chrome://os-settings`

const tabKey = `Tab`

// OpenOsSettingsPage opens chrome://os-settings in chrome
func OpenOsSettingsPage(ctx context.Context, cr *chrome.Chrome, useBlankPage bool) (*chrome.Conn, error) {
	var conn *chrome.Conn
	var err error
	// var err error

	if useBlankPage {
		conn, err = mtbfchrome.NewConn(ctx, cr, blankPage)
		if err != nil {
			return nil, err
		}

		err = sendKeyEvent(ctx)
		if err != nil {
			conn.Close()
			return nil, err
		}

		if err = conn.Navigate(ctx, osSettingURL); err != nil {
			conn.Close()
			return nil, mtbferrors.New(mtbferrors.ChromeNavigate, err, osSettingURL)
		}
	} else {
		conn, err = mtbfchrome.NewConn(ctx, cr, blankPage)
		if err != nil {
			return nil, err
		}
	}

	return conn, nil
}

// sendKeyEvent sends keyboard event to wake up screen
// We must do this for os-settings page because
// 1. To wake up the screen if it's been idle too long
// 2. To lose focus from URL area
func sendKeyEvent(ctx context.Context) error {
	// Setup keyboard.
	kb, err := input.Keyboard(ctx)

	if err != nil {
		return mtbferrors.New(mtbferrors.ChromeGetKeyboard, err)
	}

	defer kb.Close()

	if err := kb.Accel(ctx, tabKey); err != nil {
		return mtbferrors.New(mtbferrors.ChromeKeyPress, err, tabKey)
	}

	return nil
}

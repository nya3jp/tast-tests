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
	"chromiumos/tast/testing"
)

const (
	blankPage    = `about:blank`
	osSettingURL = `chrome://os-settings`
)

// OpenOsSettingsPage opens chrome://os-settings in chrome.
func OpenOsSettingsPage(ctx context.Context, cr *chrome.Chrome) (*chrome.Conn, error) {
	testing.ContextLog(ctx, "Go to os-setting page")

	var conn *chrome.Conn
	var err error

	conn, err = mtbfchrome.NewConn(ctx, cr, blankPage)
	if err != nil {
		testing.ContextLog(ctx, "Failed to open chrome: ", err)
		return nil, err
	}

	err = sendKeyEvent(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Failed to send key event: ", err)
		return nil, err
	}

	if err := conn.Navigate(ctx, osSettingURL); err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeNavigate, err, osSettingURL)
	}

	return conn, nil
}

// sendKeyEvent sends keyboard event to wake up screen.
// We must do this for os-settings page because:
// 1. To wake up the screen if it's been idle too long.
// 2. To lose focus from URL area.
func sendKeyEvent(ctx context.Context) error {
	// Setup keyboard.
	testing.ContextLog(ctx, "Send keyboard event")
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return mtbferrors.New(mtbferrors.ChromeGetKeyboard, err)
	}
	defer kb.Close()

	const tabKey = `Tab`
	if err := kb.Accel(ctx, tabKey); err != nil {
		return mtbferrors.New(mtbferrors.ChromeKeyPress, err, tabKey)
	}

	return nil
}

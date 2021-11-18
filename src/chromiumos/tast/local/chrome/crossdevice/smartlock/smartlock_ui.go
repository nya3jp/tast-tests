// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package smartlock is for contrlling ChromeOS Smart Lock functionality.
package smartlock

import (
	"context"
	"encoding/json"
	"fmt"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
)

const (
	connectedDevicesSettingsURL = "multidevice/features/"
	smartLockSettingsURL        = "smartLock"
	smartLockSubpage            = `document.querySelector("os-settings-ui").shadowRoot` +
		`.querySelector("os-settings-main").shadowRoot` +
		`.querySelector("os-settings-page").shadowRoot` +
		`.querySelector("settings-multidevice-page").shadowRoot` +
		`.querySelector("settings-multidevice-smartlock-subpage")`
)

// OpenSmartLockSubpage opens the Smart Lock sub page in OS Settings
func OpenSmartLockSubpage(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) (*ossettings.OSSettings, error) {
	settings, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, connectedDevicesSettingsURL+smartLockSettingsURL, func(context.Context) error { return nil })
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch OS Settings to the Smart Lock page")
	}
	return settings, nil
}

// DisableSmartLockLogin disables Smart Lock login functionality.
// This means that only unlocking with Smart Lock is allowed.
func DisableSmartLockLogin(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) error {
	settings, err := OpenSmartLockSubpage(ctx, tconn, cr)
	if err != nil {
		return errors.Wrap(err, "failed to open Smart Lock subpage in OS Settings")
	}
	settingsConn, err := settings.ChromeConn(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "failed to connect to OS settings target")
	}
	if err := settingsConn.WaitForExpr(ctx, smartLockSubpage); err != nil {
		return errors.Wrap(err, "failed waiting for Smart Lock subpage to load")
	}
	if err := settingsConn.Eval(ctx, smartLockSubpage+`.updateSmartLockSignInEnabled_(false)`, nil); err != nil {
		return errors.Wrap(err, "failed to toggle smart lock login button")
	}
	return nil
}

// EnableSmartLockLogin enables the ability to login with Smart Lock.
func EnableSmartLockLogin(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, password string) error {
	settings, err := OpenSmartLockSubpage(ctx, tconn, cr)
	if err != nil {
		return errors.Wrap(err, "failed to open Smart Lock subpage in OS Settings")
	}
	settingsConn, err := settings.ChromeConn(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "failed to connect to OS settings target")
	}
	token, err := settings.AuthToken(ctx, settingsConn, password)
	if err != nil {
		return errors.Wrap(err, "failed to get auth token")
	}
	data, err := json.Marshal(token)
	if err != nil {
		return errors.Wrap(err, "failed to marshall auth token to JSON")
	}
	if err := settingsConn.WaitForExpr(ctx, smartLockSubpage); err != nil {
		return errors.Wrap(err, "failed waiting for Smart Lock subpage to load")
	}
	expr := fmt.Sprintf(`%s.authToken_ = %s`, smartLockSubpage, data)
	if err := settingsConn.Eval(ctx, expr, nil); err != nil {
		return errors.Wrap(err, "failed to set authToken_ property")
	}
	if err := settingsConn.Eval(ctx, smartLockSubpage+`.onEnableSignInDialogClose_()`, nil); err != nil {
		return errors.Wrap(err, "failed to toggle smart lock login button")
	}

	return nil
}

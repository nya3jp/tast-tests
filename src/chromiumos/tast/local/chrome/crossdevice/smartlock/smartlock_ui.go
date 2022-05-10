// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package smartlock is for controlling ChromeOS Smart Lock functionality.
package smartlock

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	connectedDevicesSettingsURL = "multidevice/features/"
	smartLockSettingsURL        = "smartLock"
	multidevicePage             = `document.querySelector("os-settings-ui").shadowRoot` +
		`.querySelector("os-settings-main").shadowRoot` +
		`.querySelector("os-settings-page").shadowRoot` +
		`.querySelector("settings-multidevice-page")`
	multidevicePasswordPrompt = multidevicePage + `.shadowRoot.querySelector("settings-password-prompt-dialog")`
	smartLockSubpage          = multidevicePage + `.shadowRoot.querySelector("settings-multidevice-smartlock-subpage")`
	smartLockToggle           = smartLockSubpage + `.shadowRoot.querySelector("settings-multidevice-feature-toggle")`
)

// OpenSmartLockSubpage opens the Smart Lock sub page in OS Settings
func OpenSmartLockSubpage(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) (*ossettings.OSSettings, error) {
	settings, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, connectedDevicesSettingsURL+smartLockSettingsURL, func(context.Context) error { return nil })
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch OS Settings to the Smart Lock page")
	}
	return settings, nil
}

// ToggleSmartLockEnabled opens the Smart Lock sub page in OS Settings and toggles Smart Lock's enabled state
func ToggleSmartLockEnabled(ctx context.Context, enable bool, tconn *chrome.TestConn, cr *chrome.Chrome, password string) error {
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
	var toggleChecked bool
	if err := settingsConn.Eval(ctx, smartLockToggle+`.checked_`, &toggleChecked); err != nil {
		return errors.Wrap(err, "failed to read Smart Lock toggle checked_ property")
	}
	if toggleChecked == enable {
		// Smart Lock is already in the desired state.
		return nil
	}

	// Flip the toggle. If toggling on, we need to provide an auth token.
	if enable {
		token, err := settings.AuthToken(ctx, settingsConn, password)
		if err != nil {
			return errors.Wrap(err, "failed to get auth token")
		}
		data, err := json.Marshal(token)
		if err != nil {
			return errors.Wrap(err, "failed to marshal auth token to JSON")
		}
		expr := fmt.Sprintf(`%s.authToken_ = %s`, multidevicePage, data)
		if err := settingsConn.Eval(ctx, expr, nil); err != nil {
			return errors.Wrap(err, "failed to set authToken_ property")
		}
		if err := settingsConn.Eval(ctx, smartLockToggle+`.toggleFeature()`, nil); err != nil {
			return errors.Wrap(err, "failed to toggle Smart Lock enabled on")
		}
		// When the toggle is enabled, the password dialog will be shown,
		// but we only need to cancel it since we've already provided a token.
		if err := settingsConn.Eval(ctx, multidevicePasswordPrompt+`.onCancelTap_()`, nil); err != nil {
			return errors.Wrap(err, "failed to close password prompt")
		}
	} else {
		if err := settingsConn.Eval(ctx, smartLockToggle+`.toggleFeature()`, nil); err != nil {
			return errors.Wrap(err, "failed to toggle Smart Lock enabled off")
		}
	}

	// Ensure that the Smart Lock toggle is now in the desired state. Wait
	// up to 3 seconds since the change doesn't take effect instantaneously.
	var expr string = smartLockToggle + `.checked_`
	if !enable {
		expr = `!` + expr
	}
	if err := settingsConn.WaitForExprWithTimeout(ctx, expr, 3*time.Second); err != nil {
		return errors.Wrap(err, "failed to change Smart Lock enabled state")
	}
	return nil
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
		return errors.Wrap(err, "failed to toggle Smart Lock login button")
	}
	if err := settingsConn.Eval(ctx, smartLockSubpage+`.onSmartLockSignInEnabledChanged_()`, nil); err != nil {
		return errors.Wrap(err, "failed to update Smart Lock login setting")
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
		return errors.Wrap(err, "failed to marshal auth token to JSON")
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

// SignOut ends the existing chrome session and logs out by keyboard shortcut.
func SignOut(ctx context.Context, cr *chrome.Chrome, kb *input.KeyboardEventWriter) error {
	cr.Close(ctx)
	if err := kb.Accel(ctx, "Ctrl+Shift+Q"); err != nil {
		return errors.Wrap(err, "failed to emulate shortcut 1st press")
	}
	if err := kb.Accel(ctx, "Ctrl+Shift+Q"); err != nil {
		return errors.Wrap(err, "failed to emulate shortcut 2nd press")
	}
	return nil
}

// CheckSmartLockVisibilityOnSigninScreen signs out and checks whether
// Smart Lock is visible on the signin screen, before signing back in.
func CheckSmartLockVisibilityOnSigninScreen(ctx context.Context, expectVisible bool, cr *chrome.Chrome, tconn *chrome.TestConn, loginOpts, noLoginOpts []chrome.Option) (*chrome.Chrome, *chrome.TestConn, error) {
	// Logout is done by keyboard shortcut.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		errors.Wrap(err, "failed to get keyboard")
	}
	defer kb.Close()

	// Sign out
	if cr, tconn, err = goToLoginScreen(ctx, cr, tconn, kb, loginOpts, noLoginOpts); err != nil {
		return nil, nil, err
	}

	if err = lockscreen.WaitForSmartLockVisible(ctx, expectVisible, tconn); err != nil {
		return nil, nil, err
	}

	// Sign in
	if cr, tconn, err = signInWithPassword(ctx, cr, tconn, loginOpts); err != nil {
		return nil, nil, errors.Wrap(err, "failed to sign in")
	}

	return cr, tconn, nil
}

// CheckSmartLockVisibilityOnLockScreen locks the session and checks whether
// Smart Lock is visible on the lock screen, before unlocking.
func CheckSmartLockVisibilityOnLockScreen(ctx context.Context, expectVisible bool, tconn *chrome.TestConn, username, password string) error {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		errors.Wrap(err, "failed to get keyboard")
	}
	defer kb.Close()

	if err := lockscreen.Lock(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to lock the screen on ChromeOS")
	}

	if err = lockscreen.WaitForSmartLockVisible(ctx, expectVisible, tconn); err != nil {
		return err
	}

	// Unlock
	if err := lockscreen.EnterPassword(ctx, tconn, username, password, kb); err != nil {
		return errors.Wrap(err, "failed to unlock with password")
	}

	return nil
}

// goToLoginScreen signs out of the current session a couple of times so that
// signin screen settings have a chance to take effect.
func goToLoginScreen(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, loginOpts, noLoginOpts []chrome.Option) (*chrome.Chrome, *chrome.TestConn, error) {
	if err := testing.Sleep(ctx, 10*time.Second); err != nil {
		return nil, nil, errors.Wrap(err, "failed to sleep before SignOut")
	}

	var err error
	if err = SignOut(ctx, cr, kb); err != nil {
		return nil, nil, errors.Wrap(err, "failed to sign out")
	}

	// TODO(b/217272610) Remove this second log in once this bug is resolved.
	if cr, tconn, err = signInWithPassword(ctx, cr, tconn, loginOpts); err != nil {
		return nil, nil, errors.Wrap(err, "failed to sign in")
	}

	if _, err := OpenSmartLockSubpage(ctx, tconn, cr); err != nil {
		return nil, nil, errors.Wrap(err, "failed to open Smart lock sub page")
	}
	if err := testing.Sleep(ctx, 10*time.Second); err != nil {
		return nil, nil, errors.Wrap(err, "failed to sleep on Smart Lock subpage")
	}
	if err := SignOut(ctx, cr, kb); err != nil {
		return nil, nil, errors.Wrap(err, "failed to sign out")
	}

	cr, err = chrome.New(
		ctx,
		noLoginOpts...,
	)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to start chrome")
	}
	tconn, err = cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "getting API connection failed")
	}

	return cr, tconn, nil
}

func signInWithPassword(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, loginOpts []chrome.Option) (*chrome.Chrome, *chrome.TestConn, error) {
	cr, err := chrome.New(
		ctx,
		loginOpts...,
	)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to login to Chrome")
	}

	tconn, err = cr.TestAPIConn(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "creating test API connection failed")
	}

	return cr, tconn, nil
}

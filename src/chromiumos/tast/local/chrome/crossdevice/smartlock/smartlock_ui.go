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
	"chromiumos/tast/local/chrome/uiauto/faillog"
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

// SettingsTestData contains all of the data passed around in the Smart Lock
// Settings tests. It helps keep the function signatures small.
type SettingsTestData struct {
	Ctx         context.Context
	LoginOpts   []chrome.Option
	NoLoginOpts []chrome.Option
	Cr          *chrome.Chrome
	Tconn       *chrome.TestConn
	Kb          *input.KeyboardEventWriter
	Username    string
	Password    string
}

// OpenSmartLockSubpage opens the Smart Lock sub page in OS Settings
func OpenSmartLockSubpage(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) (*ossettings.OSSettings, error) {
	settings, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, connectedDevicesSettingsURL+smartLockSettingsURL, func(context.Context) error { return nil })
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch OS Settings to the Smart Lock page")
	}
	return settings, nil
}

// ToggleSmartLockEnabled opens the Smart Lock sub page in OS Settings and toggles Smart Lock's enabled state
func ToggleSmartLockEnabled(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, password string) error {
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
	expr := fmt.Sprintf(`%s.authToken_ = %s`, multidevicePage, data)
	if err := settingsConn.Eval(ctx, expr, nil); err != nil {
		return errors.Wrap(err, "failed to set authToken_ property")
	}
	if err := settingsConn.Eval(ctx, smartLockToggle+`.toggleFeature()`, nil); err != nil {
		return errors.Wrap(err, "failed to toggle smart lock enabled button")
	}
	// If the toggle is being enabled, the password dialog will be shown,
	// but we only need to cancel it since we've already provided a token.
	// If the toggle is being disabled, the dialog won't be shown, and this
	// expression will return an error.
	settingsConn.Eval(ctx, multidevicePasswordPrompt+`.onCancelTap_()`, nil)
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

// GetSettingsTestData builds an instance of SettingsTestData.
func GetSettingsTestData(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, username, password string, loginOpts, noLoginOpts []chrome.Option) (SettingsTestData, error) {
	// Logout is done by keyboard shortcut. So setup one to reuse throughout the test.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return SettingsTestData{}, errors.Wrap(err, "failed to get keyboard")
	}

	return SettingsTestData{
		Ctx:         ctx,
		LoginOpts:   loginOpts,
		NoLoginOpts: noLoginOpts,
		Cr:          cr,
		Tconn:       tconn,
		Kb:          kb,
		Username:    username,
		Password:    password,
	}, nil
}

// CleanUpSettingsTestData should be called before SettingsTestData falls out of scope.
func CleanUpSettingsTestData(t *SettingsTestData, outDir string, hasError func() bool) {
	t.Kb.Close()
	faillog.DumpUITreeOnError(t.Ctx, outDir, hasError, t.Tconn)
	t.Cr.Close(t.Ctx)
}

// CheckSmartLockVisibility either signs out or locks the session and checks whether
// Smart Lock is visible on the signin/lock screen, before signing back in/unlocking.
func CheckSmartLockVisibility(expectVisible, signin bool, t *SettingsTestData) error {
	var err error

	// Sign out or lock
	if signin {
		if err := goToLoginScreen(t); err != nil {
			return err
		}
	} else {
		if err := lockscreen.Lock(t.Ctx, t.Tconn); err != nil {
			return errors.Wrap(err, "failed to lock the screen on Chrome OS")
		}
	}

	if err := testing.Sleep(t.Ctx, 5*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep on lock screen")
	}

	// If Smart Lock's visibility doesn't match our expectations, return an error.
	authIconFound := lockscreen.HasAuthIconView(t.Ctx, t.Tconn)
	if authIconFound && !expectVisible {
		err = errors.New("found auth icon; Smart Lock should not be visible")
	}
	if !authIconFound && expectVisible {
		err = errors.New("auth icon missing; Smart Lock should be visible")
	}

	// Sign in or unlock
	if signin {
		if err := signInWithPassword(t); err != nil {
			return errors.Wrap(err, "failed to sign in")
		}
	} else {
		if err := lockscreen.EnterPassword(t.Ctx, t.Tconn, t.Username, t.Password, t.Kb); err != nil {
			return errors.Wrap(err, "failed to unlock with password")
		}
	}
	return err
}

// goToLoginScreen signs out of the current session a couple of times so that
// signin screen settings have a chance to take effect.
func goToLoginScreen(t *SettingsTestData) error {
	if err := testing.Sleep(t.Ctx, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep before SignOut")
	}

	var err error
	if err = SignOut(t.Ctx, t.Cr, t.Kb); err != nil {
		return errors.Wrap(err, "failed to sign out")
	}

	// TODO(b/217272610) Remove this second log in once this bug is resolved.
	if err = signInWithPassword(t); err != nil {
		return errors.Wrap(err, "failed to sign in")
	}

	if _, err := OpenSmartLockSubpage(t.Ctx, t.Tconn, t.Cr); err != nil {
		return errors.Wrap(err, "failed to open Smart lock sub page")
	}
	if err := testing.Sleep(t.Ctx, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep on Smart Lock subpage")
	}
	if err := SignOut(t.Ctx, t.Cr, t.Kb); err != nil {
		return errors.Wrap(err, "failed to sign out")
	}

	t.Cr, err = chrome.New(
		t.Ctx,
		t.NoLoginOpts...,
	)
	if err != nil {
		return errors.Wrap(err, "failed to start chrome")
	}
	t.Tconn, err = t.Cr.SigninProfileTestAPIConn(t.Ctx)
	if err != nil {
		return errors.Wrap(err, "getting API connection failed")
	}

	return nil
}

func signInWithPassword(t *SettingsTestData) error {
	var err error
	t.Cr, err = chrome.New(
		t.Ctx,
		t.LoginOpts...,
	)
	if err != nil {
		return errors.Wrap(err, "failed to login to Chrome")
	}

	t.Tconn, err = t.Cr.TestAPIConn(t.Ctx)
	if err != nil {
		return errors.Wrap(err, "creating test API connection failed")
	}

	return nil
}

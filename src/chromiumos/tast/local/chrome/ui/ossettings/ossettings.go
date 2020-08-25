// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ossettings supports controlling the Settings App on Chrome OS.
// This differs from Chrome settings (chrome://settings vs chrome://os-settings)
package ossettings

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

const uiTimeout = 15 * time.Second

// AboutChromeOS is a subpage link.
var AboutChromeOS = ui.FindParams{
	Name: "About Chrome OS",
	Role: ui.RoleTypeLink,
}

// Launch launches the Settings app.
// An error is returned if the app fails to launch.
func Launch(ctx context.Context, tconn *chrome.TestConn) error {
	app := apps.Settings
	if err := apps.Launch(ctx, tconn, app.ID); err != nil {
		return errors.Wrapf(err, "failed to launch %s", app.Name)
	}

	testing.ContextLog(ctx, "Waiting for settings app shown in shelf")
	if err := ash.WaitForApp(ctx, tconn, app.ID); err != nil {
		return errors.Wrapf(err, "%s did not appear in shelf after launch", app.Name)
	}
	return nil
}

// LaunchAtPage launches the Settings app at a particular page.
// An error is returned if the app fails to launch.
func LaunchAtPage(ctx context.Context, tconn *chrome.TestConn, subpage ui.FindParams) error {
	// Launch Settings App.
	err := Launch(ctx, tconn)
	if err != nil {
		return err
	}

	// If needed, open the main menu. On small screens the sidebar is collapsed,
	// so we click the menu or find the sidebar item we need.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		subpageFound, _ := ui.Exists(ctx, tconn, subpage)
		if subpageFound {
			return nil
		}
		menu, err := ui.Find(ctx, tconn, ui.FindParams{
			Name: "Main menu",
			Role: ui.RoleTypeButton,
		})
		if err != nil {
			return errors.New("no sidebar item loaded yet")
		}
		defer menu.Release(ctx)
		if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to wait for location change completed for menu")
		}
		return menu.LeftClick(ctx)
	}, &testing.PollOptions{Interval: 1 * time.Second}); err != nil {
		return err
	}

	// Find and click the subpage we want in the sidebar.
	if err := ui.FindAndClick(ctx, tconn, subpage, uiTimeout); err != nil {
		return errors.Wrapf(err, "failed to find subpage with %v", subpage)
	}
	return nil
}

// ChromeConn returns a Chrome connection to the Settings app.
func ChromeConn(ctx context.Context, cr *chrome.Chrome) (*chrome.Conn, error) {
	return cr.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://os-settings/"))
}

// EnablePINUnlock enables unlocking the device with the specified PIN.
func EnablePINUnlock(ctx context.Context, settingsConn *chrome.Conn, password, PIN string, autosubmit bool) error {
	// Wait for chrome.quickUnlockPrivate to be available.
	if err := settingsConn.WaitForExpr(ctx, `chrome.quickUnlockPrivate !== undefined`); err != nil {
		return errors.Wrap(err, "failed waiting for chrome.quickUnlockPrivate to load")
	}

	// An auth token is required to set up the PIN.
	var token string
	if err := settingsConn.Call(ctx, &token,
		`(password) => new Promise(function(resolve, reject) {
			chrome.quickUnlockPrivate.getAuthToken(password, function(authToken) {
			  if (chrome.runtime.lastError === undefined) {
				resolve(authToken['token']);
			  } else {
				reject(chrome.runtime.lastError.message);
			  }
			});
		  })`, password,
	); err != nil {
		return errors.Wrap(err, "failed to get auth token")
	}

	// Set the PIN and enable PIN unlock.
	if err := settingsConn.Call(ctx, nil,
		`(token, PIN) => new Promise(function(resolve, reject) {
			chrome.quickUnlockPrivate.setModes(token, [chrome.quickUnlockPrivate.QuickUnlockMode.PIN], [PIN], function(success) {
			  if (chrome.runtime.lastError === undefined) {
				resolve(success);
			  } else {
				reject(chrome.runtime.lastError.message);
			  }
			});
		  })`, token, PIN,
	); err != nil {
		return errors.Wrap(err, "failed to set PIN and enable PIN unlock")
	}

	// Enable or disable PIN autosubmit.
	if err := settingsConn.Call(ctx, nil,
		`(token, PIN, autosubmit) => new Promise(function(resolve, reject) {
			chrome.quickUnlockPrivate.setPinAutosubmitEnabled(token, PIN, autosubmit, function(success) {
				if (chrome.runtime.lastError === undefined) {
				resolve(success);
				} else {
				reject(chrome.runtime.lastError.message);
				}
			});
			})`, token, PIN, autosubmit,
	); err != nil {
		return errors.Wrap(err, "failed to enable PIN autosubmit")
	}

	return nil
}

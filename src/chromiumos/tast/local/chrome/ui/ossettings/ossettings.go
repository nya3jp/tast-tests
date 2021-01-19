// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ossettings supports controlling the Settings App on Chrome OS.
// This differs from Chrome settings (chrome://settings vs chrome://os-settings)
package ossettings

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

const uiTimeout = 15 * time.Second

var defaultOSSettingsPollOptions = &testing.PollOptions{Timeout: 10 * time.Second, Interval: 1 * time.Second}

const osSettingsURLPrefix = "chrome://os-settings/"

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

// Close closes the Settings app.
func Close(ctx context.Context, tconn *chrome.TestConn) error {
	if err := apps.Close(ctx, tconn, apps.Settings.ID); err != nil {
		return errors.Wrap(err, "failed to close settings app")
	}
	if err := ash.WaitForAppClosed(ctx, tconn, apps.Settings.ID); err != nil {
		return errors.Wrap(err, "failed waiting for settings app to close")
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
	if err := ui.StableFindAndClick(ctx, tconn, subpage, defaultOSSettingsPollOptions); err != nil {
		return errors.Wrapf(err, "failed to find subpage with %v", subpage)
	}
	return nil
}

// LaunchAtPageURL launches the Settings app at a particular page via changing URL in javascript.
// It uses a condition check to make sure the function completes correctly.
// It is high recommended to use UI validation in condition check.
func LaunchAtPageURL(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, pageShortURL string, condition func(context.Context) (bool, error)) error {
	// Launch Settings App.
	err := Launch(ctx, tconn)
	if err != nil {
		return err
	}

	osSettingsConn, err := ChromeConn(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "failed to connect to OS settings target")
	}

	// Using testing.Poll to allow 2-3 times retries.
	// Sometimes changing window.location in javascript just does not work and no error thrown out.
	// Refer to b/177868367.
	return testing.Poll(ctx, func(ctx context.Context) error {
		// Eval javascript function to change page url.
		if err := osSettingsConn.Eval(ctx, fmt.Sprintf("window.location = %q", osSettingsURLPrefix+pageShortURL), nil); err != nil {
			return errors.Wrap(err, "failed to run javascript")
		}

		// Wait for condition after changing location.
		return testing.Poll(ctx, func(ctx context.Context) error {
			if result, err := condition(ctx); err != nil {
				return errors.Wrap(err, "failed to evaluate condition")
			} else if !result {
				return errors.New("failed to match condition after changing page location in javascript")
			}
			return nil
		}, &testing.PollOptions{Timeout: 20 * time.Second, Interval: 200 * time.Millisecond})
	}, &testing.PollOptions{Timeout: 1 * time.Minute})
}

// ChromeConn returns a Chrome connection to the Settings app.
func ChromeConn(ctx context.Context, cr *chrome.Chrome) (*chrome.Conn, error) {
	settingsConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://os-settings/"))
	if err != nil {
		return nil, err
	}
	if err := chrome.AddTastLibrary(ctx, settingsConn); err != nil {
		settingsConn.Close()
		return nil, errors.Wrap(err, "failed to introduce tast library")
	}
	return settingsConn, nil
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
		`password => tast.promisify(chrome.quickUnlockPrivate.getAuthToken)(password).then(authToken => authToken['token'])`, password,
	); err != nil {
		return errors.Wrap(err, "failed to get auth token")
	}

	if err := settingsConn.Call(ctx, nil,
		`(token, PIN) => tast.promisify(chrome.quickUnlockPrivate.setModes)(token, [chrome.quickUnlockPrivate.QuickUnlockMode.PIN], [PIN])`, token, PIN,
	); err != nil {
		return errors.Wrap(err, "failed to get auth token")
	}

	if err := settingsConn.Call(ctx, nil,
		`tast.promisify(chrome.quickUnlockPrivate.setPinAutosubmitEnabled)`, token, PIN, autosubmit,
	); err != nil {
		return errors.Wrap(err, "failed to get auth token")
	}

	return nil
}

// LaunchHelpApp launches Help app by clicking "Get help with Chrome OS".
func LaunchHelpApp(ctx context.Context, tconn *chrome.TestConn) error {
	return ui.StableFindAndClick(ctx, tconn, ui.FindParams{
		Name: "Get help with Chrome OS",
		Role: ui.RoleTypeLink,
	}, defaultOSSettingsPollOptions)
}

// LaunchWhatsNew launches Help app by clicking "See what's new".
func LaunchWhatsNew(ctx context.Context, tconn *chrome.TestConn) error {
	return ui.StableFindAndClick(ctx, tconn, ui.FindParams{
		Name: "See what's new",
		Role: ui.RoleTypeLink,
	}, defaultOSSettingsPollOptions)
}

// UIRootNode returns the root a11y node of os-settings window.
func UIRootNode(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
	params := ui.FindParams{
		Attributes: map[string]interface{}{"name": regexp.MustCompile("Settings.*")},
		Role:       ui.RoleTypeRootWebArea,
	}

	return ui.StableFind(ctx, tconn, params, defaultOSSettingsPollOptions)
}

// DescendantNodeWithTimeout returns an a11y ui node inside os-settings window.
func DescendantNodeWithTimeout(ctx context.Context, tconn *chrome.TestConn, params ui.FindParams, timeout time.Duration) (*ui.Node, error) {
	rootNode, err := UIRootNode(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get root node of os-settings window")
	}
	defer rootNode.Release(ctx)

	return rootNode.DescendantWithTimeout(ctx, params, timeout)
}

// DescendantNodeExists checks if a descendant node of os-settings can be found.
func DescendantNodeExists(ctx context.Context, tconn *chrome.TestConn, params ui.FindParams) (bool, error) {
	rootNode, err := UIRootNode(ctx, tconn)
	if err != nil {
		return false, errors.Wrap(err, "failed to get root node of os-settings window")
	}
	defer rootNode.Release(ctx)

	exists, err := rootNode.DescendantExists(ctx, params)
	if err != nil {
		err = errors.Wrapf(err, "failed to find descendant node with params: %v", params)
	}
	return exists, err
}

// WaitForSearchBox waits for the search box to appear.
// Useful for checking that some content has loaded and Settings is ready to use.
func WaitForSearchBox(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration) error {
	params := ui.FindParams{
		Role: ui.RoleTypeSearchBox,
		Name: "Search settings",
	}
	return ui.WaitUntilExists(ctx, tconn, params, timeout)
}

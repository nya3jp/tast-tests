// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ossettings supports controlling the Settings App on Chrome OS.
// This differs from Chrome settings (chrome://settings vs chrome://os-settings)
package ossettings

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/testing"
)

const uiTimeout = 15 * time.Second

var defaultPollOpts = &testing.PollOptions{Timeout: 10 * time.Second, Interval: 1 * time.Second}

const urlPrefix = "chrome://os-settings/"

// WindowFinder is the finder for the Settings window.
var WindowFinder *nodewith.Finder = nodewith.MultilingualNameStartingWith("Settings", map[string]string{"de": "Einstellungen"}).
	Role(role.Window).First()

// SearchBoxFinder is the finder for the search box in the settings app.
var SearchBoxFinder = nodewith.MultilingualName("Search settings", map[string]string{"de": "In Einstellungen suchen"}).
	Role(role.SearchBox).Ancestor(WindowFinder)

// AboutChromeOS is a subpage link.
var AboutChromeOS = nodewith.MultilingualName("About Chrome OS", map[string]string{"de": "Über Chrome OS"}).
	Role(role.Link)

// OSSettings represents an instance of the Settings app.
type OSSettings struct {
	ui    *uiauto.Context
	tconn *chrome.TestConn
}

// New returns a new ossettings context.
// OSSettings can be launched from a page or app.
func New(tconn *chrome.TestConn) *OSSettings {
	return &OSSettings{ui: uiauto.New(tconn), tconn: tconn}
}

// Launch launches the Settings app.
// An error is returned if the app fails to launch.
func Launch(ctx context.Context, tconn *chrome.TestConn) (*OSSettings, error) {
	app := apps.Settings
	if err := apps.Launch(ctx, tconn, app.ID); err != nil {
		return nil, errors.Wrapf(err, "failed to launch %s", app.Name)
	}

	testing.ContextLog(ctx, "Waiting for settings app shown in shelf")
	if err := ash.WaitForApp(ctx, tconn, app.ID, time.Minute); err != nil {
		return nil, errors.Wrapf(err, "%s did not appear in shelf after launch", app.Name)
	}
	return &OSSettings{ui: uiauto.New(tconn), tconn: tconn}, nil
}

// Close closes the Settings app.
// This is automatically done when chrome resets and is not necessary to call.
func (s *OSSettings) Close(ctx context.Context) error {
	app := apps.Settings
	if err := apps.Close(ctx, s.tconn, app.ID); err != nil {
		return errors.Wrap(err, "failed to close settings app")
	}
	if err := ash.WaitForAppClosed(ctx, s.tconn, app.ID); err != nil {
		return errors.Wrap(err, "failed waiting for settings app to close")
	}
	return nil
}

// LaunchAtPage launches the Settings app at a particular page.
// An error is returned if the app fails to launch.
// TODO (b/189055966): Fix the failure to launch the right subpage.
func LaunchAtPage(ctx context.Context, tconn *chrome.TestConn, subpage *nodewith.Finder) (*OSSettings, error) {
	// Launch Settings App.
	s, err := Launch(ctx, tconn)
	if err != nil {
		return nil, err
	}

	// Wait until either the subpage or main menu exist.
	// On small screens the sidebar is collapsed, and the main menu must be clicked.
	subPageInApp := subpage.FinalAncestor(WindowFinder)
	mainMenu := nodewith.Name("Main menu").Role(role.Button).Ancestor(WindowFinder)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := s.ui.Exists(subPageInApp)(ctx); err == nil {
			return nil
		}
		if err := s.ui.Exists(mainMenu)(ctx); err == nil {
			return nil
		}
		return errors.New("neither subpage nor main menu exist")
	}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 30 * time.Second}); err != nil {
		return nil, err
	}

	// If the subpage doesn't exist, click the main menu.
	// Then click the subpage that we want in the sidebar.
	if err := uiauto.Combine("click subpage",
		s.ui.IfSuccessThen(s.ui.Gone(subPageInApp), s.ui.LeftClick(mainMenu)),
		s.ui.LeftClick(subPageInApp),
	)(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to click subpage with %v", subpage)
	}
	return s, nil
}

// LaunchAtPageURL launches the Settings app at a particular page via changing URL in javascript.
// It uses a condition check to make sure the function completes correctly.
// It is high recommended to use UI validation in condition check.
func LaunchAtPageURL(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, pageShortURL string, condition func(context.Context) error) (*OSSettings, error) {
	// Launch Settings App.
	s, err := Launch(ctx, tconn)
	if err != nil {
		return nil, err
	}

	settingsConn, err := s.ChromeConn(ctx, cr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to OS settings target")
	}

	if err := webutil.NavigateToURLInApp(settingsConn, urlPrefix+pageShortURL, condition, 20*time.Second)(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

// LaunchAtAppMgmtPage launches the Settings app at a particular app management page under app
// via changing URL in javascript.
// The URL includes an App ID.
// It calls LaunchAtPageURL.
func LaunchAtAppMgmtPage(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, appID string, condition func(context.Context) error) (*OSSettings, error) {
	return LaunchAtPageURL(ctx, tconn, cr, fmt.Sprintf("app-management/detail?id=%s", appID), condition)
}

// LaunchHelpApp returns a function that launches Help app by clicking "Get help with Chrome OS".
func (s *OSSettings) LaunchHelpApp() uiauto.Action {
	return s.ui.LeftClick(nodewith.Name("Get help with Chrome OS").Role(role.Link).Ancestor(WindowFinder))
}

// LaunchWhatsNew returns a function that launches Help app by clicking "See what's new".
func (s *OSSettings) LaunchWhatsNew() uiauto.Action {
	return s.ui.LeftClick(nodewith.Name("See what's new").Role(role.Link).Ancestor(WindowFinder))
}

// ChromeConn returns a Chrome connection to the Settings app.
func (s *OSSettings) ChromeConn(ctx context.Context, cr *chrome.Chrome) (*chrome.Conn, error) {
	targetFilter := func(t *chrome.Target) bool { return strings.HasPrefix(t.URL, urlPrefix) }
	settingsConn, err := cr.NewConnForTarget(ctx, targetFilter)
	if err != nil {
		return nil, err
	}
	if err := chrome.AddTastLibrary(ctx, settingsConn); err != nil {
		settingsConn.Close()
		return nil, errors.Wrap(err, "failed to introduce tast library")
	}
	return settingsConn, nil
}

// EnablePINUnlock returns a function that enables unlocking the device with the specified PIN.
func (s *OSSettings) EnablePINUnlock(cr *chrome.Chrome, password, PIN string, autosubmit bool) uiauto.Action {
	return func(ctx context.Context) error {
		settingsConn, err := s.ChromeConn(ctx, cr)
		if err != nil {
			return errors.Wrap(err, "failed to connect to OS settings target")
		}
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
}

// WaitForSearchBox returns a function that waits for the search box to appear.
// Useful for checking that some content has loaded and Settings is ready to use.
func (s *OSSettings) WaitForSearchBox() uiauto.Action {
	return s.ui.WaitUntilExists(SearchBoxFinder)
}

// EvalJSWithShadowPiercer executes javascript in Settings app web page.
func (s *OSSettings) EvalJSWithShadowPiercer(ctx context.Context, cr *chrome.Chrome, expr string, out interface{}) error {
	conn, err := s.ChromeConn(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "failed to connect to Settings web page")
	}
	defer conn.Close()
	return webutil.EvalWithShadowPiercer(ctx, conn, expr, out)
}

// SetToggleOption clicks toggle option to enable or disable an option.
// It does nothing if the option is already expected.
func (s *OSSettings) SetToggleOption(cr *chrome.Chrome, optionName string, expected bool) uiauto.Action {
	return func(ctx context.Context) error {
		if isEnabled, err := s.IsToggleOptionEnabled(ctx, cr, optionName); err != nil {
			return err
		} else if isEnabled == expected {
			return nil
		}
		optionFinder := nodewith.Name(optionName).Role(role.ToggleButton)
		return uiauto.Combine("set toggle option",
			s.LeftClick(optionFinder),
			s.WaitUntilToggleOption(cr, optionName, expected),
		)(ctx)
	}
}

// IsToggleOptionEnabled checks whether the toggle option is enabled or not.
func (s *OSSettings) IsToggleOptionEnabled(ctx context.Context, cr *chrome.Chrome, optionName string) (bool, error) {
	toggleButtonCSSSelector := fmt.Sprintf(`cr-toggle[aria-label=%q]`, optionName)
	expr := fmt.Sprintf(`
		var optionNode = shadowPiercingQuery(%q);
		if(optionNode == undefined){
			throw new Error("Enable glide typing setting item is not found.");
		}
		optionNode.getAttribute("aria-pressed")==="true";
		`, toggleButtonCSSSelector)

	var isEnabled bool
	if err := s.EvalJSWithShadowPiercer(ctx, cr, expr, &isEnabled); err != nil {
		return isEnabled, errors.Wrapf(err, "failed to get status of option: %q", optionName)
	}
	return isEnabled, nil
}

// WaitUntilToggleOption returns an action to wait until the toggle option enabled or disabled.
func (s *OSSettings) WaitUntilToggleOption(cr *chrome.Chrome, optionName string, expected bool) uiauto.Action {
	return func(ctx context.Context) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			if isEnabled, err := s.IsToggleOptionEnabled(ctx, cr, optionName); err != nil {
				// JS evaluation is not always reliable. So do not break if failed.
				return err
			} else if isEnabled != expected {
				return errors.Errorf("Option %q is unpected: got %v; want %v", optionName, isEnabled, expected)
			}
			return nil
		}, &testing.PollOptions{Timeout: 3 * time.Second})
	}
}

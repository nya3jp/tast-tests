// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package helpapp contains common functions used in the help app.
package helpapp

import (
	"context"

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

// HelpContext represents a context of Help app.
type HelpContext struct {
	cr    *chrome.Chrome
	tconn *chrome.TestConn
	ui    *uiauto.Context
}

// NewContext creates a new context of the Help app.
func NewContext(cr *chrome.Chrome, tconn *chrome.TestConn) *HelpContext {
	return &HelpContext{
		ui:    uiauto.New(tconn),
		tconn: tconn,
		cr:    cr,
	}
}

// RootFinder is the finder of Help app root window.
var RootFinder = nodewith.Name(apps.Help.Name).Role(role.RootWebArea)

// TabFinder is the finder of tabs in Help app.
var TabFinder = nodewith.Role(role.TreeItem).Ancestor(RootFinder)

// Tab names in Help app.
var (
	SearchTabFinder   = TabFinder.Name("Search")
	OverviewTabFinder = TabFinder.Name("Overview")
	PerksTabFinder    = TabFinder.Name("Perks")
	HelpTabFinder     = TabFinder.Name("Help")
	WhatsNewTabFinder = TabFinder.Name("See what's new")
)

// WaitForApp waits for the app to be shown and rendered.
func (hc *HelpContext) WaitForApp() uiauto.Action {
	return hc.ui.WaitUntilExists(OverviewTabFinder)
}

// Launch launches help app and waits for it to be present in shelf.
func (hc *HelpContext) Launch() uiauto.Action {
	app := apps.Help

	return func(ctx context.Context) error {
		if err := apps.Launch(ctx, hc.tconn, app.ID); err != nil {
			return errors.Wrapf(err, "failed to launch %s", app.Name)
		}

		testing.ContextLog(ctx, "Wait for help app shown in shelf")
		if err := ash.WaitForApp(ctx, hc.tconn, app.ID); err != nil {
			return errors.Wrapf(err, "%s did not appear in shelf after launch", app.Name)
		}
		return nil
	}
}

// Exists checks whether the help app exists in the accessiblity tree.
func (hc *HelpContext) Exists(ctx context.Context) (bool, error) {
	return hc.ui.IsNodeFound(ctx, RootFinder)
}

// UIConn returns a connection to the Help app HTML page,
// where JavaScript can be executed to simulate interactions with the UI.
// The caller should close the returned connection.
func (hc *HelpContext) UIConn(ctx context.Context) (*chrome.Conn, error) {
	// Establish a Chrome connection to the Help app and wait for it to finish loading.
	helpAppConn, err := hc.cr.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome-untrusted://help-app/"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get connection to help app")
	}
	if err := helpAppConn.WaitForExpr(ctx, `document.readyState === "complete"`); err != nil {
		return nil, errors.Wrap(err, "failed to wait for help app to finish loading")
	}
	return helpAppConn, nil
}

// EvalJS executes javascript in Help app web page.
func (hc *HelpContext) EvalJS(ctx context.Context, expr string, out interface{}) error {
	conn, err := hc.UIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to web page")
	}
	return conn.Eval(ctx, expr, out)
}

// EvalJSWithShadowPiercer executes javascript in Help app web page.
func (hc *HelpContext) EvalJSWithShadowPiercer(ctx context.Context, expr string, out interface{}) error {
	c, err := hc.UIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to web page")
	}
	return webutil.EvalWithShadowPiercer(ctx, c, expr, out)
}

// LoadTimeData struct for the help app.
// Following fields populated by |ChromeHelpAppUIDelegate::PopulateLoadTimeData|
// https://source.chromium.org/chromium/chromium/src/+/HEAD:chrome/browser/chromeos/web_applications/chrome_help_app_ui_delegate.cc;l=53;drc=c2c84a5ac7711dedcc0b7ff9e79bf7f2da019537.
type LoadTimeData struct {
	IsManagedDevice bool `json:"isManagedDevice"`
}

// GetLoadTimeData returns some of the LoadTimeData fields from the help app.
func (hc *HelpContext) GetLoadTimeData(ctx context.Context) (*LoadTimeData, error) {
	helpAppConn, err := hc.UIConn(ctx)
	if err != nil {
		return nil, err
	}
	defer helpAppConn.Close()
	data := &LoadTimeData{}
	if err := helpAppConn.Eval(ctx, "window.loadTimeData.data_", &data); err != nil {
		return nil, errors.Wrap(err, "failed to evaluate window.loadTimeData.data_")
	}
	return data, nil
}

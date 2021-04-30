// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package helpapp contains common functions used in the help app.
package helpapp

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

var searchInputFinder = nodewith.Name("Search").Role(role.TextField).Ancestor(RootFinder)

// WaitForApp waits for the app to be shown and rendered.
func (hc *HelpContext) WaitForApp() uiauto.Action {
	return hc.ui.WaitUntilExists(RootFinder)
}

// Launch launches help app and waits for it to be present in shelf.
func (hc *HelpContext) Launch() uiauto.Action {
	app := apps.Help

	return uiauto.Combine("launch help app",
		func(ctx context.Context) error {
			if err := apps.Launch(ctx, hc.tconn, app.ID); err != nil {
				return errors.Wrapf(err, "failed to launch %s", app.Name)
			}

			testing.ContextLog(ctx, "Wait for help app shown in shelf")
			if err := ash.WaitForApp(ctx, hc.tconn, app.ID, 30*time.Second); err != nil {
				return errors.Wrapf(err, "%s did not appear in shelf after launch", app.Name)
			}
			return nil
		},
		hc.WaitForApp(),
	)
}

// Exists checks whether the help app exists in the accessiblity tree.
func (hc *HelpContext) Exists(ctx context.Context) (bool, error) {
	return hc.ui.IsNodeFound(ctx, RootFinder)
}

// UIConn returns a connection to the Help app HTML page,
// where JavaScript can be executed to simulate interactions with the UI.
// The caller should close the returned connection. e.g. defer helpAppConn.Close().
func (hc *HelpContext) UIConn(ctx context.Context) (*chrome.Conn, error) {
	return hc.helpConn(ctx, "chrome-untrusted://help-app/")
}

// TrustedUIConn returns a connection to the trusted frame of Help app.
// It has more privileges to access browser functions same as other SWAs.
// The caller should close the returned connection. e.g. defer trustedConn.Close().
func (hc *HelpContext) TrustedUIConn(ctx context.Context) (*chrome.Conn, error) {
	return hc.helpConn(ctx, "chrome://help-app/")
}

func (hc *HelpContext) helpConn(ctx context.Context, urlPrefix string) (*chrome.Conn, error) {
	// Establish a Chrome connection to the Help app and wait for it to finish loading.
	connTargetFilter := func(t *chrome.Target) bool {
		return strings.HasPrefix(t.URL, urlPrefix)
	}
	conn, err := hc.cr.NewConnForTarget(ctx, connTargetFilter)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get connection to help app")
	}
	if err := conn.WaitForExpr(ctx, `document.readyState === "complete"`); err != nil {
		return nil, errors.Wrap(err, "failed to wait for help app to finish loading")
	}
	return conn, nil
}

// EvalJSWithShadowPiercer executes javascript in Help app web page.
func (hc *HelpContext) EvalJSWithShadowPiercer(ctx context.Context, expr string, out interface{}) error {
	helpAppConn, err := hc.UIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to web page")
	}
	defer helpAppConn.Close()
	return webutil.EvalWithShadowPiercer(ctx, helpAppConn, expr, out)
}

// LoadTimeData is a struct for the help app.
// Following fields populated by |ChromeHelpAppUIDelegate::PopulateLoadTimeData|
// https://source.chromium.org/chromium/chromium/src/+/HEAD:chrome/browser/chromeos/web_applications/chrome_help_app_ui_delegate.cc;l=53;drc=c2c84a5ac7711dedcc0b7ff9e79bf7f2da019537.
type LoadTimeData struct {
	IsManagedDevice bool `json:"isManagedDevice"`
}

// GetLoadTimeData returns some of the LoadTimeData fields from the help app.
func (hc *HelpContext) GetLoadTimeData(ctx context.Context) (*LoadTimeData, error) {
	helpAppConn, err := hc.UIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to web page")
	}
	defer helpAppConn.Close()
	data := &LoadTimeData{}
	if err := helpAppConn.Eval(ctx, "window.loadTimeData.data_", &data); err != nil {
		return nil, errors.Wrap(err, "failed to evaluate window.loadTimeData.data_")
	}
	return data, nil
}

// IsHTMLElementPresent checks whether a HTMLElement in help app is present regardless of visibility.
// It takes cssSelector as input param and returns a bool value.
// cssSelector works piercing shadowRoot.
func (hc *HelpContext) IsHTMLElementPresent(ctx context.Context, cssSelector string) (bool, error) {
	var isPresent bool
	expr := fmt.Sprintf(`shadowPiercingQueryAll(%q).length>0;`, cssSelector)
	if err := hc.EvalJSWithShadowPiercer(ctx, expr, &isPresent); err != nil {
		return false, errors.Wrapf(err, "failed to check presence of HTML element: %s", cssSelector)
	}
	return isPresent, nil
}

// NavigateToPageWithURL navigates to a sub page by changing url location directly.
func (hc *HelpContext) NavigateToPageWithURL(url string, condition uiauto.Action) uiauto.Action {
	return func(ctx context.Context) error {
		trustedConn, err := hc.TrustedUIConn(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to connect to help ui")
		}
		defer trustedConn.Close()

		return webutil.NavigateToURLInApp(trustedConn, url, condition, 10*time.Second)(ctx)
	}
}

// NavigateToSearchPage navigates to Search page by changing url via javascript.
func (hc *HelpContext) NavigateToSearchPage() uiauto.Action {
	return hc.NavigateToPageWithURL("chrome://help-app/search", hc.ui.WithTimeout(5*time.Second).WaitUntilExists(searchInputFinder))
}

// ClickSearchInputAndWaitForActive clicks search input field and waits for active.
func (hc *HelpContext) ClickSearchInputAndWaitForActive() uiauto.Action {
	return hc.ui.LeftClickUntil(searchInputFinder, hc.ui.WaitUntilExists(searchInputFinder.Focused()))
}

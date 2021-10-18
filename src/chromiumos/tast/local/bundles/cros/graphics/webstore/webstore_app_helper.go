// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webstore

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/testing"
)

// tab represents a tab on Chrome, providing several APIs to control the tab.
type tab struct {
	// id is an identifier used in chrome.tabs API for this tab.
	id int

	// conn is a connection to the tab.
	conn *chrome.Conn

	// tconn is a connection to the Tast test extension.
	tconn *chrome.TestConn
}

// newTab opens a new tab which loads the url, and return a tab instance.
func newTab(ctx context.Context, cr *chrome.Chrome, url string) (*tab, error) {
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create new renderer")
	}
	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()

	// Because chrome.tabs is not available on the conn, query active tabs
	// assuming there's only one window so only one active tab, and the active tab is
	// the newly created tab, in order to get its TabID.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the connection to the test extension")
	}
	var tabID int
	if err := tconn.Call(ctx, &tabID, `async () => {
	  const tabs = await tast.promisify(chrome.tabs.query)({active: true});
	  if (tabs.length !== 1) {
	    throw new Error("unexpected number of active tabs: got " + tabs.length)
	  }
	  return tabs[0].id;
	}`); err != nil {
		return nil, errors.Wrap(err, "cannot get tab id for the new tab")
	}

	t := &tab{id: tabID, conn: conn, tconn: tconn}
	conn = nil
	return t, nil
}

// close closes the connection to the tab.
func (t *tab) close() error {
	return t.conn.Close()
}

// activate activates the tab, i.e. it selects the tab and brings
// it to the foreground (equivalent to clicking on the tab).
// Returns the duration to perform the switching, or an error is failed.
func (t *tab) activate(ctx context.Context) (time.Duration, error) {
	startTime := time.Now()

	// Request to activate the tab.
	if err := t.tconn.Call(ctx, nil, `async (id) => tast.promisify(chrome.tabs.update)(id, {active: true})`, t.id); err != nil {
		return 0, err
	}

	// Sometimes tabs crash and the devtools connection goes away.  To avoid waiting 30 minutes
	// for this we use a shorter timeout.
	if err := webutil.WaitForRender(ctx, t.conn, 30*time.Second); err != nil {
		return 0, err
	}

	elapsed := time.Now().Sub(startTime)
	return elapsed, nil
}

// App contains info about a Chrome Web Store app. All fields are required.
type App struct {
	Name         string
	URL          string
	InstalledTxt string
	AddTxt       string
	ConfirmTxt   string
}

// pollOpts is the polling interval and timeout to be used on the Chrome Web Store.
var pollOpts = &testing.PollOptions{Interval: time.Second, Timeout: 1 * time.Minute}

// InstallWebstoreApp install/uninstall the specified Chrome app from the Chrome Web Store.
func InstallWebstoreApp(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, app App) error {
	tab := &tab{}
	if app.URL != "" {
		nTab, err := newTab(ctx, cr, app.URL)
		if err != nil {
			return err
		}
		tab = nTab
		defer tab.close()
	}

	ui := uiauto.New(tconn)
	installed := nodewith.Name(app.InstalledTxt).Role(role.Button).First()
	add := nodewith.Name(app.AddTxt).Role(role.Button).First()
	confirm := nodewith.Name(app.ConfirmTxt).Role(role.Button)

	// Click the add button at most once to prevent triggering
	// weird UI behaviors in Chrome Web Store.
	addClicked := false
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Check if the app is installed.
		if err := ui.Exists(installed)(ctx); err == nil {
			return nil
		}

		if !addClicked {
			// If the app is not installed, install it now.
			// Click on the add button, if it exists.
			if err := ui.Exists(add)(ctx); err == nil {
				if err := ui.LeftClick(add)(ctx); err != nil {
					return testing.PollBreak(err)
				}
				addClicked = true
			}
		}
		// Click on the confirm button, if it exists.
		if err := ui.IfSuccessThen(ui.Exists(confirm), ui.LeftClick(confirm))(ctx); err != nil {
			return testing.PollBreak(err)
		}
		if app.URL != "" {
			if _, err := tab.activate(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to switch to main tab")
			}
		}
		return errors.Errorf("%s still installing", app.Name)
	}, pollOpts); err != nil {
		return errors.Wrapf(err, "failed to install %s", app.Name)
	}
	return nil
}

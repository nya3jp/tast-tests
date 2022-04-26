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
func newTab(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, url string) (*tab, error) {
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
// Returns an error, if failed.
func (t *tab) activate(ctx context.Context) (bool, error) {
	// Request to activate the tab.
	if err := t.tconn.Call(ctx, nil, `async (id) => tast.promisify(chrome.tabs.update)(id, {active: true})`, t.id); err != nil {
		return false, err
	}

	// Sometimes tabs crash and the devtools connection goes away.  To avoid waiting 30 seconds
	// for this we use a shorter timeout.
	if err := webutil.WaitForRender(ctx, t.conn, 30*time.Second); err != nil {
		return false, err
	}
	return true, nil
}

// App contains info about a Chrome Web Store app. All fields are required.
type App struct {
	Name          string // Name of the Chrome app.
	URL           string // URL to install the app from.
	VerifyText    string // Button text after the app is installed/uninstalled.
	AddRemoveText string // Button text when the app is available to be added.
	ConfirmText   string // Button text to confirm the installation.
}

// UpgradeWebstoreApp install/uninstall the specified Chrome app from the Chrome Web Store.
func UpgradeWebstoreApp(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, app App) error {
	if app.URL == "" {
		return errors.New("URL must be non-empty")
	}
	tab, err := newTab(ctx, cr, tconn, app.URL)
	if err != nil {
		return err
	}
	defer tab.close()

	ui := uiauto.New(tconn)
	upgraded := nodewith.Name(app.VerifyText).Role(role.Button).First()
	addRemove := nodewith.Name(app.AddRemoveText).Role(role.Button).First()
	confirm := nodewith.Name(app.ConfirmText).Role(role.Button)
	checkingButton := nodewith.Name("Checking...").Role(role.Button)

	// Wait for addRemove element, if found then click the addRemove button.
	if err := uiauto.IfSuccessThen(ui.WaitUntilExists(addRemove), ui.LeftClick(addRemove))(ctx); err != nil {
		return errors.Wrap(err, "failed to wait and left click addRemove element")
	}

	// Wait and Click on the confirm button.
	if err := ui.WaitForLocation(confirm)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for confirm button")
	}
	if err := ui.LeftClick(confirm)(ctx); err != nil {
		return errors.Wrap(err, "failed to left click confirm button")
	}

	// Wait for the checking button gets gone after successfully installing app.
	if err := ui.WithTimeout(time.Minute).WaitUntilGone(checkingButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for checkingButton to be gone")
	}

	// After upgrading the Webstore app by pressing the Confirm button,
	// a new tab opens with the installed app.
	// Hence we are moving to the Webstore tab to check if the app got upgraded or not.
	if a, err := tab.activate(ctx); err != nil {
		return errors.Wrap(err, "failed to activate tab")
	} else if !a {
		return errors.New("main tab is not activated")
	}

	// Check if the app is upgraded or not.
	if err := ui.WaitUntilExists(upgraded)(ctx); err != nil {
		return errors.Wrap(err, "failed to find UI element")
	}
	return nil
}

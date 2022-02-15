// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cursive contains common functions used in the Cursive app.
package cursive

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// appURL is the url of Cursive PWA.
const appURL = "https://cursive.apps.chrome/"

// UIConn returns a connection to the Cursive app HTML page,
// where JavaScript can be executed to simulate interactions with the UI.
// The caller should close the returned connection. e.g. defer uiConn.Close().
func UIConn(ctx context.Context, cr *chrome.Chrome) (*chrome.Conn, error) {
	// Establish a Chrome connection to the Calculator app and wait for it to finish loading.
	appConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(appURL))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to target %q", appURL)
	}
	if err := appConn.WaitForExpr(ctx, `document.readyState === "complete"`); err != nil {
		return nil, errors.Wrap(err, "failed to wait for Calculator app to finish loading")
	}
	return appConn, nil
}

// Install installs Cursive from the app URL.
func Install(cr *chrome.Chrome) uiauto.Action {
	return func(ctx context.Context) error {
		_, err := apps.InstallPWAForURL(ctx, cr, appURL, 2*time.Minute)
		return err
	}
}

// WaitForAppRendered waits until the app is rendered by checking page heading.
func WaitForAppRendered(tconn *chrome.TestConn) uiauto.Action {
	cursiveHeadingFinder := nodewith.Name("Note - Cursive").Role(role.RootWebArea)
	return uiauto.NamedAction("Waiting for Cursive to be rendered",
		uiauto.New(tconn).WaitUntilExists(cursiveHeadingFinder))
}

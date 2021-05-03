// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package galleryapp contains common functions used in the Gallery (aka Backlight) app.
package galleryapp

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
)

// GalleryContext represents a context of Gallery app.
type GalleryContext struct {
	cr    *chrome.Chrome
	tconn *chrome.TestConn
	ui    *uiauto.Context
}

// NewContext creates a new context of the Gallery app.
func NewContext(cr *chrome.Chrome, tconn *chrome.TestConn) *GalleryContext {
	return &GalleryContext{
		ui:    uiauto.New(tconn),
		tconn: tconn,
		cr:    cr,
	}
}

// RootFinder is the finder of Gallery app root window.
var RootFinder = nodewith.Name(apps.Gallery.Name).Role(role.RootWebArea)

// DialogFinder is the finder of popup dialog in Gallery app.
var DialogFinder = nodewith.Role(role.AlertDialog).Ancestor(RootFinder)

// openImageButtonFinder is the finder of 'Open image' button on zero state page.
var openImageButtonFinder = nodewith.Role(role.Button).Name("Open image").Ancestor(RootFinder)

// CloseApp returns an action closing Gallery app.
func (gc *GalleryContext) CloseApp() uiauto.Action {
	return func(ctx context.Context) error {
		return apps.Close(ctx, gc.tconn, apps.Gallery.ID)
	}
}

// DeleteAndConfirm returns an action clicking 'Delete' button and then 'Confirm' to remove current opened media file.
// It assumes a valid media file is opened.
func (gc *GalleryContext) DeleteAndConfirm() uiauto.Action {
	deleteButtonFinder := nodewith.Role(role.Button).Name("Delete").Ancestor(RootFinder)
	confirmButtonFinder := nodewith.Role(role.Button).Name("Delete").Ancestor(DialogFinder)
	return uiauto.Combine("remove current opened media file",
		gc.ui.WithTimeout(30*time.Second).WithInterval(1*time.Second).LeftClickUntil(
			deleteButtonFinder, gc.ui.WithTimeout(3*time.Second).WaitUntilExists(confirmButtonFinder)),
		gc.ui.LeftClick(confirmButtonFinder),
	)
}

// AssertZeroState asserts that Gallery is on 'open image' page.
func (gc *GalleryContext) AssertZeroState() uiauto.Action {
	return gc.ui.WaitUntilExists(openImageButtonFinder)
}

// UIConn returns a connection to the Gallery app HTML page,
// where JavaScript can be executed to simulate interactions with the UI.
// The caller should close the returned connection. e.g. defer galleryAppConn.Close().
func (gc *GalleryContext) UIConn(ctx context.Context) (*chrome.Conn, error) {
	// Establish a Chrome connection to the Gallery app and wait for it to finish loading.
	galleryAppConn, err := gc.cr.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome-untrusted://media-app/"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get connection to Gallery app")
	}
	if err := galleryAppConn.WaitForExpr(ctx, `document.readyState === "complete"`); err != nil {
		return nil, errors.Wrap(err, "failed to wait for Gallery app to finish loading")
	}
	return galleryAppConn, nil
}

// EvalJSWithShadowPiercer executes javascript in Gallery app web page.
func (gc *GalleryContext) EvalJSWithShadowPiercer(ctx context.Context, expr string, out interface{}) error {
	galleryAppConn, err := gc.UIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to web page")
	}
	defer galleryAppConn.Close()
	return webutil.EvalWithShadowPiercer(ctx, galleryAppConn, expr, out)
}

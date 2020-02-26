// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package apps provides general ChromeOS app utilities.
package apps

import (
	"context"
	"fmt"

	"chromiumos/tast/local/chrome"
)

// App is used to represent a ChromeOS app.
type App struct {
	// ID is the Chrome extension ID of the app.
	ID string
	// Name is the name of the app.
	Name string
}

// Chrome has details about the Chrome app.
var Chrome = App{
	ID:   "mgndgikekgjfcpckkfioiadnlibdjbkf",
	Name: "Google Chrome",
}

// Files has details about the Files app.
var Files = App{
	ID:   "hhaomjibdihmijegdhdafkllkbggdgoj",
	Name: "Files",
}

// PlayStore has details about the Play Store app.
var PlayStore = App{
	ID:   "cnbgggchhmkkdmeppjobngjoejnihlei",
	Name: "Play Store",
}

// Settings has details about the Settings app.
var Settings = App{
	ID:   "odknhmnlageboeamepcngndbggdpaobj",
	Name: "Settings",
}

// WallpaperPicker has details about the Wallpaper Picker app.
var WallpaperPicker = App{
	ID:   "obklkkbkpaoaejdabbfldmcfplpdgolj",
	Name: "Wallpaper Picker",
}

// Launch launches an app specified by appID.
func Launch(ctx context.Context, tconn *chrome.TestConn, appID string) error {
	query := fmt.Sprintf("tast.promisify(chrome.autotestPrivate.launchApp)(%q)", appID)
	return tconn.EvalPromise(ctx, query, nil)
}

// Close closes an app specified by appID.
func Close(ctx context.Context, tconn *chrome.TestConn, appID string) error {
	query := fmt.Sprintf("tast.promisify(chrome.autotestPrivate.closeApp)(%q)", appID)
	return tconn.EvalPromise(ctx, query, nil)
}

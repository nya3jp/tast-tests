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

// WallpaperPicker has details about the Wallpaper Picker app.
var WallpaperPicker = App{
	ID:   "obklkkbkpaoaejdabbfldmcfplpdgolj",
	Name: "Wallpaper Picker",
}

// LaunchApp launches an app specified by appID.
func LaunchApp(ctx context.Context, tconn *chrome.Conn, appID string) error {
	launchQuery := fmt.Sprintf("tast.promisify(chrome.autotestPrivate.launchApp)(%q)", appID)
	return tconn.EvalPromise(ctx, launchQuery, nil)
}

// CloseApp closes an app specified by appID.
func CloseApp(ctx context.Context, tconn *chrome.Conn, appID string) error {
	closeQuery := fmt.Sprintf("tast.promisify(chrome.autotestPrivate.closeApp)(%q)", appID)
	return tconn.EvalPromise(ctx, closeQuery, nil)
}

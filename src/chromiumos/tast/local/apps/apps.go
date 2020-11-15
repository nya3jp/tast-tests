// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package apps provides general ChromeOS app utilities.
package apps

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
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

// Camera has details about the Camera app.
var Camera = App{
	ID:   "hfhhnacclhffhdffklopdkcgdhifgngh",
	Name: "Camera",
}

// Canvas has details about the Chrome Canvas app.
var Canvas = App{
	ID:   "ieailfmhaghpphfffooibmlghaeopach",
	Name: "Chrome Canvas",
}

// Diagnostics has details about Diagnostics SWA.
var Diagnostics = App{
	ID:   "keejpcfcpecjhmepmpcfgjemkmlicpam",
	Name: "Diagnostics",
}

// Duo has details about the Duo app.
var Duo = App{
	ID:   "djkcbcmkefiiphjkonbeknmcgiheajce",
	Name: "Duo",
}

// Files has details about the Files app.
var Files = App{
	ID:   "hhaomjibdihmijegdhdafkllkbggdgoj",
	Name: "Files",
}

// Help (aka Explore) has details about the Help app.
var Help = App{
	ID:   "nbljnnecbjbmifnoehiemkgefbnpoeak",
	Name: "Explore",
}

// Gallery (aka Backlight) has details about the Gallery app.
var Gallery = App{
	ID:   "jhdjimmaggjajfjphpljagpgkidjilnj",
	Name: "Gallery",
}

// PlayBooks has details about the Play Books app.
var PlayBooks = App{
	ID:   "cafegjnmmjpfibnlddppihpnkbkgicbg",
	Name: "Play Books",
}

// PlayGames has details about the Play Games app.
var PlayGames = App{
	ID:   "nplnnjkbeijcggmpdcecpabgbjgeiedc",
	Name: "Play Games",
}

// PlayMovies has details about the Play Movies & TV app.
var PlayMovies = App{
	ID:   "dbbihmicnlldbflflckpafphlekmjfnm",
	Name: "Play Movies & TV",
}

// PlayStore has details about the Play Store app.
var PlayStore = App{
	ID:   "cnbgggchhmkkdmeppjobngjoejnihlei",
	Name: "Play Store",
}

// PrintManagement has details about the Print Management app.
var PrintManagement = App{
	ID:   "fglkccnmnaankjodgccmiodmlkpaiodc",
	Name: "Print Jobs",
}

// Scan has details about the Scan SWA.
var Scan = App{
	ID:   "cdkahakpgkdaoffdmfgnhgomkelkocfo",
	Name: "Scan",
}

// Settings has details about the Settings app.
var Settings = App{
	ID:   "odknhmnlageboeamepcngndbggdpaobj",
	Name: "Settings",
}

// TelemetryExtension has details about the TelemetryExtension app.
var TelemetryExtension = App{
	ID:   "lhoocnmbcmmbjgdeaallonfplogkcneb",
	Name: "Telemetry Extension",
}

// Terminal has details about the Crostini Terminal app.
var Terminal = App{
	ID:   "fhicihalidkgcimdmhpohldehjmcabcf",
	Name: "Terminal",
}

// WallpaperPicker has details about the Wallpaper Picker app.
var WallpaperPicker = App{
	ID:   "obklkkbkpaoaejdabbfldmcfplpdgolj",
	Name: "Wallpaper Picker",
}

// WebStore has details about the WebStore app.
var WebStore = App{
	ID:   "ahfgeienlihckogmohjhadlkjgocpleb",
	Name: "Web Store",
}

// Launch launches an app specified by appID.
func Launch(ctx context.Context, tconn *chrome.TestConn, appID string) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		capps, err := ash.ChromeApps(ctx, tconn)
		if err != nil {
			testing.PollBreak(err)
		}
		for _, capp := range capps {
			if capp.AppID == appID {
				return nil
			}
		}
		return errors.New("App not yet found in available Chrome apps")
	}, nil); err != nil {
		return err
	}
	return tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.launchApp)`, appID)
}

// LaunchSystemWebApp launches a system web app specifide by its name and URL.
func LaunchSystemWebApp(ctx context.Context, tconn *chrome.TestConn, appName, url string) error {
	return tconn.Call(ctx, nil, `async (appName, url) => {
		await tast.promisify(chrome.autotestPrivate.waitForSystemWebAppsInstall)();
		await tast.promisify(chrome.autotestPrivate.launchSystemWebApp)(appName, url);
	}`, appName, url)
}

// Close closes an app specified by appID.
func Close(ctx context.Context, tconn *chrome.TestConn, appID string) error {
	return tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.closeApp)`, appID)
}

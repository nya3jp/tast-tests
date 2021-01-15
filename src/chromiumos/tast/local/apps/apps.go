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
	Name: "Chrome",
}

// Chromium has details about the Chromium app.
// It replaces Chrome on amd64-generic builds.
var Chromium = App{
	ID:   "mgndgikekgjfcpckkfioiadnlibdjbkf",
	Name: "Chromium",
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

// Docs has details about the Google Docs app.
var Docs = App{
	ID:   "aohghmighlieiainnegkcijnfilokake",
	Name: "Docs",
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

// Gallery (aka Backlight) has details about the Gallery app.
var Gallery = App{
	ID:   "jhdjimmaggjajfjphpljagpgkidjilnj",
	Name: "Gallery",
}

// Gmail has details about the gmail app.
var Gmail = App{
	ID:   "hhkfkjpmacfncmbapfohfocpjpdnobjg",
	Name: "Gmail",
}

// Help (aka Explore) has details about the Help app.
var Help = App{
	ID:   "nbljnnecbjbmifnoehiemkgefbnpoeak",
	Name: "Explore",
}

// Lacros has details about Lacros browser app.
var Lacros = App{
	ID:   "jaimifaeiicidiikhmjedcgdimealfbh",
	Name: "Lacros",
}

// Maps has details about Arc Maps app.
var Maps = App{
	ID:   "gmhipfhgnoelkiiofcnimehjnpaejiel",
	Name: "Maps",
}

// Photos has details about the Photos app.
var Photos = App{
	ID:   "fdbkkojdbojonckghlanfaopfakedeca",
	Name: "Photos",
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

// Clock has details about the Clock app.
var Clock = App{
	ID:   "ddmmnabaeomoacfpfjgghfpocfolhjlg",
	Name: "Clock",
}

// Contacts has details about the Contacts app.
var Contacts = App{
	ID:   "kipfkokfekalckplgaikemhghlbkgpfl",
	Name: "Contacts",
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

// Youtube has details about the Youtube app.
var Youtube = App{
	ID:   "aniolghapcdkoolpkffememnhpphmjkl",
	Name: "Youtube",
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

// LaunchOSSettings launches the OS Settings app to its subpage URL, and returns
// a connection to it. When this method returns, OS Settings page has finished
// loading.
//
// This method is necessary because OS Settings now uses System Web App link
// capturing, which doesn't work with Devtools protocol CreateTarget.
//
// Note, `url` needs to exactly match the page OS Settings ends up navigating to.
// For example, chrome://os-settings/.
func LaunchOSSettings(ctx context.Context, cr *chrome.Chrome, url string) (*chrome.Conn, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect Test API")
	}

	if LaunchSystemWebApp(ctx, tconn, "OSSettings", url); err != nil {
		return nil, errors.Wrap(err, "failed to launch OS Settings")
	}

	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(url))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get connection to os settings")
	}

	if conn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
		return nil, errors.Wrap(err, "failed to wait for document load")
	}

	return conn, nil
}

// Close closes an app specified by appID.
func Close(ctx context.Context, tconn *chrome.TestConn, appID string) error {
	return tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.closeApp)`, appID)
}

// ChromeOrChromium returns the correct browser for the current build.
// Chromium is returned on non branded builds (e.g amd64-generic).
func ChromeOrChromium(ctx context.Context, tconn *chrome.TestConn) (App, error) {
	capps, err := ash.ChromeApps(ctx, tconn)
	if err != nil {
		return App{}, errors.Wrap(err, "failed to get list of installed apps")
	}
	for _, app := range capps {
		if app.AppID == Chrome.ID {
			if app.Name == Chrome.Name {
				return Chrome, nil
			}
			return Chromium, nil
		}
	}
	return App{}, errors.New("Neither Chrome or Chromium were found in available apps")
}

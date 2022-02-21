// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package apps provides general ChromeOS app utilities.
package apps

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

// App is used to represent a ChromeOS app.
type App struct {
	// ID is the Chrome extension ID of the app.
	ID string
	// Name is the name of the app.
	Name string
}

// Borealis App represents the installer/launcher for the borealis.
var Borealis = App{
	ID:   "dkecggknbdokeipkgnhifhiokailichf",
	Name: "Borealis",
}

// Chat App has details about the Google Chat app.
var Chat = App{
	ID:   "mhihbbhgcjldimhaopinoigbbglkihll",
	Name: "Google Chat",
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
	ID:   "njfbnohfdkmbmnjapinfcopialeghnmh",
	Name: "Camera",
}

// Canvas has details about the Chrome Canvas app.
var Canvas = App{
	ID:   "ieailfmhaghpphfffooibmlghaeopach",
	Name: "Chrome Canvas",
}

// Cursive has details about the Cursive app.
var Cursive = App{
	ID:   "apignacaigpffemhdbhmnajajaccbckh",
	Name: "Cursive",
}

// ConnectivityDiagnostics has details about the Chrome Connectivity Diagnostics
// app.
var ConnectivityDiagnostics = App{
	ID:   "pinjbkpghjkgmlmfidajjdjocdpegjkg",
	Name: "Connectivity Diagnostics",
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

// Drive has details about the Google Drive app.
var Drive = App{
	ID:   "apdfllckaahabafndbhieahigkjlhalf",
	Name: "Google Drive",
}

// Duo has details about the Duo app.
var Duo = App{
	ID:   "djkcbcmkefiiphjkonbeknmcgiheajce",
	Name: "Duo",
}

// FamilyLink has details about the Family Link app.
var FamilyLink = App{
	ID:   "mljomdcpdfpfdplmgghfeoofmbbianlf",
	Name: "Family Link",
}

// Files has details about the Files Chrome app.
var Files = App{
	ID:   "hhaomjibdihmijegdhdafkllkbggdgoj",
	Name: "Files",
}

// FilesSWA has details about the Files System Web App.
var FilesSWA = App{
	ID:   "fkiggjmkendpmbegkagpmagjepfkpmeb",
	Name: "Files",
}

// FirmwareUpdate has details about the FirmwareUpdate SWA.
var FirmwareUpdate = App{
	ID:   "nedcdcceagjbkiaecmdbpafcmlhkiifa",
	Name: "Firmware Updates",
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

// Calculator has details about the Calculator app.
var Calculator = App{
	ID:   "oabkinaljpjeilageghcdlnekhphhphl",
	Name: "Calculator",
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
	Name: "Print jobs",
}

// Scan has details about the Scan SWA.
var Scan = App{
	ID:   "cdkahakpgkdaoffdmfgnhgomkelkocfo",
	Name: "Scan",
}

// AndroidSettings has details about ARC settings app.
var AndroidSettings = App{
	ID:   "mconboelelhjpkbdhhiijkgcimoangdj",
	Name: "Android Settings",
}

// Settings has details about the Settings app.
var Settings = App{
	ID:   "odknhmnlageboeamepcngndbggdpaobj",
	Name: "Settings",
}

// ShimlessRMA has details about the Shimless RMA app.
var ShimlessRMA = App{
	ID:   "ijolhdommgkkhpenofmpkkhlepahelcm",
	Name: "Shimless RMA",
}

// Translate has details about the Translate app.
var Translate = App{
	ID:   "pacmnfddiadhhfmngijgjdbnodjkmojl",
	Name: "Translate",
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

// Parallels has details about the Parallels app.
var Parallels = App{
	ID:   "lgjpclljbbmphhnalkeplcmnjpfmmaek",
	Name: "Parallels Desktop",
}

// Citrix has details about Citrix Workspace app.
var Citrix = App{
	ID:   "haiffjcadagjlijoggckpgfnoeiflnem",
	Name: "Citrix Workspace",
}

// VMWare has details about VMware Horizon app.
var VMWare = App{
	ID:   "ppkfnjlimknmjoaemnpidmdlfchhehel",
	Name: "VMware Horizon",
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

// ListSystemWebApps retrieves a list of installed apps and filters down the system web apps.
func ListSystemWebApps(ctx context.Context, tconn *chrome.TestConn) ([]*ash.ChromeApp, error) {
	if err := tconn.Call(ctx, nil, "tast.promisify(chrome.autotestPrivate.waitForSystemWebAppsInstall)"); err != nil {
		return nil, errors.Wrap(err, "failed to wait for all system web apps to be installed")
	}

	chromeApps, err := ash.ChromeApps(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get list of Chrome apps")
	}

	var systemWebApps []*ash.ChromeApp
	for _, app := range chromeApps {
		// Terminal has special handling in App Service, it has type 'Crostini' and install source 'User'.
		if (app.InstallSource == "System" && app.Type == "Web") || (app.InstallSource == "User" && app.Type == "Crostini") {
			systemWebApps = append(systemWebApps, app)
		}
	}

	return systemWebApps, nil
}

// ListSystemWebAppsInternalNames returns a string[] that contains system app's internal names.
// It queries System Web App Manager via Test API.
func ListSystemWebAppsInternalNames(ctx context.Context, tconn *chrome.TestConn) ([]string, error) {
	var result []string
	err := tconn.Eval(
		ctx,
		`new Promise((resolve, reject) => {
			chrome.autotestPrivate.getRegisteredSystemWebApps((system_apps) => {
				if (chrome.runtime.lastError) {
					reject(new Error(chrome.runtime.lastError.message));
					return;
				}

				resolve(system_apps.map(system_app => system_app.internalName));
			});
		});`, &result)

	if err != nil {
		return nil, errors.Wrap(err, "failed to get result from Test API")
	}

	return result, nil
}

// LaunchOSSettings launches the OS Settings app to its subpage URL, and returns
// a connection to it. When this method returns, OS Settings page has finished
// loading.
//
// This method is necessary because OS Settings now uses System Web App link
// capturing, which doesn't work with DevTools protocol CreateTarget.
//
// Note, `url` needs to exactly match the page OS Settings ends up navigating to.
// For example, chrome://os-settings/.
func LaunchOSSettings(ctx context.Context, cr *chrome.Chrome, url string) (*chrome.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect Test API")
	}

	if LaunchSystemWebApp(ctx, tconn, "OSSettings", url); err != nil {
		return nil, errors.Wrap(err, "failed to launch OS Settings")
	}

	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(url))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get connection to OS Settings")
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

// PrimaryBrowser returns the primary browser for the current build.
// 'Chromium' on non branded ash-chrome builds (e.g amd64-generic)
// 'Chrome' on branded ash-chrome builds
// 'Lacros' on branded lacros-chrome builds (eg, lacros64)
func PrimaryBrowser(ctx context.Context, tconn *chrome.TestConn, bt browser.Type) (App, error) {
	switch bt {
	case browser.TypeAsh:
		browserApp, err := ChromeOrChromium(ctx, tconn)
		if err != nil {
			return App{}, errors.Wrap(err, "failed to find the browser app for ash-chrome")
		}
		return browserApp, nil
	case browser.TypeLacros:
		return Lacros, nil
	}
	return App{}, errors.Errorf("no primary browser was found for the type (%v) in available apps", bt)
}

// InstallPWAForURL navigates to a PWA, attempts to install and returns the installed app ID.
func InstallPWAForURL(ctx context.Context, cr *chrome.Chrome, pwaURL string, timeout time.Duration) (string, error) {
	conn, err := cr.NewConn(ctx, pwaURL)
	if err != nil {
		return "", errors.Wrapf(err, "failed to open URL %q", pwaURL)
	}
	defer conn.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to connect to test API")
	}

	// The installability checks occur asynchronously for PWAs.
	// Wait for the Install button to appear in the Chrome omnibox before installing.
	ui := uiauto.New(tconn)
	install := nodewith.ClassName("PwaInstallView").Role(role.Button)
	if err := ui.WithTimeout(timeout).WaitUntilExists(install)(ctx); err != nil {
		return "", errors.Wrap(err, "failed to wait for the install button in the omnibox")
	}

	evalString := fmt.Sprintf("tast.promisify(chrome.autotestPrivate.installPWAForCurrentURL)(%d)", timeout.Milliseconds())

	var appID string
	if err := tconn.Eval(ctx, evalString, &appID); err != nil {
		return "", errors.Wrap(err, "failed to run installPWAForCurrentURL")
	}

	return appID, nil
}

// LaunchChromeByShortcut launches a new Chrome window in either normal user mode by shortcut `Ctl+N`
// or incognito mode by shortcut `Ctl+Shift+N`.
func LaunchChromeByShortcut(tconn *chrome.TestConn, incognitoMode bool) action.Action {
	return func(ctx context.Context) error {
		return tconn.Call(ctx, nil, `async (incognito) => {
			let accelerator = {keyCode: 'n', shift: incognito, control: true, alt: false, search: false, pressed: true};
			await tast.promisify(chrome.autotestPrivate.activateAccelerator)(accelerator);
			accelerator.pressed = false;
			await tast.promisify(chrome.autotestPrivate.activateAccelerator)(accelerator);
		}`, incognitoMode)
	}
}

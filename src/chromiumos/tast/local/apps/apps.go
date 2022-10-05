// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package apps provides general ChromeOS app utilities.
package apps

import (
	"context"
	"net/url"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros/lacrosinfo"
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

// Crosh has details about Crosh SWA.
var Crosh = App{
	ID:   "cgfnfgkafmcdkdgilmojlnaadileaach",
	Name: "Crosh",
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

// Drive has details about the Google Drive Web app.
var Drive = App{
	ID:   "aghbiahbpaijignceidepookljebhfak",
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

// Feedback has details about the Feedback app.
var Feedback = App{
	ID:   "iffgohomcomlpmkfikfffagkkoojjffm",
	Name: "Feedback",
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

// LacrosID is the ID of the Lacros browser app.
const LacrosID = "jaimifaeiicidiikhmjedcgdimealfbh"

// LacrosPrimaryLacros has details about the Lacros browser app when the Ash browser is enabled as well,
// i.e. in LacrosPrimary mode (or in the deprecated LacrosSideBySide mode).
var LacrosPrimaryLacros = App{
	ID:   LacrosID,
	Name: "Lacros",
}

// LacrosOnlyLacros has details about the Lacros browser app when the Ash browser is disabled,
// i.e. in LacrosOnly mode.
var LacrosOnlyLacros = App{
	ID:   LacrosID,
	Name: "Chrome",
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

// GoogleTV has details about the Google TV app.
var GoogleTV = App{
	ID:   "kadljooblnjdohjelobhphgeimdbcpbo",
	Name: "Google TV",
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

// TaskManager has details about the Task Manager app.
var TaskManager = App{
	ID:   "ijaigheoohcacdnplfbdimmcfldnnhdi",
	Name: "Task Manager",
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

// YouTubeCWS has details about the YouTube app from Chrome Web Store.
var YouTubeCWS = App{
	ID:   "blpcfgokakmgnkcojhhkbfbldkacnbeo",
	Name: "YouTube",
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

// Projector has details about the Screencast app.
var Projector = App{
	ID:   "nblbgfbmjfjaeonhjnbbkabkdploocij",
	Name: "Screencast",
}

// KeyboardSV has details about the Keyboard Shortcut Viewer app.
var KeyboardSV = App{
	ID:   "bhbpmkoclkgbgaefijcdgkfjghcmiijm",
	Name: "Keyboard Shortcut Viewer",
}

// Launch launches an app specified by appID.
func Launch(ctx context.Context, tconn *chrome.TestConn, appID string) error {
	_, err := getInstalledAppID(ctx, tconn, func(app *ash.ChromeApp) bool { return app.AppID == appID }, nil)
	if err != nil {
		return err
	}
	return tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.launchApp)`, appID)
}

func getInstalledAppID(ctx context.Context, tconn *chrome.TestConn, predicate func(*ash.ChromeApp) bool, pollOpts *testing.PollOptions) (string, error) {
	appID := ""
	err := testing.Poll(ctx, func(ctx context.Context) error {
		capps, err := ash.ChromeApps(ctx, tconn)
		if err != nil {
			return testing.PollBreak(err)
		}
		for _, capp := range capps {
			if predicate(capp) {
				appID = capp.AppID
				return nil
			}
		}
		return errors.New("App not yet found in available Chrome apps - have you added --enable-features=<app> to chrome options?")
	}, pollOpts)
	return appID, err
}

// FindSystemWebAppByOrigin returns an `ash.ChromeApp` that is a system web app and matches `origin`.
// This won't match Terminal app, which is managed by Crostini and tested separately.
// Returns `nil` if the app isn't found (i.e. installed).
func FindSystemWebAppByOrigin(ctx context.Context, tconn *chrome.TestConn, origin string) (*ash.ChromeApp, error) {
	installedApps, err := ash.ChromeApps(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get installed apps")
	}

	swaURL, err := url.Parse(origin)
	if err != nil {
		return nil, errors.Wrapf(err, "system web app origin is invalid, got %s", origin)
	}

	for _, app := range installedApps {
		if app.InstallSource == "System" && app.Type == "Web" {
			// SWA's `publisher_id` is their start_url, match it against the provided `origin`.
			appURL, _ := url.Parse(app.PublisherID)
			if appURL != nil && appURL.Scheme == swaURL.Scheme && appURL.Host == swaURL.Host {
				return app, nil
			}
		}
	}
	return nil, nil
}

// LaunchSystemWebApp launches a system web app specifide by its name and URL.
func LaunchSystemWebApp(ctx context.Context, tconn *chrome.TestConn, appName, url string) error {
	return tconn.Call(ctx, nil, `async (appName, url) => {
		await tast.promisify(chrome.autotestPrivate.waitForSystemWebAppsInstall)();
		await tast.promisify(chrome.autotestPrivate.launchSystemWebApp)(appName, url);
	}`, appName, url)
}

// SystemWebApp corresponds to `SystemWebApp` defined in autotest_private.idl
type SystemWebApp struct {
	InternalName string `json:"internalName"`
	URL          string `json:"url"`
	Name         string `json:"name"`
	StartURL     string `json:"startUrl"`
}

// ListRegisteredSystemWebApps returns all registered system web apps.
func ListRegisteredSystemWebApps(ctx context.Context, tconn *chrome.TestConn) ([]*SystemWebApp, error) {
	var s []*SystemWebApp
	if err := tconn.Call(ctx, &s, "tast.promisify(chrome.autotestPrivate.getRegisteredSystemWebApps)"); err != nil {
		return nil, errors.Wrap(err, "failed to call getRegisteredSystemWebApps")
	}
	return s, nil
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
	return App{}, errors.New("Neither Chrome nor Chromium were found in available apps")
}

// Lacros returns the Lacros app details for the current system configuration.
// The result (on success) is either LacrosPrimaryLacros or LacrosOnlyLacros.
// The given TestConn must be a connection to Ash.
func Lacros(ctx context.Context, tconn *chrome.TestConn) (App, error) {
	lacrosInfo, err := lacrosinfo.Snapshot(ctx, tconn)
	if err != nil {
		return App{}, errors.Wrap(err, "failed to get lacros info")
	}
	switch lacrosInfo.Mode {
	case lacrosinfo.LacrosModeDisabled:
		return App{}, errors.New("Lacros app requested but Lacros is disabled")
	case lacrosinfo.LacrosModeSideBySide, lacrosinfo.LacrosModePrimary:
		return LacrosPrimaryLacros, nil
	case lacrosinfo.LacrosModeOnly:
		return LacrosOnlyLacros, nil
	}
	return App{}, errors.Wrapf(err, "unexpected LacrosMode: %v", lacrosInfo.Mode)
}

// PrimaryBrowser returns the primary browser for the current system configuration.
// In LacrosPrimary and LacrosOnly configurations, it behaves the same as the Lacros function above.
// Otherwise it returns 'Chrome' or 'Chromium' depending on branding.
// The given TestConn must be a connection to Ash.
func PrimaryBrowser(ctx context.Context, tconn *chrome.TestConn) (App, error) {
	lacrosInfo, err := lacrosinfo.Snapshot(ctx, tconn)
	if err != nil {
		return App{}, errors.Wrap(err, "failed to get lacros info")
	}
	switch lacrosInfo.Mode {
	case lacrosinfo.LacrosModeDisabled, lacrosinfo.LacrosModeSideBySide:
		return ChromeOrChromium(ctx, tconn)
	case lacrosinfo.LacrosModePrimary:
		return LacrosPrimaryLacros, nil
	case lacrosinfo.LacrosModeOnly:
		return LacrosOnlyLacros, nil
	}
	return App{}, errors.Wrapf(err, "unexpected LacrosMode: %v", lacrosInfo.Mode)
}

// InstallPWAForURL navigates to a PWA and attempts to install it.
// The given TestConn must be a connection to Ash.
func InstallPWAForURL(ctx context.Context, tconn *chrome.TestConn, br *browser.Browser, pwaURL string, timeout time.Duration) error {
	conn, err := br.NewConn(ctx, pwaURL)
	if err != nil {
		return errors.Wrapf(err, "failed to open URL %q", pwaURL)
	}
	defer conn.Close()

	ui := uiauto.New(tconn).WithInterval(2 * time.Second)
	installIcon := nodewith.ClassName("PwaInstallView").Role(role.Button)
	installButton := nodewith.Name("Install").Role(role.Button)

	return uiauto.Combine("Install PWA through omnibox",
		// The installability checks occur asynchronously for PWAs.
		// Wait for the Install button to appear in the Chrome omnibox before installing.
		ui.WithTimeout(timeout).WaitUntilExists(installIcon),
		ui.LeftClick(installIcon),
		ui.LeftClick(installButton))(ctx)
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

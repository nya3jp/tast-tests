// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/apputil"
	"chromiumos/tast/local/arc/apputil/notificationshowcase"
	"chromiumos/tast/local/bundles/cros/wmp/wmputils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/cws"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const resizeWindowArcAppApkFileName = "ArcNotificationTest2.apk"

// subTestType indicates the type of sub test.
type subTestType int

const (
	// browserCase indicates the browser-type case.
	browserCase subTestType = iota
	// appCase indicates the app-type case.
	appCase
	// arcCase indicates the ARC app-type case.
	arcCase
)

// resizeWindowTestParams holds parameters for ResizeWindow Tests.
type resizeWindowTestParams struct {
	// caseType indicates the type of sub test.
	caseType subTestType
	// browserType is the type of browser to be used in the test.
	browserType browser.Type
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ResizeWindow,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Resize different windows by dragging 4 corners and 4 sides",
		Contacts: []string{
			"lance.wang@cienet.com",
			"alfred.yu@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"ui.gaiaPoolDefault"}, // GAIA is required to install an app from Chrome Webstore.
		Params: []testing.Param{
			{
				Val: resizeWindowTestParams{
					caseType:    browserCase,
					browserType: browser.TypeAsh,
				},
				Timeout: 2 * time.Minute,
			},
			{
				Name: "lacros",
				Val: resizeWindowTestParams{
					caseType:    browserCase,
					browserType: browser.TypeLacros,
				},
				ExtraSoftwareDeps: []string{"lacros"},
				Timeout:           2 * time.Minute,
			},
			{
				Name: "apps",
				Val: resizeWindowTestParams{
					caseType:    appCase,
					browserType: browser.TypeAsh,
				},
				Timeout: 4*time.Minute + cws.InstallationTimeout,
			},
			{
				Name: "arc",
				Val: resizeWindowTestParams{
					caseType:    arcCase,
					browserType: browser.TypeAsh,
				},
				ExtraSoftwareDeps: []string{"arc"},
				ExtraData:         []string{resizeWindowArcAppApkFileName},
				Timeout:           2*time.Minute + apputil.InstallationTimeout,
			},
		},
	})
}

// ResizeWindow tests that resizing windows by dragging 4 corners and 4 sides.
func ResizeWindow(ctx context.Context, s *testing.State) {
	param := s.Param().(resizeWindowTestParams)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	const (
		fakeAccount  = "testuser@gmail.com"
		fakePassword = "test1234"
	)

	opts := []chrome.Option{chrome.ExtraArgs("--force-tablet-mode=clamshell")}
	switch param.caseType {
	case browserCase:
		opts = append(opts, chrome.FakeLogin(chrome.Creds{User: fakeAccount, Pass: fakePassword}))
	case appCase:
		opts = append(opts, chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")))
	case arcCase:
		opts = append(opts, chrome.ARCEnabled(), chrome.UnRestrictARCCPU(),
			chrome.FakeLogin(chrome.Creds{User: fakeAccount, Pass: fakePassword}))
	default:
		s.Fatal("Unknown case type: ", param.caseType)
	}

	cr, _, closeBrowser, err := browserfixt.SetUpWithNewChrome(ctx, param.browserType, lacrosfixt.NewConfig(), opts...)
	if err != nil {
		s.Fatal("Failed to set up chrome and browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get the connection to the test API: ", err)
	}

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	var appList []*wmputils.ResizeApp
	switch param.caseType {
	case browserCase:
		// Find correct Chrome browser app.
		chromeApp, err := apps.PrimaryBrowser(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to find Chrome or Chromium app: ", err)
		}

		var browserRoot *nodewith.Finder
		if param.browserType == browser.TypeLacros {
			classNameRegexp := regexp.MustCompile(`^ExoShellSurface(-\d+)?$`)
			browserRoot = nodewith.Role(role.Window).ClassNameRegex(classNameRegexp).NameContaining("Chrome")
		} else {
			browserRoot = nodewith.Role(role.Window).HasClass("BrowserFrame")
		}

		appList = []*wmputils.ResizeApp{
			{
				Name:         chromeApp.Name,
				ID:           chromeApp.ID,
				IsArcApp:     false,
				WindowFinder: browserRoot,
			},
		}
	case appCase:
		// Install CWS app: Text.
		cwsApp := newCwsAppText(cr, tconn)

		if err := cwsApp.install(ctx); err != nil {
			s.Fatal("Failed to install CWS Text app: ", err)
		}
		defer cwsApp.uninstall(cleanupCtx)

		appList = []*wmputils.ResizeApp{
			{
				Name:         cwsApp.App.Name,
				ID:           cwsApp.id,
				IsArcApp:     false,
				WindowFinder: cwsApp.windowFinder,
			},
			{
				Name:         apps.Files.Name,
				ID:           apps.FilesSWA.ID,
				IsArcApp:     false,
				WindowFinder: filesapp.WindowFinder(apps.FilesSWA.ID),
			},
		}
	case arcCase:
		kb, err := input.Keyboard(ctx)
		if err != nil {
			s.Fatal("Failed to get the keyboard: ", err)
		}
		defer kb.Close()

		a, err := arc.New(ctx, s.OutDir())
		if err != nil {
			s.Fatal("Failed to create an ARC instance: ", err)
		}
		defer a.Close(cleanupCtx)

		nsApp, err := notificationshowcase.NewApp(ctx, a, tconn, kb, s.DataPath(resizeWindowArcAppApkFileName))
		if err != nil {
			s.Fatal("Failed to create Notification Showcase app: ", err)
		}
		defer nsApp.Close(cleanupCtx, cr, s.HasError, s.OutDir())

		if err := nsApp.Install(ctx); err != nil {
			s.Fatal("Failed to install Notification Showcase app: ", err)
		}

		appList = []*wmputils.ResizeApp{
			// We use Notification Showcase app because:
			// 1. this installed from APK file, which will not require authenticated login.
			// 2. it will skip Play Store installation process, which will not be affected by Play Store factors.
			// 3. this app is a small-size app which will be easier to resize without creating afterimages on low-end DUTs.
			{
				Name:         notificationshowcase.AppName,
				ID:           notificationshowcase.AppID,
				IsArcApp:     true,
				WindowFinder: nodewith.HasClass("RootView").NameContaining(notificationshowcase.AppName),
			},
		}
	default:
		s.Fatal("Unknown case type: ", param.caseType)
	}

	screenRecorder, err := uiauto.NewScreenRecorder(ctx, tconn)
	if err != nil {
		s.Log("Failed to create ScreenRecorder: ", err)
	}

	if err := screenRecorder.Start(ctx, tconn); err != nil {
		s.Log("Failed to start ScreenRecorder: ", err)
	}
	defer uiauto.ScreenRecorderStopSaveRelease(cleanupCtx, screenRecorder, filepath.Join(s.OutDir(), "record.webm"))

	for _, app := range appList {
		f := func(ctx context.Context, s *testing.State) {
			if err := resizeSubTest(ctx, cr, tconn, app, s.OutDir()); err != nil {
				s.Fatal("Failed to resize window: ", err)
			}
		}

		if !s.Run(ctx, fmt.Sprintf("resize on app %s", app.Name), f) {
			s.Error("Failed to test resize functionality on app ", app.Name)
		}
	}
}

func resizeSubTest(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, resizeApp *wmputils.ResizeApp, ourDir string) (retErr error) {
	cleanupSubTestCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Wait until the app is ready to use.
	if err := waitUntilAppReady(ctx, tconn, resizeApp); err != nil {
		return errors.Wrapf(err, "failed to wait for %s is installed and ready to use", resizeApp.Name)
	}

	if err := launcher.LaunchApp(tconn, resizeApp.Name)(ctx); err != nil {
		return errors.Wrapf(err, "failed to launch app %q", resizeApp.Name)
	}
	defer closeApp(cleanupSubTestCtx, tconn, resizeApp)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupSubTestCtx, ourDir, func() bool { return retErr != nil }, cr, resizeApp.Name)

	// Ensure there is only one window remain on the screen.
	testing.ContextLogf(ctx, "Waiting for the window of App %q to be shown and stabilized", resizeApp.Name)
	if err := waitUntilWindowStable(ctx, tconn, resizeApp); err != nil {
		return errors.Wrap(err, "failed to wait for window to be shown")
	}

	// Minimize the window in advance to avoid its border being off-screen after the window has been dragged to the center of screen.
	if err := resizeApp.ResizeWindowToMin(tconn, resizeApp.WindowFinder)(ctx); err != nil {
		return errors.Wrap(err, "failed to minimize window")
	}
	testing.ContextLog(ctx, "Window has been resized to minimize")

	// Move the window to the center of the screen to observe the test.
	if err := resizeApp.MoveWindowToCenter(tconn, resizeApp.WindowFinder, resizeApp.IsArcApp)(ctx); err != nil {
		return errors.Wrap(err, "failed to move window to center")
	}
	testing.ContextLog(ctx, "Window has been moved to the center of screen")

	// Perform resize operations.
	for _, bound := range wmputils.AllBounds() {
		if err := resizeApp.ResizeWindow(tconn, resizeApp.WindowFinder, bound)(ctx); err != nil {
			return errors.Wrapf(err, "failed to resize window when resizing bound %q", bound)
		}
	}

	return nil
}

// waitUntilAppReady waits until the app is installed and ready to use.
func waitUntilAppReady(ctx context.Context, tconn *chrome.TestConn, resizeApp *wmputils.ResizeApp) error {
	// PlayStore app might not appear instantly after switch back from tablet mode.
	// Hence, retry for few times to ensure the app is ready.
	return testing.Poll(ctx, func(ctx context.Context) error {
		installedApps, err := ash.ChromeApps(ctx, tconn)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get installed apps"))
		}

		for _, app := range installedApps {
			// If given app ID is empty (e.g., Play Store), we use app name to verify if app is installed and ready to use.
			if resizeApp.ID != "" {
				if app.AppID == resizeApp.ID {
					return nil
				}
			} else {
				if app.Name == resizeApp.Name {
					return nil
				}
			}
		}

		return errors.Errorf("app %q is not installed", resizeApp.Name)
	}, &testing.PollOptions{Timeout: 30 * time.Second})
}

// waitUntilWindowStable ensures there is only one shown window.
// When running ARC subcase, Google Play Store will display two windows, which will cause the test to fail.
// We will wait until one window is disappeared automatically.
func waitUntilWindowStable(ctx context.Context, tconn *chrome.TestConn, resizeApp *wmputils.ResizeApp) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		// Ensure there are only one visible window.
		// Play Store app will pop up a delegate window for a few seconds, so we need to wait until it disappears automatically.
		// Otherwise, it will reduce or impact UI performance(such as freezing window elements) on low-end DUTs.
		if windows, err := ash.FindAllWindows(ctx, tconn, func(w *ash.Window) bool {
			return w.IsVisible || w.IsFrameVisible
		}); err != nil {
			return errors.Wrap(err, "failed to get active window")
		} else if len(windows) != 1 {
			return errors.Errorf("expect 1 window but got: %d", len(windows))
		}

		// Find the window that has the correct name.
		// One exception is Play Store (see comment on below).
		window, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
			switch resizeApp.ID {
			case "":
				// For Play Store, we use exact string "Google Play Store" to find the window.
				return w.Title == "Google Play Store"
			case apps.LacrosID:
				// For browser, we use exact string "New Tab - Google Chrome" to find the window.
				return strings.Contains(w.Title, "New Tab - Google Chrome")
			default:
				// For regular apps(i.e., apps with valid ID), we use app name to find the correct window.
				return strings.Contains(w.Title, resizeApp.Name)
			}
		})
		if err != nil {
			return errors.Wrapf(err, "failed to find window with app name %q", resizeApp.Name)
		}

		if err := ash.WaitWindowFinishAnimating(ctx, tconn, window.ID); err != nil {
			return errors.Wrap(err, "failed to wait window finish animating")
		}

		return ash.SetWindowStateAndWait(ctx, tconn, window.ID, ash.WindowStateNormal)
	}, &testing.PollOptions{Timeout: time.Minute})
}

type cwsAppText struct {
	cws.App
	id           string
	windowFinder *nodewith.Finder

	cr    *chrome.Chrome
	tconn *chrome.TestConn
}

func newCwsAppText(cr *chrome.Chrome, tconn *chrome.TestConn) *cwsAppText {
	return &cwsAppText{
		App: cws.App{
			Name: "Text",
			URL:  "https://chrome.google.com/webstore/detail/text/mmfbcljfglbokpmkimbfghdkjmjhdgbg",
		},
		id:           "mmfbcljfglbokpmkimbfghdkjmjhdgbg",
		windowFinder: nodewith.HasClass("RootView").Name("Text"),
		cr:           cr,
		tconn:        tconn,
	}
}

func (app *cwsAppText) install(ctx context.Context) error {
	isInstalled, err := ash.ChromeAppInstalled(ctx, app.tconn, app.id)
	if err != nil {
		return errors.Wrap(err, "failed to check CWS app's existence")
	}

	if isInstalled {
		return nil
	}
	return cws.InstallApp(ctx, app.cr, app.tconn, app.App)
}

func (app *cwsAppText) uninstall(ctx context.Context) error {
	defer func() {
		settings := ossettings.New(app.tconn)
		settings.Close(ctx)
	}()
	return ossettings.UninstallApp(ctx, app.tconn, app.cr, app.Name, app.id)
}

// closeApp closes the app id ID is provided; otherwise, will find the app ID first.
func closeApp(ctx context.Context, tconn *chrome.TestConn, app *wmputils.ResizeApp) error {
	// The app "Play Store" isn't unified depending on different models.
	// Thus, the ID isn't specified in this test.
	// Instead, we find the ID from running apps on the shelf.
	if app.ID == "" {
		// Finds the currently running app ID with specified name on the shelf.
		shelfApps, err := ash.ShelfItems(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to obtain all shelf apps")
		}
		for _, shelfapp := range shelfApps {
			if shelfapp.Status == ash.ShelfItemRunning {
				return apps.Close(ctx, tconn, shelfapp.AppID)
			}
		}
		return errors.Errorf("failed to find app ID of app %s", app.Name)
	}

	return apps.Close(ctx, tconn, app.ID)
}

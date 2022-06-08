// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/wmp/wmputils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/cws"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ResizeWindow,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Resize different windows by dragging 4 corners and 4 sides",
		Contacts: []string{
			"lance.wang@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"ui.gaiaPoolDefault"}, // GAIA is required to install an app from Chrome Webstore.
		Params: []testing.Param{
			{
				Val:     false, // Expect not to test on ARC++ app.
				Timeout: 5*time.Minute + cws.InstallationTimeout,
			}, {
				Name:              "arc",
				Val:               true, // Expect to test on ARC++ app.
				ExtraSoftwareDeps: []string{"arc"},
				Timeout:           5 * time.Minute,
			}},
	})
}

// ResizeWindow tests that resizing windows by dragging 4 corners and 4 sides.
func ResizeWindow(ctx context.Context, s *testing.State) {
	isArc := s.Param().(bool)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	var opts []chrome.Option
	if isArc {
		opts = append(opts, chrome.ARCEnabled(), chrome.UnRestrictARCCPU(),
			chrome.FakeLogin(chrome.Creds{User: "testuser@gmail.com", Pass: "test1234"}))
	} else {
		opts = append(opts, chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")))
	}

	// Avoid share session with other tests to ensure the window size is in initial state.
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to Chrome login: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get the connection to the test API: ", err)
	}

	var appList []*wmputils.ResizeApp
	if !isArc {
		// Install CWS app: Text.
		cwsApp := newCwsAppText(cr, tconn)

		if err := cwsApp.install(ctx); err != nil {
			s.Fatal("Failed to install CWS Text app: ", err)
		}
		defer cwsApp.uninstall(cleanupCtx)

		// Find correct Chrome browser app.
		chromeApp, err := apps.ChromeOrChromium(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to find Chrome or Chromium app: ", err)
		}

		appList = []*wmputils.ResizeApp{
			{
				Name:         cwsApp.App.Name,
				ID:           cwsApp.id,
				IsArcApp:     false,
				WindowFinder: cwsApp.windowFinder,
			}, {
				Name:         chromeApp.Name,
				ID:           chromeApp.ID,
				IsArcApp:     false,
				WindowFinder: nodewith.HasClass("NonClientView").NameContaining(chromeApp.Name),
			}, {
				Name:         apps.Files.Name,
				ID:           apps.Files.ID,
				IsArcApp:     false,
				WindowFinder: filesapp.WindowFinder(apps.Files.ID),
			},
		}
	} else {
		appList = []*wmputils.ResizeApp{
			// Choose PlayStore as an ARC++ app so that it won't have to install any extra ARC++ app.
			{
				Name: apps.PlayStore.Name,
				// The app "Play Store" isn't unified depending on different models.
				// e.g.: the app is apps.PlayStore.ID on Volteer,
				// "acemnolionkahnbnbangggohjkggjfpl" on Hayato,
				// "cnbgggchhmkkdmeppjobngjoejnihlei" on Berknip.
				// Thus, the ID isn't specified in this test.
				ID:           "",
				IsArcApp:     true,
				WindowFinder: nodewith.HasClass("RootView").NameContaining("Google Play Store"),
			},
		}
	}

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

	if err := launcher.LaunchApp(tconn, resizeApp.Name)(ctx); err != nil {
		return errors.Wrapf(err, "failed to launch %s", resizeApp.Name)
	}
	defer closeApp(cleanupSubTestCtx, tconn, resizeApp)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupSubTestCtx, ourDir, func() bool { return retErr != nil }, cr, resizeApp.Name)

	// Ensure only one window is shown.
	// When running ARC subcase, Google Play Store will display two windows, which will cause the test to fail.
	// We will wait until one window is disappeared automatically.
	var window *ash.Window
	if err := testing.Poll(ctx, func(ctx context.Context) (err error) {
		if window, err = ash.FindOnlyWindow(ctx, tconn, func(w *ash.Window) bool { return w.IsActive }); err != nil {
			return errors.Wrap(err, "failed to get active window")
		}

		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for window to be shown")
	}

	if err := ash.SetWindowStateAndWait(ctx, tconn, window.ID, ash.WindowStateNormal); err != nil {
		return errors.Wrap(err, "failed to set window state to normal")
	}

	if err := uiauto.New(tconn).WaitForLocation(resizeApp.WindowFinder)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for window to stabilize")
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
			return errors.Wrap(err, "failed to resize window")
		}
	}

	return nil
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

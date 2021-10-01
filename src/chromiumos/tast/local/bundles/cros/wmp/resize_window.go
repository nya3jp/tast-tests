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
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/wmp/wmputils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
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
		Func: ResizeWindow,
		Desc: "Resize different windows by dragging 4 corners and 4 sides",
		Contacts: []string{
			"lance.wang@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"ui.gaiaPoolDefault"},
		LacrosStatus: testing.LacrosVariantUnneeded,
		Params: []testing.Param{
			{
				Val:     false, // Expect not to test on ARC++ app.
				Timeout: 5*time.Minute + cws.InstallationTimeout,
			}, {
				Name:              "arc",
				Val:               true, // Expect to test on ARC++ app.
				ExtraSoftwareDeps: []string{"arc"},
				Timeout:           5*time.Minute + arcAppInstallationTimeout,
			}},
	})
}

const (
	arcAppInstallationTimeout = 5 * time.Minute

	arcAppName    = "YT Music"
	arcAppID      = "hpdkdmlckojaocbedhffglopeafcgggc"
	arcAppPkgName = "com.google.android.apps.youtube.music"
)

// ResizeWindow tests that resizing windows by dragging 4 corners and 4 sides.
func ResizeWindow(ctx context.Context, s *testing.State) {
	isArc := s.Param().(bool)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	var opts []chrome.Option
	if isArc {
		opts = append(opts, chrome.ARCSupported(), chrome.ExtraArgs(arc.DisableSyncFlags()...), chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")))
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

	if isArc {
		if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
			s.Fatal("Failed to opt into Play Store: ", err)
		}
	}

	// Initialize new resize utility.
	testResizeUtil, err := wmputils.NewResizeUtil(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to prepare for resize window: ", err)
	}

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

	var appList = []*wmputils.ResizeApp{
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

	if isArc {
		a, err := arc.New(ctx, s.OutDir())
		if err != nil {
			s.Fatal("Failed to start ARC: ", err)
		}
		defer a.Close(cleanupCtx)

		d, err := a.NewUIDevice(ctx)
		if err != nil {
			s.Fatal("Failed initializing UI Automater: ", err)
		}
		defer d.Close(cleanupCtx)

		if err := playstore.InstallApp(ctx, a, d, arcAppPkgName, -1); err != nil {
			s.Fatal("Failed to install YT Music: ", err)
		}
		defer a.Uninstall(cleanupCtx, arcAppPkgName)

		if err := optin.ClosePlayStore(ctx, tconn); err != nil {
			s.Fatal("Failed to close Play Store: ", err)
		}

		appList = []*wmputils.ResizeApp{
			{
				Name:         arcAppName,
				ID:           arcAppID,
				IsArcApp:     true,
				WindowFinder: nodewith.HasClass("RootView").Name("YT Music"),
			},
		}
	}

	for _, app := range appList {
		f := func(ctx context.Context, s *testing.State) {
			if err := resizeSubTest(ctx, cr, tconn, testResizeUtil, app, s.OutDir()); err != nil {
				s.Fatal("Failed to resize window: ", err)
			}
		}

		if !s.Run(ctx, fmt.Sprintf("resize on app %s", app.Name), f) {
			s.Error("Failed to test resize functionality on app ", app.Name)
		}
	}
}

func resizeSubTest(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, testResizeUtil *wmputils.ResizeWindowUtil, resizeApp *wmputils.ResizeApp, ourDir string) (retErr error) {
	cleanupSubTestCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	if err := launcher.LaunchAndWaitForAppOpen(tconn, apps.App{ID: resizeApp.ID, Name: resizeApp.Name})(ctx); err != nil {
		return err
	}
	defer apps.Close(cleanupSubTestCtx, tconn, resizeApp.ID)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupSubTestCtx, ourDir, func() bool { return retErr != nil }, cr, resizeApp.Name)

	if err := resizeApp.TurnOffWindowPreset(ctx, cr, tconn); err != nil {
		return errors.Wrap(err, "failed to set window mode to maximized")
	}

	// Minimize the window in advance to avoid its border being off-screen after the window has been dragged to the center of screen.
	if err := testResizeUtil.ResizeWindowToMin(resizeApp.WindowFinder)(ctx); err != nil {
		return errors.Wrap(err, "failed to minimize window")
	}
	testing.ContextLog(ctx, "Window has been resized to minimize")

	// Move the window to the center of the screen to observe the test.
	if err := testResizeUtil.MoveWindowToCenter(resizeApp)(ctx); err != nil {
		return errors.Wrap(err, "failed to move window to center")
	}
	testing.ContextLog(ctx, "Window has been moved to the center of screen")

	// Perform resize operations.
	for _, bound := range wmputils.GetAllBounds() {
		if err := testResizeUtil.ResizeWindow(resizeApp.WindowFinder, bound)(ctx); err != nil {
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

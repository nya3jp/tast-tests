// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/cws"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SearchInstalledApps,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Install apps from CWS and verify that it appears in the launcher",
		Contacts: []string{
			"kyle.chen@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3*time.Minute + cws.InstallationTimeout,
		Params: []testing.Param{{
			Name:    "productivity_launcher_clamshell_mode",
			Val:     launcher.TestCase{ProductivityLauncher: true, TabletMode: false},
			Fixture: "chromeLoggedInWithGaiaProductivityLauncher",
		}, {
			Name:    "clamshell_mode",
			Val:     launcher.TestCase{ProductivityLauncher: false, TabletMode: false},
			Fixture: "chromeLoggedInWithGaiaLegacyLauncher",
		}, {
			Name:              "productivity_launcher_tablet_mode",
			Val:               launcher.TestCase{ProductivityLauncher: true, TabletMode: true},
			Fixture:           "chromeLoggedInWithGaiaProductivityLauncher",
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}, {
			Name:              "tablet_mode",
			Val:               launcher.TestCase{ProductivityLauncher: false, TabletMode: true},
			Fixture:           "chromeLoggedInWithGaiaLegacyLauncher",
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}},
	})
}

// SearchInstalledApps tests that apps installed from CWS appear in launcher.
func SearchInstalledApps(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	ui := uiauto.New(tconn)

	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kw.Close()

	testCase := s.Param().(launcher.TestCase)
	tabletMode := testCase.TabletMode
	if !tabletMode {
		defer func(ctx context.Context) {
			if err := ui.RetryUntil(
				kw.AccelAction("esc"),
				func(ctx context.Context) error { return ash.WaitForLauncherState(ctx, tconn, ash.Closed) },
			)(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to dismiss launcher: ", err)
			}
		}(cleanupCtx)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to ensure clamshell/tablet mode: ", err)
	}
	defer cleanup(cleanupCtx)

	// When a DUT switches from tablet mode to clamshell mode, sometimes it takes a while to settle down.
	if !tabletMode {
		if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
			s.Fatal("Launcher not closed after transition to clamshell mode: ", err)
		}
	}

	cwsapp := newCwsAppGoogleDrawings(cr, tconn)

	if err := cwsapp.install(ctx); err != nil {
		s.Fatal("Failed to install an app from Chrome Web Store: ", err)
	}
	defer cwsapp.uninstall(cleanupCtx)

	if err := uiauto.Combine("open Launcher and verify the app appears in search result",
		launcher.Open(tconn),
		launcher.Search(tconn, kw, cwsapp.name),
		func(ctx context.Context) error {
			return ui.WaitUntilExists(launcher.CreateAppSearchFinder(ctx, tconn, cwsapp.name))(ctx)
		},
	)(ctx); err != nil {
		s.Fatal("Failed to verify that the app is in Launcher: ", err)
	}

	defer func(ctx context.Context) {
		faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)
		faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	}(cleanupCtx)

}

// cwsAppGoogleDrawings defines the cws-app `Google Drawings`.
type cwsAppGoogleDrawings struct {
	cr    *chrome.Chrome
	tconn *chrome.TestConn

	id     string
	name   string
	cwsURL string

	app *cws.App
}

// newCwsAppGoogleDrawings returns the instance of cwsAppGoogleDrawings.
func newCwsAppGoogleDrawings(cr *chrome.Chrome, tconn *chrome.TestConn) *cwsAppGoogleDrawings {
	const (
		id   = "mkaakpdehdafacodkgkpghoibnmamcme"
		name = "Google Drawings"
		url  = "https://chrome.google.com/webstore/detail/google-drawings/mkaakpdehdafacodkgkpghoibnmamcme"
	)

	return &cwsAppGoogleDrawings{
		cr:     cr,
		tconn:  tconn,
		id:     id,
		name:   name,
		cwsURL: url,
		app:    &cws.App{Name: name, URL: url},
	}
}

// install installs the cws-app via Chrome web store.
func (c *cwsAppGoogleDrawings) install(ctx context.Context) error {
	isInstalled, err := ash.ChromeAppInstalled(ctx, c.tconn, c.id)
	if err != nil {
		return errors.Wrap(err, "failed to check CWS app's existance")
	}

	if isInstalled {
		return nil
	}

	testing.ContextLogf(ctx, "Install CWS app: %q", c.name)
	return cws.InstallApp(ctx, c.cr, c.tconn, *c.app)
}

// uninstall uninstalls the cws-app via ossettings.
func (c *cwsAppGoogleDrawings) uninstall(ctx context.Context) error {
	isInstalled, err := ash.ChromeAppInstalled(ctx, c.tconn, c.id)
	if err != nil {
		return errors.Wrap(err, "failed to check CWS app's existance")
	}

	if !isInstalled {
		return nil
	}

	defer func() {
		settings := ossettings.New(c.tconn)
		settings.Close(ctx)
	}()
	testing.ContextLogf(ctx, "Uninstall CWS app: %q", c.name)
	return ossettings.UninstallApp(ctx, c.tconn, c.cr, c.name, c.id)
}

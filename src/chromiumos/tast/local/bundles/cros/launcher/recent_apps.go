// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/cws"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type initParams struct {
	TabletMode  bool
	BootWithArc bool // if the session was booted with arc; must match the fixture.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         RecentApps,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify that a local file shows to Continue Section",
		Contacts: []string{
			"anasalazar@chromium.org",
			"chromeos-sw-engprod@google.com",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_cws",
			Val: initParams{
				TabletMode:  false,
				BootWithArc: false,
			},
			Fixture: "chromeLoggedInWithGaiaProductivityLauncher",
			Timeout: 3*time.Minute + cws.InstallationTimeout,
		},
			{
				Name: "clamshell_androidp",
				Val: initParams{
					TabletMode:  false,
					BootWithArc: true,
				},
				Fixture:           "arcBootedWithProductivityLauncher",
				ExtraSoftwareDeps: []string{"android_p"},
				Timeout:           chrome.GAIALoginTimeout + arc.BootTimeout + 120*time.Second,
			},
			{
				Name: "clamshell_androidvm",
				Val: initParams{
					TabletMode:  false,
					BootWithArc: true,
				},
				Fixture:           "arcBootedWithProductivityLauncher",
				ExtraSoftwareDeps: []string{"android_vm"},
				Timeout:           chrome.GAIALoginTimeout + arc.BootTimeout + 120*time.Second,
			},
			{
				Name: "tablet_cws",
				Val: initParams{
					TabletMode:  true,
					BootWithArc: false,
				},
				Fixture:           "chromeLoggedInWithGaiaProductivityLauncher",
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				Timeout:           3*time.Minute + cws.InstallationTimeout,
			},
			{
				Name: "tablet_androidp",
				Val: initParams{
					TabletMode:  true,
					BootWithArc: true,
				},
				Fixture:           "arcBootedWithProductivityLauncher",
				ExtraSoftwareDeps: []string{"android_p"},
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				Timeout:           chrome.GAIALoginTimeout + arc.BootTimeout + 120*time.Second,
			},
			{
				Name: "tablet_androidvm",
				Val: initParams{
					TabletMode:  true,
					BootWithArc: true,
				},
				Fixture:           "arcBootedWithProductivityLauncher",
				ExtraSoftwareDeps: []string{"android_vm"},
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				Timeout:           chrome.GAIALoginTimeout + arc.BootTimeout + 120*time.Second,
			}},
	})
}

func RecentApps(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	testCase := s.Param().(initParams)
	tabletMode := testCase.TabletMode
	arcBoot := testCase.BootWithArc

	var cr *chrome.Chrome

	if arcBoot {
		cr = s.FixtValue().(*arc.PreData).Chrome
	} else {
		cr = s.FixtValue().(*chrome.Chrome)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	ui := uiauto.New(tconn)

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to ensure clamshell/tablet mode: ", err)
	}
	defer cleanup(cleanupCtx)

	if !tabletMode {
		if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
			s.Fatal("Launcher not closed after transition to clamshell mode: ", err)
		}
	}

	if err := launcher.OpenProductivityLauncher(ctx, tconn, tabletMode); err != nil {
		s.Fatal("Failed to open launcher: ", err)
	}

	// Recent apps always show with 5 default suggestions.
	recentApps := nodewith.ClassName("RecentAppsView")
	if err := ui.WithTimeout(3 * time.Second).WaitUntilExists(recentApps)(ctx); err != nil {
		s.Fatal("Failed to show recent apps section: ", err)
	}

	if tabletMode {
		if err := launcher.HideTabletModeLauncher(tconn)(ctx); err != nil {
			s.Fatal("Failed to hide the launcher in tablet: ", err)
		}
	} else if err := launcher.CloseBubbleLauncher(tconn)(ctx); err != nil {
		s.Fatal("Failed to close the bubble launcher: ", err)
	}

	var appName string
	var appID string

	if arcBoot {
		// Install a mock Android app.
		const apk = "ArcInstallAppWithAppListSortedTest.apk"
		a := s.FixtValue().(*arc.PreData).ARC
		if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
			s.Fatal("Failed installing app: ", err)
		}

		appName = "InstallAppWithAppListSortedMockApp"
		appID, err = ash.WaitForChromeAppByNameInstalled(ctx, tconn, appName, 1*time.Minute)
		if err != nil {
			s.Fatalf("Failed to wait until %s(%s) is installed: %v", appName, appID, err)
		}
	} else {
		cwsapp := newCwsApp(cr, tconn)

		if err := cwsapp.install(ctx); err != nil {
			s.Fatal("Failed to install an app from Chrome Web Store: ", err)
		}
		defer cwsapp.uninstall(cleanupCtx)
		appName = cwsapp.name
	}

	if err := launcher.OpenProductivityLauncher(ctx, tconn, tabletMode); err != nil {
		s.Fatal("Failed to open launcher: ", err)
	}

	if err := ui.Exists(recentApps)(ctx); err != nil {
		s.Fatal("Failed to show recent apps section: ", err)
	}

	newApp := nodewith.NameContaining(appName).Ancestor(recentApps).First()
	if err := ui.WaitUntilExists(newApp)(ctx); err != nil {
		s.Fatalf("Failed to show %s(%s) in recent apps: %v", appName, appID, err)
	}

	if err := openContextMenuAndClickUninstall(ctx, tconn, newApp); err != nil {
		s.Fatalf("Failed to uninstall %s(%s): %v", appName, appID, err)
	}

	// When uninstalled, the app should disappear from recent apps.
	if err := ui.WaitUntilGone(newApp)(ctx); err != nil {
		s.Fatalf("Failed to verify that %s(%s) is removed from recent apps: %v", appName, appID, err)
	}
}

type cwsApp struct {
	cr    *chrome.Chrome
	tconn *chrome.TestConn

	id     string
	name   string
	cwsURL string

	app *cws.App
}

// newCwsApp returns the instance of cwsApp.
func newCwsApp(cr *chrome.Chrome, tconn *chrome.TestConn) *cwsApp {
	const (
		id   = "mkaakpdehdafacodkgkpghoibnmamcme"
		name = "Google Drawings"
		url  = "https://chrome.google.com/webstore/detail/google-drawings/mkaakpdehdafacodkgkpghoibnmamcme"
	)

	return &cwsApp{
		cr:     cr,
		tconn:  tconn,
		id:     id,
		name:   name,
		cwsURL: url,
		app:    &cws.App{Name: name, URL: url},
	}
}

// install installs the cws-app via Chrome web store.
func (c *cwsApp) install(ctx context.Context) error {
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
func (c *cwsApp) uninstall(ctx context.Context) error {
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

func openContextMenuAndClickUninstall(ctx context.Context, tconn *chrome.TestConn, newApp *nodewith.Finder) error {
	ui := uiauto.New(tconn)
	confirmUninstall := nodewith.Name("Uninstall").Role(role.Button)
	if err := uiauto.Combine("Uninstall app",
		ui.Exists(newApp),
		ui.RightClick(newApp),
		ui.LeftClick(nodewith.Name("Uninstall").ClassName("MenuItemView")),
		ui.WaitUntilExists(confirmUninstall),
		ui.LeftClick(confirmUninstall),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to remove the app on recent apps")
	}
	return nil
}

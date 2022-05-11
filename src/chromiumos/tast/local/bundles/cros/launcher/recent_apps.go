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
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
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
		Desc:         "Verify that different types of apps show in the recent apps section",
		Contacts: []string{
			"anasalazar@chromium.org",
			"chromeos-sw-engprod@google.com",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name:              "androidp_clamshell",
				Val:               initParams{TabletMode: false, BootWithArc: true},
				Fixture:           "arcBootedWithProductivityLauncher",
				ExtraSoftwareDeps: []string{"android_p"},
				Timeout:           chrome.GAIALoginTimeout + arc.BootTimeout + 120*time.Second,
			},
			{
				Name:              "androidp_tablet",
				Val:               initParams{TabletMode: true, BootWithArc: true},
				Fixture:           "arcBootedWithProductivityLauncher",
				ExtraSoftwareDeps: []string{"android_p"},
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				Timeout:           chrome.GAIALoginTimeout + arc.BootTimeout + 120*time.Second,
			},
			{
				Name:              "androidvm_clamshell",
				Val:               initParams{TabletMode: false, BootWithArc: true},
				Fixture:           "arcBootedWithProductivityLauncher",
				ExtraSoftwareDeps: []string{"android_vm"},
				Timeout:           chrome.GAIALoginTimeout + arc.BootTimeout + 120*time.Second,
			},
			{
				Name:              "androidvm_tablet",
				Val:               initParams{TabletMode: true, BootWithArc: true},
				Fixture:           "arcBootedWithProductivityLauncher",
				ExtraSoftwareDeps: []string{"android_vm"},
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				Timeout:           chrome.GAIALoginTimeout + arc.BootTimeout + 120*time.Second,
			},
			{
				Name:    "cws_clamshell",
				Val:     initParams{TabletMode: false, BootWithArc: false},
				Fixture: "chromeLoggedInWithGaiaProductivityLauncher",
				Timeout: 3*time.Minute + cws.InstallationTimeout,
			},
			{
				Name:              "cws_tablet",
				Val:               initParams{TabletMode: true, BootWithArc: false},
				Fixture:           "chromeLoggedInWithGaiaProductivityLauncher",
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				Timeout:           3*time.Minute + cws.InstallationTimeout,
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

	// Recent apps always show the first time with default suggestions.
	recentApps := nodewith.ClassName("RecentAppsView")
	if err := ui.WithTimeout(3 * time.Second).WaitUntilExists(recentApps)(ctx); err != nil {
		s.Fatal("Failed to show recent apps section: ", err)
	}

	// Hide the launcher and install an app to trigger an update for Recent Apps on the next open.
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
		// Install an app from the Chrome webstore.
		cwsapp := newCwsApp(cr, tconn)

		if err := cwsapp.install(ctx); err != nil {
			s.Fatal("Failed to install an app from Chrome Web Store: ", err)
		}
		appName = cwsapp.name
		appID = cwsapp.id
	}
	defer uninstallAppViaSettings(ctx, cr, tconn, appName, appID)

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

	expectedOrderedAppNames := []string{appName}
	if err := verifyLeadingRecentApps(ctx, tconn, expectedOrderedAppNames); err != nil {
		s.Fatal("Failed to confirm recent apps order: ", err)
	}

	// Launch the files app. It should now be the first app in the recent apps section.
	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Could not launch the Files App: ", err)
	}
	defer filesApp.Close(cleanupCtx)

	if err := launcher.OpenProductivityLauncher(ctx, tconn, tabletMode); err != nil {
		s.Fatal("Failed to open launcher: ", err)
	}

	expectedOrderedAppNames = []string{"Files", appName}
	if err := verifyLeadingRecentApps(ctx, tconn, expectedOrderedAppNames); err != nil {
		s.Fatal("Failed to confirm recent apps order: ", err)
	}

	// Launch the new app. It should be the first app in the recent apps section again.
	if err := ui.DoubleClick(newApp)(ctx); err != nil {
		s.Fatalf("Failed to click %s in recent apps: %v", appName, err)
	}

	if err := ash.WaitForApp(ctx, tconn, appID, 10*time.Second); err != nil {
		s.Fatalf("App %s never opened: %v", appName, err)
	}

	if err := launcher.OpenProductivityLauncher(ctx, tconn, tabletMode); err != nil {
		s.Fatal("Failed to open launcher: ", err)
	}

	expectedOrderedAppNames = []string{appName, "Files"}
	if err := verifyLeadingRecentApps(ctx, tconn, expectedOrderedAppNames); err != nil {
		s.Fatal("Failed to confirm recent apps order: ", err)
	}

	// When uninstalled, the app should disappear from recent apps.
	if err := launcher.UninstallsAppUsingContextMenu(ctx, tconn, newApp); err != nil {
		s.Fatalf("Failed to uninstall %s(%s): %v", appName, appID, err)
	}

	if err := ui.WaitUntilGone(newApp)(ctx); err != nil {
		s.Fatalf("Failed to verify that %s(%s) is removed from recent apps: %v", appName, appID, err)
	}

	expectedOrderedAppNames = []string{"Files"}
	if err := verifyLeadingRecentApps(ctx, tconn, expectedOrderedAppNames); err != nil {
		s.Fatal("Failed to confirm recent apps order: ", err)
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
		id   = "mojbfofholnnfgdkcgabcfkjelblkgmh"
		name = "Google Wallpaper Art"
		url  = "https://chrome.google.com/webstore/detail/google-wallpaper-art/mojbfofholnnfgdkcgabcfkjelblkgmh"
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
		return errors.Wrap(err, "failed to check app's existance")
	}

	if isInstalled {
		return nil
	}

	testing.ContextLogf(ctx, "Install app: %q", c.name)
	return cws.InstallApp(ctx, c.cr, c.tconn, *c.app)
}

// uninstallAppViaSettings uninstalls the an app via ossettings.
func uninstallAppViaSettings(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, name, id string) error {
	isInstalled, err := ash.ChromeAppInstalled(ctx, tconn, id)
	if err != nil {
		return errors.Wrap(err, "failed to check app's existance")
	}

	if !isInstalled {
		return nil
	}

	defer func() {
		settings := ossettings.New(tconn)
		settings.Close(ctx)
	}()
	testing.ContextLogf(ctx, "Uninstall app: %q", name)
	return ossettings.UninstallApp(ctx, tconn, cr, name, id)
}

// verifyLeadingRecentApps confirms that the leading apps in the recent app section appear in the same order as orderedAppsNames
func verifyLeadingRecentApps(ctx context.Context, tconn *chrome.TestConn, orderedAppsNames []string) error {
	ui := uiauto.New(tconn)
	recentApps := nodewith.ClassName("RecentAppsView")
	if err := ui.Exists(recentApps)(ctx); err != nil {
		return errors.Wrap(err, "failed to show recent apps")
	}

	for i, appName := range orderedAppsNames {
		app := nodewith.ClassName(launcher.ExpandedItemsClass).Ancestor(recentApps).Nth(i)
		appInfo, err := ui.Info(ctx, app)
		if err != nil {
			return errors.Wrapf(err, "failed to search the #%d app in recent apps", i)
		}
		if appInfo.Name != appName {
			return errors.Wrapf(err, "#%d app(%s) is not expected(%s)", i, appInfo.Name, appName)
		}
	}
	return nil
}

// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shelf

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

type newInstanceType string

const (
	tab    newInstanceType = "tab"
	window newInstanceType = "window"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AppWindowOnShelf,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks the window instances when opening a chrome app in a new window/tab",
		Contacts: []string{
			"wcwang@chromium.org",
			"tbarzic@chromium.org",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:    "productivity_launcher_clamshell_mode",
			Val:     launcher.TestCase{TabletMode: false},
			Fixture: "chromeLoggedInWith100FakeAppsProductivityLauncher",
		}, {
			Name:    "productivity_launcher_tablet_mode",
			Val:     launcher.TestCase{TabletMode: true},
			Fixture: "chromeLoggedInWith100FakeAppsProductivityLauncher",
		}},
	})
}

// AppWindowOnShelf verifies that the number of windows and instances on the shelf matches how the apps are opened.
func AppWindowOnShelf(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tabletMode := s.Param().(launcher.TestCase).TabletMode
	originallyEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to check if DUT is in tablet mode: ", err)
	}
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to ensure clamshell/tablet mode: ", err)
	}
	defer cleanup(ctx)

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)
	defer faillog.SaveScreenshotOnError(cleanupCtx, cr, s.OutDir(), s.HasError)

	// Ensure that the tablet launcher is closed before opening a launcher instance for test in clamshell.
	if originallyEnabled && !tabletMode {
		if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
			s.Fatal("Launcher not closed after transition to clamshell mode: ", err)
		}
	}

	if err := openLauncher(ctx, tconn, tabletMode); err != nil {
		s.Fatal("Failed to open the launcher")
	}

	appsGridView := nodewith.ClassName("ScrollableAppsGridView")
	if tabletMode {
		appsGridView = nodewith.ClassName("AppsGridView")
	}

	if err := launcher.WaitForStableNumberOfApps(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for item count in app list to stabilize: ", err)
	}

	fakeApps := nodewith.NameContaining("fake app").Ancestor(appsGridView)
	ui := uiauto.New(tconn)

	app := fakeApps.Nth(0)

	// Open the `app` as a tab
	if err := openAppAs(ctx, tconn, app, tab); err != nil {
		s.Fatal("Failed to open app in a new tab for the first time: ", err)
	}

	// Check if a chrome instance is created for the tab.
	activeWindow, err := ash.GetActiveWindow(ctx, tconn)
	if err != nil {
		s.Fatal("Error getting the active window")
	}

	chromeBrowser, err := apps.ChromeOrChromium(ctx, tconn)
	if err != nil {
		s.Fatal("Could not determine the correct Chrome app to use: ", err)
	}
	if !strings.Contains(activeWindow.Title, chromeBrowser.Name) {
		s.Fatal("The chrome app is not opened in the chrome browser")
	}

	if err := openLauncher(ctx, tconn, tabletMode); err != nil {
		s.Fatal("Failed to open the launcher")
	}
	// Open the `app` as a window
	if err := openAppAs(ctx, tconn, app, window); err != nil {
		s.Fatal("Failed to open app in a new window for the first time: ", err)
	}

	appItemOnShelf := nodewith.NameContaining("fake app").ClassName("ash/ShelfAppButton")
	if err := ui.WaitUntilExists(appItemOnShelf)(ctx); err != nil {
		s.Fatal("The chrome app is not opened in a new window")
	}

	activeWindow, err = ash.GetActiveWindow(ctx, tconn)
	if err != nil {
		s.Fatal("Error getting the active window")
	}
	if !strings.Contains(activeWindow.Title, "fake app ") {
		s.Fatal("The chrome app window is not activated")
	}

	if err := openLauncher(ctx, tconn, tabletMode); err != nil {
		s.Fatal("Failed to open the launcher")
	}
	// Open the `app` as a window
	if err := openAppAs(ctx, tconn, app, window); err != nil {
		s.Fatal("Failed to open app in a new window for the second time: ", err)
	}

	if err := openLauncher(ctx, tconn, tabletMode); err != nil {
		s.Fatal("Failed to open the launcher")
	}
	// Open the `app` as a tab
	if err := openAppAs(ctx, tconn, app, tab); err != nil {
		s.Fatal("Failed to open app in a new tab for the second time: ", err)
	}

	// Check if the chrome browser is open now.
	activeWindow, err = ash.GetActiveWindow(ctx, tconn)
	if err != nil {
		s.Fatal("Error getting the active window")
	}
	if !strings.Contains(activeWindow.Title, chromeBrowser.Name) {
		s.Fatal("The chrome app is not opened in the chrome browser")
	}

	// Check the number of tabs.
	tabFinder := nodewith.ClassName("Tab")
	tabs, err := ui.NodesInfo(ctx, tabFinder)
	if err != nil {
		s.Fatal("Failed to get the tabs in the browser")
	}

	if len(tabs) != 2 {
		s.Fatalf("Wrong number of open tabs. The number of tabs should be %v but %v is opened", 2, len(tabs))
	}

	if tabletMode {
		// Small swipe up from the bottom should cause the hotseat shelf to become visible.
		tc, err := pointer.NewTouchController(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create the touch controller: ", err)
		}
		defer tc.Close()

		if err := ash.SwipeUpHotseatAndWaitForCompletion(ctx, tconn, tc.EventWriter(), tc.TouchCoordConverter()); err != nil {
			s.Fatal("Failed to swipe up the hotseat to show extended shelf: ", err)
		}
	}
	if err := ui.LeftClick(appItemOnShelf)(ctx); err != nil {
		s.Fatal("Failed to click on the fake app on shelf")
	}

	chromeAppInstances := nodewith.Name("Google").ClassName("MenuItemView")

	instances, err := ui.NodesInfo(ctx, chromeAppInstances)
	if err != nil {
		s.Fatal("Failed to get the chrome app instances")
	}

	if len(instances) != 4 {
		s.Fatalf("Wrong number of open instances. The number of instances should be %v but %v is opened", 4, len(instances))
	}
}

// openAppAs opens a chrome extension app as window/tab according to `instanceType`. The series of actions are done in launcher.
func openAppAs(ctx context.Context, tconn *chrome.TestConn, app *nodewith.Finder, instanceType newInstanceType) error {
	newInstanceMenuItem := nodewith.NameContaining("New ").ClassName("MenuItemView")

	ui := uiauto.New(tconn)

	if err := uiauto.Combine("Right click on the app to show context menu",
		ui.RightClick(app),
		ui.WaitUntilExists(newInstanceMenuItem),
	)(ctx); err != nil {
		return errors.Wrapf(err, "failed to open the context menu on %+q", app.Pretty())
	}

	if instanceType == tab {
		newInstanceMenuItem = nodewith.Name("New tab").ClassName("MenuItemView")
	} else {
		newInstanceMenuItem = nodewith.Name("New window").ClassName("MenuItemView")
	}

	isMenuItemFound, err := ui.IsNodeFound(ctx, newInstanceMenuItem)
	if err != nil {
		return errors.Wrapf(err, "error finding the New %s menu item", instanceType)
	}
	if !isMenuItemFound {
		if err := setLaunchAppAs(ctx, tconn, app, instanceType); err != nil {
			return err
		}
	} else {
		// If the default instance to open matches the `instanceType`, directly click on the menu item to open.
		if err := uiauto.Combine(fmt.Sprintf("Open the app in a new %s", instanceType),
			ui.LeftClick(newInstanceMenuItem),
			ui.WaitUntilGone(newInstanceMenuItem),
		)(ctx); err != nil {
			return err
		}
		return nil
	}

	// Right click on the context menu again to open a new window/tab on the app
	if err := uiauto.Combine(fmt.Sprintf("Open a new %s on the app", instanceType),
		ui.RightClick(app),
		ui.WaitUntilExists(newInstanceMenuItem),
		ui.LeftClick(newInstanceMenuItem),
	)(ctx); err != nil {
		return errors.Wrapf(err, "failed to open a new %s of %s", instanceType, app.Pretty())
	}

	return nil
}

// setLaunchAppAs set the default new chrome app instance to be opened as window/tab, according to `instanceType`.
// Note that this function doesn't actually open a new instance.
func setLaunchAppAs(ctx context.Context, tconn *chrome.TestConn, appFinder *nodewith.Finder, instanceType newInstanceType) error {
	newInstanceMenuItem := nodewith.NameContaining("New ").ClassName("MenuItemView")
	newTabOption := nodewith.Name("New tab").ClassName("MenuItemView")
	newWindowOption := nodewith.Name("New window").ClassName("MenuItemView")

	ui := uiauto.New(tconn)

	// Make sure the context menu on the target app is already showing
	isContextMenuShown, err := ui.IsNodeFound(ctx, newInstanceMenuItem)
	if err != nil || !isContextMenuShown {
		return errors.Wrap(err, "context menu on app is not available")
	}

	verticalSeparatorLocation, err := ui.Location(ctx, nodewith.ClassName("VerticalSeparator").Ancestor(newInstanceMenuItem))
	if err != nil {
		return errors.Wrap(err, "can not get the location of the vertical separator in MenuItemView. Is the menu item a actionable submenu?")
	}
	submenuArrow := coords.NewPoint(verticalSeparatorLocation.Right()+5, verticalSeparatorLocation.CenterY())

	targetButton := newTabOption
	if instanceType == window {
		targetButton = newWindowOption
	}

	if err := uiauto.Combine(fmt.Sprintf("Set to open the app as %s in default", instanceType),
		mouse.Move(tconn, submenuArrow, 0),
		ui.WaitUntilExists(targetButton),
		ui.LeftClick(targetButton),
		ui.WaitUntilGone(newInstanceMenuItem),
	)(ctx); err != nil {
		return errors.Wrapf(err, "failed to set the default new instance to %s", instanceType)
	}

	return nil
}

func openLauncher(ctx context.Context, tconn *chrome.TestConn, tabletMode bool) error {
	// Open the Launcher and go to Apps list page.
	if tabletMode {
		if err := launcher.OpenExpandedView(tconn)(ctx); err != nil {
			return errors.Wrap(err, "failed to open Expanded Application list view")
		}
	} else {
		if err := launcher.OpenBubbleLauncher(tconn)(ctx); err != nil {
			return errors.Wrap(err, "failed to open bubble launcher")
		}
	}
	return nil
}

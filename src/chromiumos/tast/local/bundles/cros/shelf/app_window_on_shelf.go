// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shelf

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
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
		Desc:         "Checks the window instances when opening an chrome app in a new window/tab",
		Contacts: []string{
			"wcwang@chromium.org",
			"tbarzic@chromium.org",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:    "productivity_launcher_clamshell_mode",
			Val:     launcher.TestCase{ProductivityLauncher: true, TabletMode: false},
			Fixture: "chromeLoggedInWith100FakeAppsProductivityLauncher",
		}, {
			Name:    "clamshell_mode",
			Val:     launcher.TestCase{ProductivityLauncher: false, TabletMode: false},
			Fixture: "chromeLoggedInWith100FakeAppsLegacyLauncher",
		}, {
			Name:    "productivity_launcher_tablet_mode",
			Val:     launcher.TestCase{ProductivityLauncher: true, TabletMode: true},
			Fixture: "chromeLoggedInWith100FakeAppsProductivityLauncher",
		}, {
			Name:    "tablet_mode",
			Val:     launcher.TestCase{ProductivityLauncher: false, TabletMode: true},
			Fixture: "chromeLoggedInWith100FakeAppsLegacyLauncher",
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

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)
	defer faillog.SaveScreenshotOnError(cleanupCtx, cr, s.OutDir(), s.HasError)

	testCase := s.Param().(launcher.TestCase)
	tabletMode := testCase.TabletMode
	productivityLauncher := testCase.ProductivityLauncher
	originallyEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to check if DUT is in tablet mode: ", err)
	}
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to ensure clamshell/tablet mode: ", err)
	}
	defer cleanup(ctx)

	// Ensure that the tablet launcher is closed before opening a launcher instance for test in clamshell.
	if originallyEnabled && !tabletMode {
		if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
			s.Fatal("Launcher not closed after transition to clamshell mode: ", err)
		}
	}

	usingBubbleLauncher := productivityLauncher && !tabletMode
	var appsGridView *nodewith.Finder
	// Open the Launcher and go to Apps list page.
	if usingBubbleLauncher {
		if err := launcher.OpenBubbleLauncher(tconn)(ctx); err != nil {
			s.Fatal("Failed to open bubble launcher: ", err)
		}
		appsGridView = nodewith.ClassName("ScrollableAppsGridView")
	} else {
		if err := launcher.OpenExpandedView(tconn)(ctx); err != nil {
			s.Fatal("Failed to open Expanded Application list view: ", err)
		}
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
	w, err := ash.GetActiveWindow(ctx, tconn)
	if err != nil {
		s.Fatal("Error getting the active window")
	}
	if !strings.Contains(w.Title, "Chrome") {
		s.Fatal("The chrome app is not opened in the chrome browser")
	}

	// Open the Launcher and go to Apps list page.
	if usingBubbleLauncher {
		if err := launcher.OpenBubbleLauncher(tconn)(ctx); err != nil {
			s.Fatal("Failed to open bubble launcher: ", err)
		}
	} else {
		if err := launcher.OpenExpandedView(tconn)(ctx); err != nil {
			s.Fatal("Failed to open Expanded Application list view: ", err)
		}
	}

	// Open the `app` as a window
	if err := openAppAs(ctx, tconn, app, window); err != nil {
		s.Fatal("Failed to open app in a new window for the first time: ", err)
	}

	fakeAppOnShelf := nodewith.NameContaining("fake app").ClassName("ash/ShelfAppButton")
	if err := ui.WaitUntilExists(fakeAppOnShelf)(ctx); err != nil {
		s.Fatal("The chrome app is not opened in a new window")
	}

	w, err = ash.GetActiveWindow(ctx, tconn)
	if err != nil {
		s.Fatal("Error getting the active window")
	}
	if !strings.Contains(w.Title, "fake app ") {
		s.Fatal("The chrome app window is not activated")
	}

	// Open the Launcher and go to Apps list page.
	if usingBubbleLauncher {
		if err := launcher.OpenBubbleLauncher(tconn)(ctx); err != nil {
			s.Fatal("Failed to open bubble launcher: ", err)
		}
	} else {
		if err := launcher.OpenExpandedView(tconn)(ctx); err != nil {
			s.Fatal("Failed to open Expanded Application list view: ", err)
		}
	}

	// Open the `app` as a window
	if err := openAppAs(ctx, tconn, app, window); err != nil {
		s.Fatal("Failed to open app in a new window for the second time: ", err)
	}

	// Open the Launcher and go to Apps list page.
	if usingBubbleLauncher {
		if err := launcher.OpenBubbleLauncher(tconn)(ctx); err != nil {
			s.Fatal("Failed to open bubble launcher: ", err)
		}
	} else {
		if err := launcher.OpenExpandedView(tconn)(ctx); err != nil {
			s.Fatal("Failed to open Expanded Application list view: ", err)
		}
	}

	// Open the `app` as a tab
	if err := openAppAs(ctx, tconn, app, tab); err != nil {
		s.Fatal("Failed to open app in a new tab for the second time: ", err)
	}

	// Check if the chrome browser is open now.
	w, err = ash.GetActiveWindow(ctx, tconn)
	if err != nil {
		s.Fatal("Error getting the active window")
	}
	if !strings.Contains(w.Title, "Chrome") {
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
	if err := ui.LeftClick(fakeAppOnShelf)(ctx); err != nil {
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

func openAppAs(ctx context.Context, tconn *chrome.TestConn, app *nodewith.Finder, instanceType newInstanceType) error {
	newInstanceMenuItem := nodewith.NameContaining("New ").ClassName("MenuItemView")
	newTabOption := nodewith.Name("New tab").ClassName("MenuItemView")
	newWindowOption := nodewith.Name("New window").ClassName("MenuItemView")

	ui := uiauto.New(tconn)

	if err := uiauto.Combine("Right click on the app to show context menu",
		ui.RightClick(app),
		ui.WaitUntilExists(newInstanceMenuItem),
	)(ctx); err != nil {
		return errors.Wrapf(err, "failed to open the context menu on %+q", app.Pretty())
	}

	// Check if the context menu is showing "New window" or "New tab"
	isNewTabShown, tabErr := ui.IsNodeFound(ctx, newTabOption)
	isNewWindowShown, windowErr := ui.IsNodeFound(ctx, newWindowOption)

	if tabErr != nil || windowErr != nil || (!isNewTabShown && !isNewWindowShown) {
		return errors.New("error checking the New window/tab menu item")
	}

	if instanceType == tab {
		if !isNewTabShown {
			if err := setLaunchAppAs(ctx, tconn, app, instanceType); err != nil {
				return err
			}
		} else {
			// If the default is to open a new tab, directly click on the menu item to open.
			if err := uiauto.Combine("Open the app in a new tab",
				ui.LeftClick(newTabOption),
				ui.WaitUntilGone(newInstanceMenuItem),
			)(ctx); err != nil {
				return err
			}
			return nil
		}
	} else {
		// instanceType == Window
		if !isNewWindowShown {
			if err := setLaunchAppAs(ctx, tconn, app, instanceType); err != nil {
				return err
			}
		} else {
			// If the default is to open a new window, directly click on the menu item to open.
			if err := uiauto.Combine("Open the app in a new window",
				ui.LeftClick(newWindowOption),
				ui.WaitUntilGone(newInstanceMenuItem),
			)(ctx); err != nil {
				return err
			}
			return nil
		}
	}

	// Right click on the context menu again to open a new window/tab on the app
	if err := uiauto.Combine("Open a new window/tab on the app",
		ui.RightClick(app),
		ui.WaitUntilExists(newInstanceMenuItem),
		ui.LeftClick(newInstanceMenuItem),
	)(ctx); err != nil {
		return errors.Wrapf(err, "failed to open a new %s of %s", instanceType, app.Pretty())
	}

	return nil
}

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

	if err := uiauto.Combine("Set to open the app as window/tab in default",
		mouse.Move(tconn, submenuArrow, 0),
		ui.WaitUntilExists(targetButton),
		ui.LeftClick(targetButton),
		ui.WaitUntilGone(newInstanceMenuItem),
	)(ctx); err != nil {
		return errors.Wrapf(err, "failed to set the default new instance to %s", instanceType)
	}

	return nil
}

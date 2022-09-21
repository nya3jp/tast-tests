// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
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
		Desc:         "Checks the window instances when opening a chrome app in a new window / tab",
		Contacts: []string{
			"wcwang@chromium.org",
			"tbarzic@chromium.org",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedInWith100FakeAppsNoAppSort",
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val:  launcher.TestCase{TabletMode: false},
		}, {
			Name:              "tablet_mode",
			Val:               launcher.TestCase{TabletMode: true},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}},
	})
}

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
	cleanup, err := launcher.SetUpLauncherTest(ctx, tconn, tabletMode, true /*stabilizeAppCount*/)
	if err != nil {
		s.Fatal("Failed to set up launcher test case: ", err)
	}
	defer cleanup(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	appsGridView := nodewith.ClassName("ScrollableAppsGridView")
	if tabletMode {
		appsGridView = nodewith.ClassName("AppsGridView")
	}

	// Wait for at least one fake app to be installed.
	fakeAppName := "fake app 1"
	if _, err := ash.WaitForChromeAppByNameInstalled(ctx, tconn, fakeAppName, 1*time.Minute); err != nil {
		s.Fatalf("Didn't find the installed %s: %v", fakeAppName, err)
	}

	// The app with `fakeAppName` doesn't necessarily be the first app in appsGridView. Pick the first fake app in node finder for the test.
	fakeApp := nodewith.NameContaining("fake app").Ancestor(appsGridView).First()
	ui := uiauto.New(tconn)

	// SetUpLauncherTest leaves launcher in show state - hide it before the first
	// call to openAppFromLauncherAs(), which would toggle bubble launcher when
	// called.
	if !tabletMode {
		if err := launcher.HideLauncher(tconn, !tabletMode)(ctx); err != nil {
			s.Fatal("Failed to hide the launcher at the test start: ", err)
		}
	}

	// Open the `app` as a tab. The tab should be in the chrome browser window.
	if err := openAppFromLauncherAs(ctx, tconn, tabletMode, fakeApp, tab); err != nil {
		s.Fatal("Failed to open app in a new tab for the first time: ", err)
	}

	chromeBrowser, err := apps.ChromeOrChromium(ctx, tconn)
	if err != nil {
		s.Fatal("Could not determine the correct Chrome app to use: ", err)
	}

	// Wait for the chrome window to open.
	appWindow, err := waitForAnyVisibleWindowWithTitle(ctx, tconn, chromeBrowser.Name)
	if err != nil {
		s.Fatal("Could not find the opened chrome browser that contains the fake app instance: ", err)
	}
	if err := ash.SetWindowStateAndWait(ctx, tconn, appWindow.ID, ash.WindowStateMinimized); err != nil {
		s.Fatal("Could not minimize the opened browser window: ", err)
	}

	// Open the `app` as a window.
	if err := openAppFromLauncherAs(ctx, tconn, tabletMode, fakeApp, window); err != nil {
		s.Fatal("Failed to open app in a new window for the first time: ", err)
	}

	appItemOnShelf := nodewith.NameContaining("fake app").ClassName("ash/ShelfAppButton")
	if err := ui.WaitUntilExists(appItemOnShelf)(ctx); err != nil {
		s.Fatal("The chrome app is not opened in a new window with an icon on shelf: ", err)
	}

	// The fake app open its window with the same title as the website title, which is "Google" in this case
	if _, err := waitForAnyVisibleWindowWithTitle(ctx, tconn, "Google"); err != nil {
		s.Fatal("Could not find the opened fake app window: ", err)
	}

	// Close all windows to make sure `waitForAnyVisibleWindowWithTitle` called next time can wait for the newly opened window.
	if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
		return ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateMinimized)
	}); err != nil {
		s.Fatal("Could not minimize all browser app windows: ", err)
	}

	// Open the `app` as a window.
	if err := openAppFromLauncherAs(ctx, tconn, tabletMode, fakeApp, window); err != nil {
		s.Fatal("Failed to open app in a new window for the second time: ", err)
	}

	// The fake app open its window with the same title as the website title, which is "Google" in this case
	if _, err := waitForAnyVisibleWindowWithTitle(ctx, tconn, "Google"); err != nil {
		s.Fatal("Could not find the opened fake app window: ", err)
	}

	// Close all windows to make sure `waitForAnyVisibleWindowWithTitle` called next time can wait for the newly opened window.
	if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
		return ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateMinimized)
	}); err != nil {
		s.Fatal("Could not minimize all browser app windows: ", err)
	}

	// Open the `app` as a tab. The tab should be in the chrome browser window.
	if err := openAppFromLauncherAs(ctx, tconn, tabletMode, fakeApp, tab); err != nil {
		s.Fatal("Failed to open app in a new tab for the second time: ", err)
	}

	if _, err := waitForAnyVisibleWindowWithTitle(ctx, tconn, chromeBrowser.Name); err != nil {
		s.Fatal("Could not find the opened chrome browser that contains the fake app instance: ", err)
	}

	// Check the number of tabs.
	tabs, err := browser.CurrentTabs(ctx, tconn)
	if err != nil {
		s.Fatal("Unable to retrieve tabs: ", err)
	}

	if len(tabs) != 2 {
		s.Fatalf("Wrong number of open tabs. The number of tabs should be %d but %d is opened", 2, len(tabs))
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
		s.Fatal("Failed to click on the fake app on shelf: ", err)
	}

	// The fake app opens Google webpage as default, so the instances with name "Google" are expected.
	chromeAppInstances := nodewith.Name("Google").ClassName("MenuItemView")

	instances, err := ui.NodesInfo(ctx, chromeAppInstances)
	if err != nil {
		s.Fatal("Failed to get the chrome app instances: ", err)
	}

	if len(instances) != 4 {
		s.Fatalf("Wrong number of open instances. The number of instances should be %d but %d is opened", 4, len(instances))
	}
}

// openAppFromLauncherAs opens a chrome extension app as window / tab according to `instanceType`. The series of actions are done in launcher.
// Note that this function expect `app` Finder to exist and be visible.
func openAppFromLauncherAs(ctx context.Context, tconn *chrome.TestConn, tabletMode bool, app *nodewith.Finder, instanceType newInstanceType) error {
	if err := launcher.OpenProductivityLauncher(ctx, tconn, tabletMode); err != nil {
		return errors.Wrap(err, "failed to open the launcher")
	}

	newInstanceMenuItem := nodewith.NameContaining("New ").ClassName("MenuItemView")

	ui := uiauto.New(tconn)

	if err := uiauto.Combine("Right click on the app to show context menu",
		ui.RightClick(app),
		ui.WaitUntilExists(newInstanceMenuItem),
	)(ctx); err != nil {
		return errors.Wrapf(err, "failed to open the context menu on %s", app.Pretty())
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

	// If the current app launch type is different from the target one, update it.
	if !isMenuItemFound {
		if err := setLaunchAppAs(ctx, tconn, newInstanceMenuItem, instanceType); err != nil {
			return err
		}
		// Setting the app launch type from the app context menu will close the context menu.
		// Reopen it, and the rest of the method assumes the context menu is open.
		if err := uiauto.Combine("Right click on the app again to show context menu",
			ui.RightClick(app),
			ui.WaitUntilExists(newInstanceMenuItem),
		)(ctx); err != nil {
			return errors.Wrapf(err, "failed to open the context menu on %s", app.Pretty())
		}
	}

	// Click on the menu option to open a new window / tab on the app.
	if err := ui.LeftClick(newInstanceMenuItem)(ctx); err != nil {
		return errors.Wrapf(err, "failed to open a new %s of %s", instanceType, app.Pretty())
	}

	return nil
}

// setLaunchAppAs set the default new chrome app instance to be opened as window / tab, according to `instanceType`.
// Note that this function doesn't actually open a new instance.
func setLaunchAppAs(ctx context.Context, tconn *chrome.TestConn, newInstanceMenuItem *nodewith.Finder, instanceType newInstanceType) error {
	ui := uiauto.New(tconn)

	// Make sure the context menu on the target app is already showing.
	contextMenu := nodewith.ClassName("SubmenuView")
	isContextMenuShown, err := ui.IsNodeFound(ctx, contextMenu)
	if err != nil || !isContextMenuShown {
		return errors.Wrap(err, "context menu on app is not available")
	}

	verticalSeparatorLocation, err := ui.Location(ctx, nodewith.ClassName("VerticalSeparator").Ancestor(contextMenu))
	if err != nil {
		return errors.Wrap(err, "can not get the location of the vertical separator in MenuItemView. Is the menu item a actionable submenu?")
	}
	submenuArrow := coords.NewPoint(verticalSeparatorLocation.Right()+5, verticalSeparatorLocation.CenterY())

	if err := uiauto.Combine(fmt.Sprintf("Set to open the app as %s in default", instanceType),
		mouse.Move(tconn, submenuArrow, 0),
		ui.WaitUntilExists(newInstanceMenuItem),
		ui.LeftClick(newInstanceMenuItem),
		ui.WaitUntilGone(newInstanceMenuItem),
	)(ctx); err != nil {
		return errors.Wrapf(err, "failed to set the default new instance to %s", instanceType)
	}

	return nil
}

// waitForAnyVisibleWindowWithTitle waits for the first window whose title is title to be visible.
func waitForAnyVisibleWindowWithTitle(ctx context.Context, tconn *chrome.TestConn, title string) (*ash.Window, error) {
	return ash.WaitForAnyWindow(ctx, tconn, func(w *ash.Window) bool {
		return strings.Contains(w.Title, title) && w.IsVisible
	})
}

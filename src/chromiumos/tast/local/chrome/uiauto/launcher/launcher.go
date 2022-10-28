// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package launcher is used for controlling the launcher directly through the UI.
package launcher

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// ExpandedItemsClass define the class name of the expanded launcher view which is used as search parameters in ui.
const ExpandedItemsClass = "AppListItemView"

// BubbleAppsGridViewClass defines the class name of the bubble apps grid.
const BubbleAppsGridViewClass = "ScrollableAppsGridView"

// BubbleSearchPage defines the class name of the bubble apps search page.
const BubbleSearchPage = "AppListBubbleSearchPage"

// BubbleAppsPage defines the class name of the bubble apps search page.
const BubbleAppsPage = "AppListBubbleAppsPage"

// PagedAppsGridViewClass defines the class name of the paged apps grid.
const PagedAppsGridViewClass = "AppsGridView"

// SearchResultPageView defines the class name of the search view shown in
// tablet mode.
const SearchResultPageView = "SearchResultPageView"

// UnnamedFolderFinder is the finder of a newly created folder with the default name.
var UnnamedFolderFinder = nodewith.Name("Folder Unnamed").ClassName(ExpandedItemsClass)

// SearchResultListItemFinder is the finder of the list items in launcher search result.
var SearchResultListItemFinder = nodewith.ClassName("ui/app_list/SearchResultView")

// SearchResultListViewFinder is the finder of the list views in launcher search result.
var SearchResultListViewFinder = nodewith.ClassName("SearchResultListView")

// SearchResultListLabelFinder is the finder of the list label in launcher search result.
var SearchResultListLabelFinder = nodewith.ClassName("Label")

// ReorderEducationNudgeFinder is the finder of the reorder education nudge.
var ReorderEducationNudgeFinder = nodewith.ClassName("Label").Name("Sort your apps by name or color")

// TestCase describes modes in which the launcher UI can be shown, and by which launcher test should generally be parameterized.
// Use a struct because it makes the individual test cases more readable.
type TestCase struct {
	TabletMode bool // Whether the test runs in tablet mode
}

// SortType Indicates the order that the launcher is sorted with.
type SortType string

const (
	// AlphabeticalSort indicates the items are sorted with the app name alphabetical order.
	AlphabeticalSort SortType = "alphabetical sort"

	// ColorSort indicates the items are sorted with the app icon color order.
	ColorSort SortType = "color sort"
)

// FakeAppInfoForSort defines the fake apps' info used in tests that verify app list sort.
type FakeAppInfoForSort struct {
	// AlphabeticalNames refers to an array of strings sorted in alphabetical order. These strings are used as app names when installing fake apps.
	AlphabeticalNames []string

	// ColorOrderNames refers to the app names whose corresponding icons follow the color order.
	ColorOrderNames []string

	// IconFileNames indicates the icon files used by fake apps. NOTE: IconFileNames is associated with ColorOrderNames, i.e. the app that uses
	// the i-th element of ColorOrderNames as the app name utilizes the i-th element of IconFileNames as the icon file.
	IconFileNames []string

	// AlphabeticalNamesAfterAppInstall defines the app names in alphabetical order after the extra app installation. The tests that verify app installation
	// with app list sorted use it.
	AlphabeticalNamesAfterAppInstall []string

	// ColorOrderNamesAfterAppInstall defines the app names in color order after the extra app installation. The tests that verify app installation
	// with app list sorted use it.
	ColorOrderNamesAfterAppInstall []string
}

// SortTestType specifies the test parameters for sort-related tests.
type SortTestType struct {
	TabletMode                  bool     // Whether the test runs in tablet mode
	SortMethod                  SortType // Indicates the sort method used in tests
	OrderedAppNames             []string // Specifies the fake app names arranged in the expected sort order
	OrderedAppNamesAfterInstall []string // Indicates the fake app names in order after fake app installation. Used by the tests that verify app installation after sort
}

// WaitForCategoryLabel waits for a search result list view of type 'category'
// to be created and labeled.
func WaitForCategoryLabel(tconn *chrome.TestConn, category, categoryLabel string) uiauto.Action {
	ui := uiauto.New(tconn)
	categoryListView := SearchResultListViewFinder.Name(category)
	return ui.WaitUntilExists(SearchResultListLabelFinder.Name(categoryLabel).Ancestor(categoryListView))
}

// WaitForCategorizedResult waits for a search result list view of type
// 'category' to be populated with 'result'.
func WaitForCategorizedResult(tconn *chrome.TestConn, category, result string) uiauto.Action {
	ui := uiauto.New(tconn)
	categoryListView := SearchResultListViewFinder.Name(category)
	return ui.WaitUntilExists(SearchResultListItemFinder.Name(result).Ancestor(categoryListView))
}

// SetUpLauncherTest performs common launcher test setup steps that set tablet
// mode state, and open the launcher.
// tabletMode indicates whether the test uses tablet mode or clamshell mode
// launcher.
// stabilizeAppCount indicates whether setup should wait for the number of
// apps shown in the launcher to stabilize (and for default system apps to
// finish installing). This can be skipped by tests that don't interact with app
// items in the app list directly - for example, while testing launcher search.
// If unsure, set this to true.
//
// Returns a method that should be called to reset system UI state set by this
// method.
// Expected usage is:
//
//	cleanup, err := launcher.SetUpLauncherTest(ctx, tconn, ...)
//	if err != nil {
//		s.Fatal("Test setup failed: ", err)
//	}
//	defer cleanup(ctx)
func SetUpLauncherTest(ctx context.Context, tconn *chrome.TestConn, tabletMode, stabilizeAppCount bool) (func(ctx context.Context), error) {
	cleanupTabletMode, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to ensure tablet mode state %t", tabletMode)
	}

	if !tabletMode {
		// Ensure that tablet mode launcher animation completes before proceeding with tests.
		if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
			cleanupTabletMode(ctx)
			return nil, errors.Wrap(err, "Launcher not closed after transition to clamshell mode")
		}
	}

	if err := OpenProductivityLauncher(ctx, tconn, tabletMode); err != nil {
		cleanupTabletMode(ctx)
		return nil, errors.Wrap(err, "failed to open bubble launcher")
	}

	// Function that presses the Escape key twice to ensure that the
	// clamshell mode launcher is closed. Pressing the Escape key once is
	// not always enough - if a folder is open, or the launcher is showing
	// search results, pressing the Escape key will go back to the launcher
	// apps page.
	ensureLauncherClosed := func(ctx context.Context) error {
		kb, err := input.Keyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to find keyboard")
		}
		defer kb.Close()

		if err := kb.TypeKey(ctx, input.KEY_ESC); err != nil {
			return errors.Wrapf(err, "failed to send %d", input.KEY_ESC)
		}
		if err := kb.TypeKey(ctx, input.KEY_ESC); err != nil {
			return errors.Wrapf(err, "failed to send %d for the second time", input.KEY_ESC)
		}

		if !tabletMode {
			ui := uiauto.New(tconn)
			if err := ui.WaitUntilGone(nodewith.ClassName(BubbleAppsGridViewClass))(ctx); err != nil {
				return errors.Wrap(err, "Bubble launcher faild to close")
			}
		}
		return nil
	}

	// Wait for the set of apps show in the launcher to stabilize - app insertion into the apps grid may interfere with
	// certain operations within the apps grid (for example, app list item drag), and may cause failures if a default app
	// is installed mid test.
	if stabilizeAppCount {
		if err := WaitForStableNumberOfApps(ctx, tconn); err != nil {
			ensureLauncherClosed(ctx)
			cleanupTabletMode(ctx)
			return nil, errors.Wrap(err, "failed to wait for item count in app list to stabilize")
		}
	}

	return func(ctx context.Context) {
		ensureLauncherClosed(ctx)
		cleanupTabletMode(ctx)
	}, nil
}

// SetupContinueSectionFiles creates and opens enough files for the continue section
// suggestions to show up. The files are created in the user's Downloads folder.
// Returns:
// - A function that deletes the temporary files
// - A list of the file names (just the file name, not the full path)
// - An error or nil.
//
// Expected usage:
// cleanupFiles, testFileNames, err := launcher.SetupContinueSectionFiles(...)
// if err != nil { ... }
// defer cleanupFiles()
func SetupContinueSectionFiles(ctx context.Context, tconn *chrome.TestConn,
	cr *chrome.Chrome, tabletMode bool) (func(), []string, error) {
	// Create enough fake files to show the continue section.
	var numFiles int
	if tabletMode {
		numFiles = 2
	} else {
		numFiles = 3
	}

	// Create the files in the user's Downloads directory.
	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get user's Download path")
	}
	var testDocFileNames []string
	var testDocFilePaths []string
	for i := 0; i < numFiles; i++ {
		testFileName := fmt.Sprintf("fake-file-%d.html", i)
		testDocFileNames = append(testDocFileNames, testFileName)
		// Create a test file.
		filePath := filepath.Join(downloadsPath, testFileName)
		fileContent := fmt.Sprintf("Test file %d", i)
		if err := ioutil.WriteFile(filePath, []byte(fileContent), 0644); err != nil {
			return nil, nil, errors.Wrapf(err, "failed to create file %d in Downloads", i)
		}
		testDocFilePaths = append(testDocFilePaths, filePath)
	}

	// Create a cleanup function. This can't be deferred because we need to
	// return it to the caller (who might need to manipulate the files).
	cleanupFiles := func() {
		for _, path := range testDocFilePaths {
			os.Remove(path)
		}
	}

	// Launch the Files app.
	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		cleanupFiles()
		return nil, nil, errors.Wrap(err, "could not launch the Files App")
	}
	defer filesApp.Close(ctx)

	// Files need to be opened for them to get picked up for the Continue Section.
	chromeApp, err := apps.ChromeOrChromium(ctx, tconn)
	for i, filePath := range testDocFileNames {
		if err := uiauto.Combine("Open file",
			filesApp.OpenDownloads(),
			filesApp.OpenFile(filePath),
		)(ctx); err != nil {
			cleanupFiles()
			return nil, nil,
				errors.Wrapf(err, "failed open the file %d - %s", i, filePath)
		}

		if err := ash.WaitForApp(ctx, tconn, chromeApp.ID, 10*time.Second); err != nil {
			cleanupFiles()
			return nil, nil,
				errors.Wrapf(err, "file %d - %s never opened", i, filePath)
		}

		if err := apps.Close(ctx, tconn, chromeApp.ID); err != nil {
			cleanupFiles()
			return nil, nil,
				errors.Wrap(err, "failed to close browser")
		}

		if err := ash.WaitForAppClosed(ctx, tconn, chromeApp.ID); err != nil {
			cleanupFiles()
			return nil, nil,
				errors.Wrap(err, "browser did not close successfully")
		}
	}
	return cleanupFiles, testDocFileNames, nil
}

// CreateAppSearchFinder creates a finder for an app search result in the current launcher search UI.
// It expects the launcher search page to be open - search containers within which apps are searched depend on
// whether productivity launcher is enabled, which is inferred from the current app list search UI state.
func CreateAppSearchFinder(ctx context.Context, tconn *chrome.TestConn, appName string) *nodewith.Finder {
	ui := uiauto.New(tconn)
	// Look for results in different search containers depending on productivity launcher flag.
	// ProductivityLauncherSearchView for productivity launcher.
	// SearchResultPageView otherwise.
	if err := ui.Exists(nodewith.ClassName("AppListBubbleView"))(ctx); err == nil {
		return AppSearchFinder(appName, "ProductivityLauncherSearchView")
	}
	return AppSearchFinder(appName, "SearchResultPageView")
}

// SearchAndWaitForAppOpen return a function that searches for an app, launches it, and waits for it to be open.
func SearchAndWaitForAppOpen(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, app apps.App) uiauto.Action {
	return uiauto.Combine(fmt.Sprintf("SearchAndWaitForAppOpen(%+q)", app),
		SearchAndLaunch(tconn, kb, app.Name),
		func(ctx context.Context) error {
			return ash.WaitForApp(ctx, tconn, app.ID, time.Minute)
		},
	)
}

// SearchAndLaunch return a function that searches an app in the launcher and executes it.
func SearchAndLaunch(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, appName string) uiauto.Action {
	return SearchAndLaunchWithQuery(tconn, kb, appName, appName)
}

// SearchAndLaunchWithQuery return a function that searches a query in the launcher and executes an app from the list.
func SearchAndLaunchWithQuery(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, query, appName string) uiauto.Action {
	ui := uiauto.New(tconn)
	return uiauto.Combine(fmt.Sprintf("SearchAndLaunchWithQuery(%s, %s)", query, appName),
		Open(tconn),
		Search(tconn, kb, query),
		func(ctx context.Context) error {
			return ui.WithInterval(time.Second).DoDefault(CreateAppSearchFinder(ctx, tconn, appName))(ctx)
		},
	)
}

// SearchAndRightClick returns a function that searches a query in the launcher and right click the app from the list.
// It right clicks the app until a menu item is displayed.
func SearchAndRightClick(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, query, appName string) uiauto.Action {
	ui := uiauto.New(tconn)
	menuItem := nodewith.Role(role.MenuItem).First()
	app := AppSearchFinder(appName, "SearchResultPageView")
	return uiauto.Combine(fmt.Sprintf("SearchAndRightClick(%s, %s)", query, appName),
		Open(tconn),
		Search(tconn, kb, query),
		ui.WithTimeout(time.Minute).WithInterval(10*time.Second).RightClickUntil(app, ui.WaitUntilExists(menuItem)),
	)
}

// Open return a function that opens the launcher.
func Open(tconn *chrome.TestConn) uiauto.Action {
	return OpenExpandedView(tconn)
}

// ShowLauncher shows the launcher and waits until the launcher is visible. isBubbleLauncher specifies the type of launcher to show.
func ShowLauncher(tconn *chrome.TestConn, isBubbleLauncher bool) uiauto.Action {
	if isBubbleLauncher {
		return OpenBubbleLauncher(tconn)
	}

	return OpenExpandedView(tconn)
}

// HideLauncher hides the launcher and waits until the launcher is hidden or its state becomes "hidden". isBubbleLauncher specifies the type of launcher to hide.
func HideLauncher(tconn *chrome.TestConn, isBubbleLauncher bool) uiauto.Action {
	if isBubbleLauncher {
		return CloseBubbleLauncher(tconn)
	}

	return HideTabletModeLauncher(tconn)
}

// OpenExpandedView return a function that opens the Launcher to the Apps list page.
func OpenExpandedView(tconn *chrome.TestConn) uiauto.Action {
	return func(ctx context.Context) error {
		// TODO: Call autotestPrivate API instead after http://crbug.com/1127384 is implemented.
		ui := uiauto.New(tconn)
		// The app list widget may exist and be hidden (cached), so explicitly
		// check for a visible app list item.
		appListItem := nodewith.ClassName(ExpandedItemsClass).Visible().First()
		if err := ui.Exists(appListItem)(ctx); err == nil {
			// Even if it exist, active window may cover it in tablet mode. Check for active windows.
			windows, err := ash.FindAllWindows(ctx, tconn, func(window *ash.Window) bool {
				return window.IsVisible
			})
			if err != nil {
				return errors.Wrap(err, "failed to get all windows")
			}
			// Do nothing if there are no windows. Homescreen should be there already.
			if len(windows) == 0 {
				return nil
			}
		}

		if err := ash.TriggerLauncherStateChange(ctx, tconn, ash.AccelShiftSearch); err != nil {
			return errors.Wrap(err, "failed to switch to fullscreen")
		}

		if err := ash.WaitForLauncherState(ctx, tconn, ash.FullscreenAllApps); err != nil {
			// WaitForLauncherState is expected to fail for bubble launcher - use uiauto
			// API to wait for the bubble launcher location to stabilize.
			if strings.Contains(err.Error(), "Not supported for bubble launcher") {
				bubbleLauncher := nodewith.ClassName("AppListBubbleView")
				if err := ui.WaitUntilExists(bubbleLauncher)(ctx); err != nil {
					return errors.Wrap(err, "failed waiting for bubble launcher")
				}
				if err := ui.WaitForLocation(bubbleLauncher)(ctx); err != nil {
					return errors.Wrap(err, "failed waiting for bubble launcher location to stabilize")
				}
				return nil
			}

			return errors.Wrap(err, "failed to switch the state to 'FullscreenAllApps'")
		}

		return nil
	}
}

// HideTabletModeLauncher returns a function that hides the launcher in tablet mode by launching the Chrome browser.
func HideTabletModeLauncher(tconn *chrome.TestConn) uiauto.Action {
	return func(ctx context.Context) error {
		browser, err := apps.PrimaryBrowser(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get browser app")
		}

		if err = apps.Launch(ctx, tconn, browser.ID); err != nil {
			return errors.Wrap(err, "failed to launch browser")
		}

		if err = ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
			return errors.Wrap(err, "failed to hide the launcher")
		}

		return nil
	}
}

// OpenBubbleLauncher opens launcher using search accelerator and  waits until the bubble launcher UI becomes visible.
func OpenBubbleLauncher(tconn *chrome.TestConn) uiauto.Action {
	bubbleLauncher := nodewith.ClassName("AppListBubbleView")
	ui := uiauto.New(tconn)
	return uiauto.Combine("Wait for bubble launcher visibility",
		func(ctx context.Context) error {
			if err := ash.TriggerLauncherStateChange(ctx, tconn, ash.AccelSearch); err != nil {
				return errors.Wrap(err, "failed to trigger search accelerator to open launcher")
			}
			return nil
		},
		ui.WaitUntilExists(bubbleLauncher),
		ui.WaitForLocation(bubbleLauncher),
	)
}

// CloseBubbleLauncher closes launcher by mouse clicking at the home button.
func CloseBubbleLauncher(tconn *chrome.TestConn) uiauto.Action {
	bubbleLauncher := nodewith.ClassName(BubbleAppsGridViewClass)
	ui := uiauto.New(tconn)
	return uiauto.Combine("Wait for bubble launcher to be closed",
		ui.LeftClick(nodewith.ClassName("ash/HomeButton")),
		ui.WaitUntilGone(bubbleLauncher),
	)
}

// WaitForLauncherSearchExit waits for the launcher to exit the search UI.
func WaitForLauncherSearchExit(tconn *chrome.TestConn, tabletMode bool) uiauto.Action {
	if tabletMode {
		return WaitForTabletLauncherSearchExit(tconn)
	}
	return WaitForClamshellLauncherSearchExit(tconn)
}

// WaitForTabletLauncherSearchExit waits for the search page to be hidden.
func WaitForTabletLauncherSearchExit(tconn *chrome.TestConn) uiauto.Action {
	ui := uiauto.New(tconn)
	return uiauto.Combine("Wait for bubble launcher search to be closed and apps page to be shown",
		ui.WaitUntilGone(nodewith.ClassName(SearchResultPageView)),
	)
}

// WaitForClamshellLauncherSearchExit waits for the search page to be hidden and the
// apps page to be shown.
func WaitForClamshellLauncherSearchExit(tconn *chrome.TestConn) uiauto.Action {
	ui := uiauto.New(tconn)
	return uiauto.Combine("Wait for bubble launcher search to be closed and apps page to be shown",
		ui.WaitUntilGone(nodewith.ClassName(BubbleSearchPage)),
		ui.WaitUntilExists(nodewith.ClassName(BubbleAppsPage)),
	)
}

// AppSearchFinder returns a Finder to find the specified app in an open launcher's search results.
func AppSearchFinder(appName, searchContainer string) *nodewith.Finder {
	searchResultView := nodewith.ClassName(searchContainer)
	re := regexp.MustCompile(regexp.QuoteMeta(appName) + ", [Ii]nstalled [Aa]pp")
	return nodewith.NameRegex(re).Ancestor(searchResultView)
}

// AppItemViewFinder returns a Finder to find the specified app in an open launcher's item view.
func AppItemViewFinder(appName string) *nodewith.Finder {
	return nodewith.Name(appName).ClassName(ExpandedItemsClass)
}

// Search return a function that executes a search query.
// Launcher should be open already.
func Search(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, query string) uiauto.Action {
	return func(ctx context.Context) error {
		// Click the search box.
		ui := uiauto.New(tconn)
		if err := ui.LeftClick(nodewith.ClassName("SearchBoxView").Visible().First())(ctx); err != nil {
			return errors.Wrap(err, "failed to click launcher searchbox")
		}

		// Search for anything by typing query string.
		if err := kb.Type(ctx, query); err != nil {
			return errors.Wrapf(err, "failed to type %q", query)
		}
		return nil
	}
}

// GetUndoButtonNameForSortType returns the undo button's name based on the sorting method.
func GetUndoButtonNameForSortType(sortType SortType) string {
	var undoButtonName string
	switch sortType {
	case AlphabeticalSort:
		undoButtonName = "Undo sort order by name"
	case ColorSort:
		undoButtonName = "Undo sort order by color"
	}
	return undoButtonName
}

// TriggerAppListSortAndWaitForUndoButtonExist sorts app list items through the
// item context menu with the specified sorting method. Waits until the undo
// button exists.
func TriggerAppListSortAndWaitForUndoButtonExist(ctx context.Context, ui *uiauto.Context, sortType SortType, item *nodewith.Finder) error {
	var sortMenuName string
	switch sortType {
	case AlphabeticalSort:
		sortMenuName = "Name"
	case ColorSort:
		sortMenuName = "Color"
	}

	sortContextMenuItem := nodewith.Name(sortMenuName).ClassName("MenuItemView")
	reorderContextMenuItem := nodewith.Name("Sort by").ClassName("MenuItemView")
	undoButton := nodewith.Name(GetUndoButtonNameForSortType(sortType)).ClassName("PillButton")

	if err := uiauto.Combine("sort app list items through the context menu",
		ui.RightClick(item),
		ui.WaitUntilExists(reorderContextMenuItem),
		ui.MouseMoveTo(reorderContextMenuItem, 0),
		ui.WaitUntilExists(sortContextMenuItem),
		ui.LeftClick(sortContextMenuItem),
		ui.WaitUntilExists(undoButton),
	)(ctx); err != nil {
		return errors.Wrapf(err, "failed to trigger %v from an item's context menu", sortType)
	}

	return nil
}

// LaunchAndWaitForAppOpen return a function that launches an app from the expanded launcher and waits for it to be open.
func LaunchAndWaitForAppOpen(tconn *chrome.TestConn, app apps.App) uiauto.Action {
	return uiauto.Combine(fmt.Sprintf("LaunchAndWaitForAppOpen(%+q)", app),
		LaunchApp(tconn, app.Name),
		func(ctx context.Context) error {
			return ash.WaitForApp(ctx, tconn, app.ID, time.Minute)
		},
	)
}

// LaunchApp return a function that launches an app from the expanded launcher.
func LaunchApp(tconn *chrome.TestConn, appName string) uiauto.Action {
	ui := uiauto.New(tconn)
	return uiauto.Combine(fmt.Sprintf("LaunchApp(%s)", appName),
		OpenExpandedView(tconn),
		ui.FocusAndWait(AppItemViewFinder(appName).First()),
		ui.LeftClick(AppItemViewFinder(appName).First()),
	)
}

// PinAppToShelf return a function that pins an app from the expanded launcher to shelf.
// It assumes that launcher UI is already opened before it is called.
func PinAppToShelf(tconn *chrome.TestConn, app apps.App, container *nodewith.Finder) uiauto.Action {
	ui := uiauto.New(tconn)
	return uiauto.Combine(fmt.Sprintf("PinAppToShelf(%+q)", app),
		ui.FocusAndWait(AppItemViewFinder(app.Name).Ancestor(container)),
		ui.RightClick(AppItemViewFinder(app.Name).Ancestor(container)),
		ui.LeftClick(nodewith.Name("Pin to shelf").ClassName("MenuItemView")),
	)
}

// UnpinAppFromShelf return a function that unpins an app from the shelf using a context menu in the expanded launcher UI.
func UnpinAppFromShelf(tconn *chrome.TestConn, app apps.App, container *nodewith.Finder) uiauto.Action {
	ui := uiauto.New(tconn)
	return uiauto.Combine(fmt.Sprintf("UnpinAppFromShelf(%+q)", app),
		OpenExpandedView(tconn),
		ui.FocusAndWait(AppItemViewFinder(app.Name).Ancestor(container)),
		ui.RightClick(AppItemViewFinder(app.Name).Ancestor(container)),
		ui.LeftClick(nodewith.Name("Unpin from shelf").ClassName("MenuItemView")),
	)
}

// WaitForStableNumberOfApps waits for the number of apps shown in the app list to stabilize. As a special case,
// waits for all system web apps to finish installing, as web app installation may add an item to the app list.
func WaitForStableNumberOfApps(ctx context.Context, tconn *chrome.TestConn) error {
	if err := tconn.Call(ctx, nil, "tast.promisify(chrome.autotestPrivate.waitForSystemWebAppsInstall)"); err != nil {
		return errors.Wrap(err, "failed to wait for all system web apps to be installed")
	}

	ui := uiauto.New(tconn)
	latestCount := -1
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		items, err := ui.NodesInfo(ctx, nodewith.ClassName("AppListItemView"))
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to collect app list items"))
		}

		currentCount := len(items)
		if currentCount == latestCount {
			return nil
		}

		latestCount = currentCount
		return errors.New("Number of items changed")
	}, &testing.PollOptions{Timeout: 5 * time.Second, Interval: time.Second}); err != nil {
		return errors.Wrap(err, "Number of apps in launcher unstable")
	}

	return nil
}

// RenameFolder return a function that renames a folder to a new name.
// from is the node finder for the folder to be renamed - RenameFolder will fail if the target node is not a folder item.
func RenameFolder(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, from *nodewith.Finder, to string) uiauto.Action {
	// Chrome add prefix "Folder " to all folder names in AppListItemView.
	toFolder := nodewith.Name("Folder " + to).ClassName(ExpandedItemsClass)
	folderView := nodewith.ClassName("AppListFolderView")
	ui := uiauto.New(tconn)
	return uiauto.Combine(fmt.Sprintf("RenameFolder to %s", to),
		OpenExpandedView(tconn),
		ui.LeftClick(from),
		ui.WaitUntilExists(folderView),
		ui.FocusAndWait(nodewith.ClassName("Textfield").Ancestor(folderView)),
		func(ctx context.Context) error {
			return kb.Type(ctx, to+"\n")
		},
		func(ctx context.Context) error {
			return kb.Accel(ctx, "esc")
		},
		ui.WaitUntilGone(folderView),
		ui.WaitUntilExists(toFolder),
	)
}

// IndexOfFirstVisibleItem returns the index of the first item which is visible in the current app list UI,
// and whose index among app list item views is at least minIndex.
func IndexOfFirstVisibleItem(ctx context.Context, tconn *chrome.TestConn, minIndex int) (int, error) {
	for itemIndex := minIndex; ; itemIndex++ {
		item := nodewith.ClassName(ExpandedItemsClass).Nth(itemIndex)
		onPage, err := IsItemOnCurrentPage(ctx, tconn, item)
		if err != nil {
			return -1, errors.Wrapf(err, "failed to query whether item is on page %d", itemIndex)
		}

		if onPage {
			return itemIndex, nil
		}
	}
}

// FirstNonRecentAppItem returns the first app list item view shown in the current app list UI that is not in the recent apps container.
// If productivity launcher is disabled, in which case recent apps container does not exist, return 0 - the index of the first app list item view.
// The return value will be -1 on error.
func FirstNonRecentAppItem(ctx context.Context, tconn *chrome.TestConn) (int, error) {
	ui := uiauto.New(tconn)
	recentAppsContainer := nodewith.ClassName("RecentAppsView")
	// If the recent apps container is not present (which will be the case if productivity launcher is disabled),
	// all app list items are non-recent app items, so return the index of the first one.
	if err := ui.Exists(recentAppsContainer)(ctx); err != nil {
		return 0, nil
	}

	recentAppsLocation, err := ui.Location(ctx, recentAppsContainer)
	if err != nil {
		return -1, errors.Wrap(err, "failed to query recent apps")
	}

	// Get the index of the first non-folder item.
	for itemIndex := 0; ; itemIndex++ {
		itemLocation, err := ui.Location(ctx, nodewith.ClassName(ExpandedItemsClass).Nth(itemIndex))
		if err != nil {
			return -1, errors.Wrap(err, "failed to get item locatoin")
		}
		if !recentAppsLocation.Contains(*itemLocation) {
			return itemIndex, nil
		}
	}
}

// CloseFolderView closes app list folder view - expects that the app list UI is currently showing a folder content.
func CloseFolderView(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)
	folderView := nodewith.ClassName("AppListFolderView")
	if err := ui.WaitUntilExists(folderView)(ctx); err != nil {
		return errors.Wrap(err, "failed to find an open folder")
	}

	folderViewLocation, err := ui.Location(ctx, folderView)
	if err != nil {
		return errors.Wrap(err, "failed to get folderViewLocation")
	}
	pointOutsideFolder := coords.NewPoint(folderViewLocation.Right()+50, folderViewLocation.CenterY()+50)

	// Click to close the folder.
	if err := mouse.Click(tconn, pointOutsideFolder, mouse.LeftButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to click outside of the folder")
	}

	return nil
}

// CreateFolder is a helper function to create a folder by dragging the first non-folder item on top of the second non-folder item.
// folderOpensOnCreation indicates whether the folder view expected to get opened after creation (with no extra user input).
func CreateFolder(ctx context.Context, tconn *chrome.TestConn) error {
	// When productivity launcher is enabled, first row of items in the app list will be recent apps, which are not draggable, and cannot
	// be used for tests that create folder.
	precedingRecentApps, err := FirstNonRecentAppItem(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to count recent apps items")
	}

	firstItem := precedingRecentApps - 1

	// Get the index of the first non-folder item.
	for {
		firstItem++
		item := nodewith.ClassName(ExpandedItemsClass).Nth(firstItem)

		isFolder, err := IsFolderItem(ctx, tconn, item)
		if err != nil {
			return errors.Wrap(err, "failed to check if item is a folder")
		}
		if !isFolder {
			break
		}
	}

	// Get the index of the second non-folder item.
	secondItem := firstItem
	for {
		secondItem++
		item := nodewith.ClassName(ExpandedItemsClass).Nth(secondItem)

		isFolder, err := IsFolderItem(ctx, tconn, item)
		if err != nil {
			return errors.Wrap(err, "failed to check if item is a folder")
		}
		if !isFolder {
			break
		}
	}

	if err := DragIconToIcon(tconn, secondItem, firstItem)(ctx); err != nil {
		return errors.Wrap(err, "failed to drag icon over another icon")
	}

	// Folders get opened automatically on creation by user gesture, so close the folder view.
	if err := CloseFolderView(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to close the folder")
	}

	// Make sure the folder items was added to the apps grid.
	ui := uiauto.New(tconn)
	if err := ui.WaitUntilExists(UnnamedFolderFinder.First())(ctx); err != nil {
		return errors.Wrap(err, "failed to find the unnamed folder")
	}

	return nil
}

// DragIconAfterIcon moves an app list item at srcIndex to destIndex by
// drag-and-drop. srcIndex and destIndex are app list item view indices in
// the provided apps grid.
func DragIconAfterIcon(ctx context.Context, tconn *chrome.TestConn, srcIndex, destIndex int, appsGrid *nodewith.Finder) uiauto.Action {
	return func(ctx context.Context) error {
		if srcIndex == destIndex {
			return errors.Errorf("destIndex should be different from srcIndex: srcIndex is %d; destIndex is %d", srcIndex, destIndex)
		}

		ui := uiauto.New(tconn)
		itemListFinder := nodewith.ClassName(ExpandedItemsClass).Ancestor(appsGrid)
		srcBounds, err := ui.Location(ctx, itemListFinder.Nth(srcIndex))
		if err != nil {
			return errors.Wrap(err, "failed to get the source item bounds")
		}

		if err := mouse.Move(tconn, srcBounds.CenterPoint(), 0)(ctx); err != nil {
			return errors.Wrap(err, "failed to move to the start location")
		}
		if err := mouse.Press(tconn, mouse.LeftButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to press the button")
		}

		// Move a little bit first to trigger launcher-app-paging.
		if err := mouse.Move(tconn, srcBounds.CenterPoint().Add(coords.Point{X: 10, Y: 10}), time.Second)(ctx); err != nil {
			return errors.Wrap(err, "failed to move the mouse a little bit to trigger launcher-app-paging")
		}

		// Fetch the bounds of the item view at destIndex after launcher-app-paging completes.
		destBounds, err := ui.Location(ctx, itemListFinder.Nth(destIndex))
		if err != nil {
			return errors.Wrap(err, "failed to wait for the destination item bounds")
		}

		// Calculate the move target location. If srcIndex is smaller(bigger) than
		// destIndex, the source item should be dragged to the right(left) of the
		// destination item to trigger apps grid reorder.
		var x int
		if srcIndex < destIndex {
			x = destBounds.Right() + 5
		} else {
			x = destBounds.Left - 5
		}
		targetLocation := coords.NewPoint(x, destBounds.CenterY())

		if err := mouse.Move(tconn, targetLocation, 200*time.Millisecond)(ctx); err != nil {
			return errors.Wrap(err, "failed to move the mouse to drag the item to the target location")
		}

		if err := mouse.Release(tconn, mouse.LeftButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to release the mouse")
		}

		return nil
	}
}

// DragItemAfterItem drags an app list item returned by src node finder to a location after the app
// list item node returned by dest node finder.
func DragItemAfterItem(tconn *chrome.TestConn, src, dest *nodewith.Finder) uiauto.Action {
	const duration = time.Second
	return func(ctx context.Context) error {
		ui := uiauto.New(tconn)
		start, err := ui.Location(ctx, src)
		if err != nil {
			return errors.Wrap(err, "failed to get location for src icon")
		}

		if err := mouse.Move(tconn, start.CenterPoint(), 0)(ctx); err != nil {
			return errors.Wrap(err, "failed to move to the start location")
		}
		if err := mouse.Press(tconn, mouse.LeftButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to press the button")
		}

		// Move a little bit first to trigger launcher-app-paging.
		if err := mouse.Move(tconn, start.CenterPoint().Add(coords.Point{X: 10, Y: 10}), time.Second)(ctx); err != nil {
			return errors.Wrap(err, "failed to move the mouse")
		}

		// Get destination location during drag.
		end, err := ui.Location(ctx, dest)
		if err != nil {
			return errors.Wrap(err, "failed to get location for dst icon")
		}
		if err := mouse.Move(tconn, coords.NewPoint(end.Right()+5, end.CenterY()), 200*time.Millisecond)(ctx); err != nil {
			return errors.Wrap(err, "failed to move the mouse")
		}

		if err := mouse.Release(tconn, mouse.LeftButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to release the mouse")
		}

		return nil
	}
}

// DragIconToIcon drags from one icon to another icon.
func DragIconToIcon(tconn *chrome.TestConn, srcIndex, destIndex int) uiauto.Action {
	src := nodewith.ClassName(ExpandedItemsClass).Nth(srcIndex)
	dest := nodewith.ClassName(ExpandedItemsClass).Nth(destIndex)
	return DragItemToItem(tconn, src, dest)
}

// DragItemToItem drags from the src item to the dest item.
// We cannot use simple mouse.Drag because the default UI behavior is now that the icon location changes after you start a drag.
// This function will delay the calculation of the destination point until after mouse press.
func DragItemToItem(tconn *chrome.TestConn, src, dest *nodewith.Finder) uiauto.Action {
	return func(ctx context.Context) error {
		ui := uiauto.New(tconn)
		start, err := ui.Location(ctx, src)
		if err != nil {
			return errors.Wrap(err, "failed to get location for first icon")
		}
		if err := mouse.Move(tconn, start.CenterPoint(), 0)(ctx); err != nil {
			return errors.Wrap(err, "failed to move to the start location")
		}
		if err := mouse.Press(tconn, mouse.LeftButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to press the button")
		}

		// Move a little bit first to trigger launcher-app-paging.
		if err := mouse.Move(tconn, start.CenterPoint().Add(coords.Point{X: 10, Y: 10}), time.Second)(ctx); err != nil {
			return errors.Wrap(err, "failed to move the mouse")
		}

		// Get destination location during drag.
		end, err := ui.Location(ctx, dest)
		if err != nil {
			return errors.Wrap(err, "failed to get location for second icon")
		}
		if err := mouse.Move(tconn, end.CenterPoint(), 200*time.Millisecond)(ctx); err != nil {
			return errors.Wrap(err, "failed to move the mouse")
		}
		return mouse.Release(tconn, mouse.LeftButton)(ctx)
	}
}

// FetchItemIndicesByName returns the view indices of the items specified by app names in the given apps container.
func FetchItemIndicesByName(ctx context.Context, ui *uiauto.Context, appNames []string, appsContainer *nodewith.Finder) ([]int, error) {
	viewIndices := make([]int, len(appNames))
	var defaultIndex = -1
	for idx := range viewIndices {
		viewIndices[idx] = defaultIndex
	}

	nameIndexMapping := make(map[string]int)
	for index, name := range appNames {
		nameIndexMapping[name] = index
	}

	// Get the node information of all app list items.
	appListItems, err := ui.NodesInfo(ctx, nodewith.ClassName(ExpandedItemsClass).Ancestor(appsContainer))
	if err != nil {
		return viewIndices, errors.Wrap(err, "failed to get the node information of all app list items")
	}

	for viewIndex, item := range appListItems {
		nameIndex, found := nameIndexMapping[item.Name]
		if !found {
			continue
		}

		if viewIndices[nameIndex] != defaultIndex {
			return viewIndices, errors.Errorf("multiple items have the same name as %q", item.Name)
		}

		viewIndices[nameIndex] = viewIndex
	}

	for index, viewIndex := range viewIndices {
		if viewIndex == -1 {
			return viewIndices, errors.Errorf("failed to find the view index for the app %q", appNames[index])
		}
	}

	return viewIndices, nil
}

// RemoveIconFromFolder opens a folder and drags an icon out of the folder.
func RemoveIconFromFolder(tconn *chrome.TestConn, folderFinder *nodewith.Finder) uiauto.Action {
	return func(ctx context.Context) error {
		ui := uiauto.New(tconn)

		// Ensure the folder node is focused, to get its location to update.
		ui.FocusAndWait(folderFinder)(ctx)

		// Click to open the folder.
		if err := ui.LeftClick(folderFinder)(ctx); err != nil {
			return errors.Wrap(err, "failed to click the folder")
		}

		// Get the location for the first item in the folder.
		folderItems := nodewith.ClassName(ExpandedItemsClass).Ancestor(nodewith.ClassName("AppListFolderView"))
		start, err := ui.Location(ctx, folderItems.Nth(0))
		if err != nil {
			return errors.Wrap(err, "failed to get the location of the first folder item")
		}

		// Get a point outside of the folder view.
		folderView := nodewith.ClassName("AppListFolderView")
		folderViewLocation, err := ui.Location(ctx, folderView)
		if err != nil {
			return errors.Wrap(err, "failed to get folderViewLocation")
		}
		pointOutsideFolder := coords.NewPoint(folderViewLocation.Right()+50, folderViewLocation.CenterY()+50)

		// Drag the first folder item outside of the folder.
		mouse.Move(tconn, start.CenterPoint(), 0)(ctx)
		mouse.Press(tconn, mouse.LeftButton)(ctx)
		mouse.Move(tconn, pointOutsideFolder, time.Second)(ctx)

		// Get the location of the folder during the drag.
		folderLocation, err := ui.Location(ctx, folderFinder)
		if err != nil {
			return errors.Wrap(err, "failed to get location for folder")
		}

		// Drag app to the right of the folder.
		mouse.Move(tconn, folderLocation.CenterPoint().Add(coords.Point{X: (folderLocation.Width + 1) / 2, Y: 0}), time.Second)(ctx)

		// Release the mouse, ending the drag.
		mouse.Release(tconn, mouse.LeftButton)(ctx)

		// Make sure that the folder has closed.
		if err := ui.WaitUntilGone(folderView)(ctx); err != nil {
			errors.Wrap(err, "folderView is not gone")
		}

		return nil
	}
}

// AddItemsToFolder adds non-folder items to the specified folder.
// Assumes that the folder is on the current page.
// If the next available item to add is not on the current page, then the folder will be moved to the next page if app list is paginated.
// paginatedAppList indicates whether the app list supports pagination.
// If the folder is full, then the attempt will still be made to add the item to the folder, and no error will be returned for that case.
func AddItemsToFolder(ctx context.Context, tconn *chrome.TestConn, folder *nodewith.Finder, numItemsToAdd int, paginatedAppList bool) error {
	numItemsInFolder, err := GetFolderSize(ctx, tconn, folder)
	if err != nil {
		return errors.Wrap(err, "failed to get folder size")
	}
	// When productivity launcher is enabled, first row of items in the app list will be recent apps, which are not draggable, and cannot
	// be used for tests that create folder.
	itemToAddIndex, err := FirstNonRecentAppItem(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to count recent apps items preceding grid items")
	}

	targetTotalItems := numItemsInFolder + numItemsToAdd

	for numItemsInFolder < targetTotalItems {
		item := nodewith.ClassName(ExpandedItemsClass).Nth(itemToAddIndex)

		// If the apps grid is paginated, try moving the folder to the next page if the next item is not on the current page.
		if paginatedAppList {
			onPage, err := IsItemOnCurrentPage(ctx, tconn, item)
			if err != nil {
				return errors.Wrap(err, "failed to check if the item is on the current page")
			}
			if !onPage {
				if err := DragIconAtIndexToNextPage(tconn, 0)(ctx); err != nil {
					return errors.Wrap(err, "failed to drag icon to the next page")
				}
			}
		}

		// Cannot add folders to a folder so skip over folder items.
		isFolder, err := IsFolderItem(ctx, tconn, item)
		if err != nil {
			return errors.Wrap(err, "failed to check if item is a folder")
		}
		if isFolder {
			itemToAddIndex++
			continue
		}

		// Add the item to the folder.
		if err := DragItemToItem(tconn, item, folder)(ctx); err != nil {
			return errors.Wrap(err, "failed to drag icon into a folder")
		}
		numItemsInFolder++
	}
	return nil
}

// DragIconAtIndexToNextPage drags an icon which has itemIndex in the app list to the next page of the app list.
func DragIconAtIndexToNextPage(tconn *chrome.TestConn, itemIndex int) uiauto.Action {
	return DragIconToNextPage(tconn, nodewith.ClassName(ExpandedItemsClass).Nth(itemIndex))
}

// DragIconToNextPage drags an icon to the next page of the app list.
func DragIconToNextPage(tconn *chrome.TestConn, item *nodewith.Finder) uiauto.Action {
	return DragIconToNeighbourPage(tconn, item, true /*next*/)
}

// DragIconToNeighbourPage drags an icon to the next, or previous page of the app list.
// next indicates whether the icon should be dragged to the next page.
func DragIconToNeighbourPage(tconn *chrome.TestConn, item *nodewith.Finder, next bool) uiauto.Action {
	return func(ctx context.Context) error {
		ui := uiauto.New(tconn)
		// Move and press the mouse on the drag item.
		start, err := ui.Location(ctx, item)
		if err != nil {
			return errors.Wrap(err, "failed to get location for drag item")
		}
		if err := mouse.Move(tconn, start.CenterPoint(), 0)(ctx); err != nil {
			return errors.Wrap(err, "failed to move to the start location")
		}
		if err := mouse.Press(tconn, mouse.LeftButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to press the icon")
		}

		// Move a little bit first to trigger launcher-app-paging.
		if err := mouse.Move(tconn, start.CenterPoint().Add(coords.Point{X: 20, Y: 20}), time.Second)(ctx); err != nil {
			return errors.Wrap(err, "failed to move the mouse")
		}

		// Get destination location during drag.
		end, err := ui.Location(ctx, nodewith.ClassName("AppsGridView"))
		if err != nil {
			return errors.Wrap(err, "failed to get location for AppsGridView")
		}

		var endPoint coords.Point
		if next {
			endPoint = coords.NewPoint(end.CenterPoint().X, end.Bottom())
		} else {
			endPoint = coords.NewPoint(end.CenterPoint().X, end.Top)
		}

		// Drag icon to the bottom of the AppsGridView.
		if err := mouse.Move(tconn, endPoint, 200*time.Millisecond)(ctx); err != nil {
			return errors.Wrap(err, "failed to move the mouse")
		}

		// Move a little bit and wait for page change.
		pageSwitcher := nodewith.ClassName("PageSwitcher")
		if err := ui.WaitForEvent(pageSwitcher, event.Alert, mouse.Move(tconn, endPoint.Add(coords.Point{X: 1, Y: 0}), time.Second))(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for page change event")
		}

		// Move icon to near the top left of the AppsGridView and release to place the icon at the start of the page.
		if err := mouse.Move(tconn, end.TopLeft().Add(coords.Point{X: 10, Y: 10}), 200*time.Millisecond)(ctx); err != nil {
			return errors.Wrap(err, "failed to move to near the top left of the AppsGridView")
		}
		return mouse.Release(tconn, mouse.LeftButton)(ctx)
	}
}

// GetFolderSize opens the given folder, gets the number of apps inside, and then closes the folder.
// This assumes that there is no folder open and may not work if a folder is already opened.
func GetFolderSize(ctx context.Context, tconn *chrome.TestConn, folder *nodewith.Finder) (int, error) {
	ui := uiauto.New(tconn)
	// Click to open the folder.
	if err := ui.LeftClick(folder)(ctx); err != nil {
		return 0, errors.Wrap(err, "failed to click the folder")
	}

	// Get |folderItemsInfo| which is used to get the size of the folder.
	folderView := nodewith.ClassName("AppListFolderView")
	folderItems := nodewith.ClassName("AppListItemView").Ancestor(folderView)
	folderItemsInfo, err := ui.NodesInfo(ctx, folderItems)
	if err != nil {
		return 0, errors.Wrap(err, "failed to find folderItemsInfo")
	}

	if err := CloseFolderView(ctx, tconn); err != nil {
		return 0, errors.Wrap(err, "failed to close the folder")
	}

	return len(folderItemsInfo), nil
}

// IsItemOnCurrentPage will return whether the item is shown on the current launcher page.
// Assumes that there is no folder open, and may not work if a folder is opened.
func IsItemOnCurrentPage(ctx context.Context, tconn *chrome.TestConn, item *nodewith.Finder) (bool, error) {
	ui := uiauto.New(tconn)
	ui.WaitForLocation(item)(ctx)
	info, err := ui.Info(ctx, item)
	if err != nil {
		return false, errors.Wrap(err, "failed to get item info")
	}

	return !info.State[state.Offscreen], nil
}

// ScrollBubbleLauncherDuringItemDragUntilItemVisible moves the pointer to top or the bottom of the app list bubble view, which is expected to scroll the view.
// It keeps the pointer in scrolling position until the item returned by targetItem becomes visible.
// up indicates whether the bubble app list view apps page should be scrolled up, or down.
// NOTE: This is intended to be used during app list item drag, otherwise just hovering the pointer over bubble bounds will not scroll the app list.
// This may be flaky if targetItem is not visible in fully scrolled state - in that case polling interval may miss the period when the view is visible.
func ScrollBubbleLauncherDuringItemDragUntilItemVisible(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, targetItem *nodewith.Finder, up bool) error {
	// Move the icon to the bottom of the bubble launcher - this should trigger scroll within the app list bubble.
	bubbleView := nodewith.ClassName("AppListBubbleView")
	bubbleViewLocation, err := ui.Location(ctx, bubbleView)
	if err != nil {
		return errors.Wrap(err, "failed to get bubble view bounds")
	}

	var scrollPoint coords.Point
	if up {
		scrollPoint = coords.NewPoint(bubbleViewLocation.CenterX(), bubbleViewLocation.Top)
	} else {
		scrollPoint = bubbleViewLocation.BottomCenter()
	}
	if err := mouse.Move(tconn, scrollPoint, 200)(ctx); err != nil {
		return errors.Wrap(err, "failed to move to the start location")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		targetItemInfo, err := ui.Info(ctx, targetItem)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get target item info"))
		}

		if targetItemInfo.State[state.Offscreen] {
			return errors.New("target item not in viewport")
		}
		return nil

	}, &testing.PollOptions{Timeout: 30 * time.Second, Interval: time.Second}); err != nil {
		return errors.Wrap(err, "Target item did not become visible")
	}

	if err := mouse.Move(tconn, bubbleViewLocation.CenterPoint(), 200)(ctx); err != nil {
		return errors.Wrap(err, "failed to move to the grid center")
	}

	return nil
}

// DragItemInBubbleLauncherWithScrolling performs app list item drag in bubble launcher where the launcher is expected to scroll during the drag.
// dragItem is the item that will be dragged, and is expected to be visible when the method gets called.
// targetItem is an item used to determine the drag item drop spot - the drag item will be dropped just right of the targetItem.
// up describes the direction the app list should be scrolled for the targetItem to become visible
// NOTE: targetItem should be an item that's visible when the app list is fully scrolled.
func DragItemInBubbleLauncherWithScrolling(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, dragItem, targetItem *nodewith.Finder, up bool) error {
	// Start item drag.
	start, err := ui.Location(ctx, dragItem)
	if err != nil {
		return errors.Wrap(err, "failed to get location for drag icon")
	}

	if err := mouse.Move(tconn, start.CenterPoint(), 0)(ctx); err != nil {
		return errors.Wrap(err, "failed to move to the start location")
	}
	if err := mouse.Press(tconn, mouse.LeftButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to press the button")
	}

	if err := ScrollBubbleLauncherDuringItemDragUntilItemVisible(ctx, tconn, ui, targetItem, up); err != nil {
		return errors.Wrap(err, "bubble launcher scroll failed")
	}

	// Get drag drop location.
	end, err := ui.Location(ctx, targetItem)
	if err != nil {
		return errors.Wrap(err, "failed to get location for drag drop point")
	}
	if err := mouse.Move(tconn, coords.NewPoint(end.Right()+5, end.CenterY()), 200)(ctx); err != nil {
		return errors.Wrap(err, "failed to move to drop location")
	}
	if err := mouse.Release(tconn, mouse.LeftButton)(ctx); err != nil {
		return errors.Wrap(err, "mouse release failed")
	}

	return nil
}

// IsFolderItem returns whether the item is a folder. Assumes that there is no folder open.
func IsFolderItem(ctx context.Context, tconn *chrome.TestConn, item *nodewith.Finder) (bool, error) {
	ui := uiauto.New(tconn)
	i, err := ui.Info(ctx, item)
	if err != nil {
		return false, errors.Wrap(err, "failed to get location for the item")
	}

	// If the name of the item begins with Folder, then it is considered a folder.
	match, err := regexp.MatchString("^Folder ", i.Name)
	if err != nil {
		return false, errors.Wrap(err, "failed to match name with regexp")
	}
	return match, nil
}

// indexNamePair associates one app list item's visual index with its item name.
type indexNamePair struct {
	viewIndex int
	appName   string
}

type byViewIndex []indexNamePair

func (data byViewIndex) Len() int           { return len(data) }
func (data byViewIndex) Swap(i, j int)      { data[i], data[j] = data[j], data[i] }
func (data byViewIndex) Less(i, j int) bool { return data[i].viewIndex < data[j].viewIndex }
func (data byViewIndex) NameList() []string {
	names := make([]string, data.Len())
	for index, pair := range data {
		names[index] = pair.appName
	}
	return names
}

// VerifyFakeAppsOrdered checks that the visual order of the app list items specified by app names is consistent with namesInOrder.
// appsGrid specifies the apps grid on which item order is verified. If wait is true, wait until the specified app list items show before
// getting their view indices.
func VerifyFakeAppsOrdered(ctx context.Context, ui *uiauto.Context, appsGrid *nodewith.Finder, namesInOrder []string, wait bool) error {
	if wait {
		for _, name := range namesInOrder {
			if err := ui.WaitUntilExists(nodewith.ClassName(ExpandedItemsClass).Name(name).Ancestor(appsGrid))(ctx); err != nil {
				return errors.Wrapf(err, "failed to find app %q after sort", name)
			}
		}
	}

	viewIndices, err := FetchItemIndicesByName(ctx, ui, namesInOrder, appsGrid)
	if err != nil {
		return errors.Wrap(err, "failed to get view indices of fake apps")
	}

	if len(viewIndices) != len(namesInOrder) {
		return errors.Errorf("unexpected view indices count: got %d, expecting %d", len(namesInOrder), len(viewIndices))
	}

	for index := 1; index < len(viewIndices); index++ {
		if viewIndices[index] > viewIndices[index-1] {
			// viewIndices still keep increasing order. It is expected.
			continue
		}

		// The actual view order is unexpected. The code below calculates item names under the actual order to provide more informative error message.
		actualNames := make([]indexNamePair, len(viewIndices))
		for indexInArray, viewIndex := range viewIndices {
			actualNames[indexInArray] = indexNamePair{viewIndex: viewIndex, appName: namesInOrder[indexInArray]}
		}

		data := byViewIndex(actualNames)
		sort.Sort(data)
		return errors.Errorf("unexpected fake app order: got %v, expecting %v", data.NameList(), namesInOrder)
	}

	return nil
}

// ReadImageBytesFromFilePath reads a PNG image from the specified file path.
func ReadImageBytesFromFilePath(filePath string) ([]byte, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	image, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	buf := &bytes.Buffer{}
	if err := png.Encode(buf, image); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// OpenProductivityLauncher performs the correct action to show the launcher depending on the tabletMode state.
// tabletMode when true, drags upwards from the hotseat to show the home screen. Otherwise, it opens the launcher by triggering the search accelerator.
func OpenProductivityLauncher(ctx context.Context, tconn *chrome.TestConn, tabletMode bool) error {
	if tabletMode {
		touchScreen, err := input.Touchscreen(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get the touch screen")
		}
		defer touchScreen.Close()

		stw, err := touchScreen.NewSingleTouchWriter()
		if err != nil {
			return errors.Wrap(err, "failed to get the single touch event writer")
		}
		defer stw.Close()

		// Make sure the shelf bounds is stable before dragging.
		if err := ash.WaitForStableShelfBounds(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to wait for stable shelf bounds")
		}
		if err := ash.DragToShowHomescreen(ctx, touchScreen.Width(), touchScreen.Height(), stw, tconn); err != nil {
			return errors.Wrap(err, "failed to show homescreen")
		}
	} else {
		if err := OpenBubbleLauncher(tconn)(ctx); err != nil {
			return errors.Wrap(err, "failed to open bubble launcher")
		}
	}
	return nil
}

// DismissSortNudgeIfExists will get rid of the sort nudge that appears the first time the productivity launcher is open.
// This method will click on the OK button on the sort nudge
func DismissSortNudgeIfExists(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)
	sortNudge := nodewith.Name("Sort your apps by name or color")
	sortNudgeFound, err := ui.IsNodeFound(ctx, sortNudge)
	if err != nil {
		return errors.Wrap(err, "failed to search for sort nudge")
	}

	if sortNudgeFound {
		dismissButton := nodewith.Name("OK").ClassName("PillButton")
		if err := uiauto.Combine("Click on the dismiss button",
			ui.WaitUntilExists(dismissButton),
			ui.LeftClick(dismissButton),
			ui.WaitUntilGone(sortNudge),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to click on the OK button")
		}
	}
	return nil
}

// DismissPrivacyNotice waits for the continue section privacy notice to appear,
// then dismisses it by clicking the "OK" button.
func DismissPrivacyNotice(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)
	continueSection := nodewith.ClassName("ContinueSectionView")
	privacyNoticeButton := nodewith.Ancestor(continueSection).ClassName("PillButton").Name("OK")
	if err := uiauto.Combine("Click on privacy notice OK button",
		ui.WaitUntilExists(privacyNoticeButton),
		ui.LeftClick(privacyNoticeButton),
		ui.WaitUntilGone(privacyNoticeButton),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to click OK button")
	}
	return nil
}

// UninstallsAppUsingContextMenu uninstalls an app using the context menu in the apps grid. This method should be called with an open apps grid.
func UninstallsAppUsingContextMenu(ctx context.Context, tconn *chrome.TestConn, app *nodewith.Finder) error {
	ui := uiauto.New(tconn)
	confirmUninstall := nodewith.Name("Uninstall").Role(role.Button)
	uninstallOption := nodewith.Name("Uninstall").ClassName("MenuItemView")
	if err := uiauto.Combine("Uninstall app",
		ui.Exists(app),
		ui.RightClick(app),
		ui.WaitUntilExists(uninstallOption),
		ui.LeftClick(uninstallOption),
		ui.WaitUntilExists(confirmUninstall),
		// Uninstall dialog has a heuristic to determine
		// unintended clicks, which includes ignoring events
		// that happen soon after the dialog is shown. Add a
		// small delay before clicking the uninstall button.
		action.Sleep(1*time.Second),
		ui.LeftClick(confirmUninstall),
		ui.WaitUntilGone(confirmUninstall),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to remove the app on recent apps")
	}
	return nil
}

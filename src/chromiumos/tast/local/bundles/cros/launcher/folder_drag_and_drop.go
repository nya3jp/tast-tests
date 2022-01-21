// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FolderDragAndDrop,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Renaming Folder In Launcher",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
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
			Fixture: "chromeLoggedInWith100FakeApps",
		}, {
			Name:              "productivity_launcher_tablet_mode",
			Val:               launcher.TestCase{ProductivityLauncher: true, TabletMode: true},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
			Fixture:           "chromeLoggedInWith100FakeAppsProductivityLauncher",
		}, {
			Name:              "tablet_mode",
			Val:               launcher.TestCase{ProductivityLauncher: false, TabletMode: true},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
			Fixture:           "chromeLoggedInWith100FakeApps",
		}},
	})
}

// FolderDragAndDrop runs a test that drags app list folders within the launcher UI.
func FolderDragAndDrop(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	testCase := s.Param().(launcher.TestCase)
	tabletMode := testCase.TabletMode
	productivityLauncher := testCase.ProductivityLauncher

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to ensure clamshell/tablet mode: ", err)
	}
	defer cleanup(ctx)

	if !tabletMode {
		if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
			s.Fatal("Launcher not closed after transition to clamshell mode: ", err)
		}
	}

	ui := uiauto.New(tconn)

	usingBubbleLauncher := productivityLauncher && !tabletMode
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

	if err := launcher.WaitForStableNumberOfApps(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for item count in app list to stabilize: ", err)
	}

	if !usingBubbleLauncher {
		pageSwitcher := nodewith.ClassName("Button").Ancestor(nodewith.ClassName("PageSwitcher"))
		if err := ui.WithTimeout(5 * time.Second).LeftClick(pageSwitcher.Nth(0))(ctx); err != nil {
			s.Fatal("Failed to switch launcher to first page: ", err)
		}
	}

	// Create a folder that will be dragged around in the test.
	if err := launcher.CreateFolder(ctx, tconn, productivityLauncher); err != nil {
		s.Fatal("Failed to create a folder item: ", err)
	}

	folderInfo, err := ui.Info(ctx, launcher.UnnamedFolderFinder)
	if err != nil {
		s.Fatal("Failed to get folder info: ", err)
	}
	folderName := folderInfo.Name

	// For paged launcher, start by dragging the folder to the second page - when productivity launcher is disabled,
	// the first page contains only default apps, and may not have enough items to test drag within the current page.
	if !usingBubbleLauncher {
		if err := launcher.DragIconToNextPage(tconn, launcher.UnnamedFolderFinder)(ctx); err != nil {
			s.Fatal("Failed to drag folder to the next page: ", err)
		}

		// Verify that the first item in the grid is offscreen.
		firstItemInfo, err := ui.Info(ctx, nodewith.ClassName(launcher.ExpandedItemsClass).First())
		if err != nil {
			s.Fatal("Failed to get first item info after drag to second page")
		}

		if !firstItemInfo.State[state.Offscreen] {
			s.Fatal("First item unexpectedly on sreen after drag to second page")
		}
	}

	firstVisibleIndex, err := launcher.FirstNonRecentAppItem(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to count recent apps items: ", err)
	}

	firstVisibleIndex, err = launcher.IndexOfFirstVisibleItem(ctx, tconn, firstVisibleIndex)
	if err != nil {
		s.Fatal("Failed to find visible item: ", err)
	}

	firstVisibleItemInfo, err := ui.Info(ctx, nodewith.ClassName(launcher.ExpandedItemsClass).Nth(firstVisibleIndex))
	if err != nil {
		s.Fatal("Failed to get first visible item info: ", err)
	}

	// Verify that the folder item is the first visible non-recent app item on the page.
	if firstVisibleItemInfo.Name != folderName {
		s.Fatalf("Folder is not the first visible item: %s", firstVisibleItemInfo.Name)
	}

	// Test folder drag within the current page.
	targetIndex := firstVisibleIndex + 6
	targetItem := nodewith.ClassName(launcher.ExpandedItemsClass).Nth(targetIndex)
	if err := launcher.DragItemAfterItem(tconn, launcher.UnnamedFolderFinder, targetItem)(ctx); err != nil {
		s.Fatal("Failed to drag folder within the same page: ", err)
	}

	// Verify the new folder position.
	targetItemInfo, err := ui.Info(ctx, targetItem)
	if err != nil {
		s.Fatal("Failed to get target item info: ", err)
	}
	if targetItemInfo.Name != folderName {
		s.Fatalf("Item at target name (%s) not the folder item (%s)", targetItemInfo.Name, folderName)
	}

	// Verify the folder is still onscreen.
	folderInfo, err = ui.Info(ctx, launcher.UnnamedFolderFinder)
	if err != nil {
		s.Fatal("Failed to get updated folder info within the current page: ", err)
	}
	if folderInfo.State[state.Offscreen] {
		s.Fatal("First item unexpectedly off sreen after drag within current page")
	}

	// If launcher is paginated, test dragging the folder to the previous page.
	if !usingBubbleLauncher {
		if err := launcher.DragIconToNeighbourPage(tconn, launcher.UnnamedFolderFinder, false /*next*/)(ctx); err != nil {
			s.Fatal("Failed to drag folder to previous page: ", err)
		}

		// Verify that the first item is on the current page.
		firstItemVisible, err := launcher.IsItemOnCurrentPage(ctx, tconn, nodewith.ClassName(launcher.ExpandedItemsClass).First())
		if err != nil {
			s.Fatal("Failed to query first item visibility after drag to first page: ", err)
		}
		if !firstItemVisible {
			s.Fatal("First item unexpectedly off sreen after drag to first page")
		}

		// Verify the folder is still onscreen
		folderVisible, err := launcher.IsItemOnCurrentPage(ctx, tconn, launcher.UnnamedFolderFinder)
		if err != nil {
			s.Fatal("Failed to query folder visibility after drag to first page: ", err)
		}
		if !folderVisible {
			s.Fatal("First item unexpectedly off sreen after drag to first page")
		}
	}

	// For bubble launcher, that apps grid can be scrolled by dragging the folder.
	if usingBubbleLauncher {
		if err := dragFolderToScrollableContainerBottom(ctx, tconn, ui); err != nil {
			s.Fatal("Failed to drag the first icon to bottom of scrollable container: ", err)
		}

		if err := dragFolderToScrollableContainerTop(ctx, tconn, ui); err != nil {
			s.Fatal("Failed to drag the last item to top of scrollable container: ", err)
		}

		// Workaround for https://crbug.com/1290569 - item locations within a scroll view (which is used
		// by bubble launcher) may be incorrect if the container node location gets updated when scroll
		// offset is non-zero. This happens when opening and closing a folder, which is an action that
		// the test is about to perform - reset the bubble scroll position so app list item node locations
		// can be trusted.
		scrollView := nodewith.ClassName("ScrollView").Ancestor(nodewith.ClassName("AppListBubbleView"))
		if err := ui.ResetScrollOffset(scrollView)(ctx); err != nil {
			s.Fatal("Failed to reset scroll offset: ", err)
		}
	}

	// Clean up the folder used during the test.
	if err := launcher.RemoveIconFromFolder(tconn)(ctx); err != nil {
		s.Fatal("Failed to drag out the icon from folder: ", err)
	}

	if productivityLauncher {
		if err := launcher.RemoveIconFromFolder(tconn)(ctx); err != nil {
			s.Fatal("Failed to drag out the icon from single-item folder: ", err)
		}
	}

	// Make sure that the folder has closed.
	if err := ui.WaitUntilGone(launcher.UnnamedFolderFinder)(ctx); err != nil {
		errors.Wrap(err, "folder item is not gone")
	}
}

// dragFolderToScrollableContainerBottom drags a folder item in the scrollable apps grid view to the last available slot in the view.
func dragFolderToScrollableContainerBottom(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context) error {
	dragItem := launcher.UnnamedFolderFinder
	dragItemInfo, err := ui.Info(ctx, dragItem)
	if err != nil {
		return errors.Wrap(err, "failed to get drag item info")
	}
	dragItemName := dragItemInfo.Name

	// Find the last available slot.
	scrollableGridItems := nodewith.ClassName(launcher.ExpandedItemsClass).Ancestor(nodewith.ClassName("ScrollableAppsGridView"))
	allItemsInfo, err := ui.NodesInfo(ctx, scrollableGridItems)
	if err != nil {
		return errors.Wrap(err, "failed to get list of grid items")
	}
	itemCount := len(allItemsInfo)
	targetItem := scrollableGridItems.Nth(itemCount - 1)

	if err := launcher.DragItemInBubbleLauncherWithScrolling(ctx, tconn, ui, dragItem, targetItem, false /*up*/); err != nil {
		return errors.Wrap(err, "bubble launcher scroll failed")
	}

	// Drag item should have been moved right of the last item in the scrollable grid.
	lastItemInfo, err := ui.Info(ctx, scrollableGridItems.Nth(itemCount-1))
	if err != nil {
		return errors.Wrap(err, "failed to get second app item info")
	}

	if dragItemName != lastItemInfo.Name {
		return errors.Wrapf(err, "Last item %s is not the drag item %s", lastItemInfo.Name, dragItemInfo.Name)
	}

	dragItemInfo, err = ui.Info(ctx, dragItem)
	if err != nil {
		return errors.Wrap(err, "failed to get drag item info")
	}
	if dragItemInfo.State[state.Offscreen] {
		return errors.New("drag item offscreen after drag with scroll")
	}

	return nil
}

// dragFolderToScrollableContainerTop drags a folder item in the scrollable apps grid view to the slot after the first item.
func dragFolderToScrollableContainerTop(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context) error {
	dragItem := launcher.UnnamedFolderFinder
	dragItemInfo, err := ui.Info(ctx, dragItem)
	if err != nil {
		return errors.Wrap(err, "failed to get drag item info")
	}
	dragItemName := dragItemInfo.Name

	scrollableGridItems := nodewith.ClassName(launcher.ExpandedItemsClass).Ancestor(nodewith.ClassName("ScrollableAppsGridView"))
	targetItem := scrollableGridItems.First()

	if err := launcher.DragItemInBubbleLauncherWithScrolling(ctx, tconn, ui, dragItem, targetItem, true /*up*/); err != nil {
		return errors.Wrap(err, "bubble launcher scroll failed")
	}

	// Drag item should have been moved right of the first item in the scrollable grid.
	secondItemInfo, err := ui.Info(ctx, scrollableGridItems.Nth(1))
	if err != nil {
		return errors.Wrap(err, "failed to get second app item info")
	}

	if dragItemName != secondItemInfo.Name {
		return errors.Wrapf(err, "Last item %s is not the drag item %s", secondItemInfo.Name, dragItemName)
	}

	dragItemInfo, err = ui.Info(ctx, dragItem)
	if err != nil {
		return errors.Wrap(err, "failed to get drag item info")
	}
	if dragItemInfo.State[state.Offscreen] {
		return errors.New("drag item offscreen after drag with scroll")
	}

	return nil
}

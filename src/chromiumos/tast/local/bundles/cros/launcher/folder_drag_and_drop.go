// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FolderDragAndDrop,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Launcher Folder Item Drag and Drop",
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

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	testCase := s.Param().(launcher.TestCase)
	tabletMode := testCase.TabletMode
	productivityLauncher := testCase.ProductivityLauncher

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

	ui := uiauto.New(tconn)
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

	folderName := fmt.Sprintf("FolderDnD %t %t", productivityLauncher, tabletMode)
	if err := launcher.RenameFolder(tconn, kb, launcher.UnnamedFolderFinder.First(), folderName)(ctx); err != nil {
		s.Fatal("Failed to rename test folder")
	}

	folderItemName := "Folder " + folderName
	folderFinder := nodewith.ClassName(launcher.ExpandedItemsClass).Name(folderItemName)
	if err := ui.Exists(folderFinder)(ctx); err != nil {
		s.Fatal("Unable to find test folder: ", err)
	}

	// For paged launcher, start by dragging the folder to the second page - when productivity launcher is disabled,
	// the first page contains only default apps, and may not have enough items to test drag within the current page.
	if !usingBubbleLauncher {
		if err := launcher.DragIconToNextPage(tconn, folderFinder)(ctx); err != nil {
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
	if firstVisibleItemInfo.Name != folderItemName {
		s.Fatalf("Folder is not the first visible item: %s", firstVisibleItemInfo.Name)
	}

	// Test folder drag within the current page.
	targetIndex := firstVisibleIndex + 6
	targetItem := nodewith.ClassName(launcher.ExpandedItemsClass).Nth(targetIndex)
	if err := launcher.DragItemAfterItem(tconn, folderFinder, targetItem)(ctx); err != nil {
		s.Fatal("Failed to drag folder within the same page: ", err)
	}

	// Verify the new folder position.
	targetItemInfo, err := ui.Info(ctx, targetItem)
	if err != nil {
		s.Fatal("Failed to get target item info: ", err)
	}
	if targetItemInfo.Name != folderItemName {
		s.Fatalf("Item at target name (%s) not the folder item (%s)", targetItemInfo.Name, folderItemName)
	}

	// Verify the folder is still onscreen.
	folderInfo, err := ui.Info(ctx, folderFinder)
	if err != nil {
		s.Fatal("Failed to get updated folder info within the current page: ", err)
	}
	if folderInfo.State[state.Offscreen] {
		s.Fatal("First item unexpectedly off sreen after drag within current page")
	}

	// If launcher is paginated, test dragging the folder to the previous page.
	if !usingBubbleLauncher {
		if err := launcher.DragIconToNeighbourPage(tconn, folderFinder, false /*next*/)(ctx); err != nil {
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
		folderVisible, err := launcher.IsItemOnCurrentPage(ctx, tconn, folderFinder)
		if err != nil {
			s.Fatal("Failed to query folder visibility after drag to first page: ", err)
		}
		if !folderVisible {
			s.Fatal("First item unexpectedly off sreen after drag to first page")
		}
	}

	// For bubble launcher, that apps grid can be scrolled by dragging the folder.
	if usingBubbleLauncher {
		if err := dragFolderAndScrollContainer(ctx, tconn, ui, folderFinder, false /*up*/); err != nil {
			s.Fatal("Failed to drag the first icon to bottom of scrollable container: ", err)
		}

		if err := dragFolderAndScrollContainer(ctx, tconn, ui, folderFinder, true /*up*/); err != nil {
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
	if err := launcher.RemoveIconFromFolder(tconn, folderFinder)(ctx); err != nil {
		s.Fatal("Failed to drag out the icon from folder: ", err)
	}

	if productivityLauncher {
		if err := launcher.RemoveIconFromFolder(tconn, folderFinder)(ctx); err != nil {
			s.Fatal("Failed to drag out the icon from single-item folder: ", err)
		}
	}

	// Make sure that the folder has closed.
	if err := ui.WaitUntilGone(folderFinder)(ctx); err != nil {
		errors.Wrap(err, "folder item is not gone")
	}
}

// dragFolderAndScrollContainer drags a folder item in the scrollable apps grid view and scrolls is as need.
// If up is true, the container will be scrolled upwards, and the item will be dropped in the second slot in the apps grid.
// If up is false, the container will be scrolled downwards, and the item will be dropped in the last slot in the apps grid.
// Verifies the item at drop location matches the dragged item, and that it's onscreen.
func dragFolderAndScrollContainer(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, dragItem *nodewith.Finder, up bool) error {
	dragItemInfo, err := ui.Info(ctx, dragItem)
	if err != nil {
		return errors.Wrap(err, "failed to get drag item info")
	}
	dragItemName := dragItemInfo.Name

	scrollableGridItems := nodewith.ClassName(launcher.ExpandedItemsClass).Ancestor(nodewith.ClassName("ScrollableAppsGridView"))
	allItemsInfo, err := ui.NodesInfo(ctx, scrollableGridItems)
	if err != nil {
		return errors.Wrap(err, "failed to get list of grid items")
	}
	itemCount := len(allItemsInfo)

	// Index of the item to be used as a refecence for dropping the dragged view - the view will be dropped right of the item.
	var targetIndex int
	// The expected index in the apps grid in which the dragged item will be dropped.
	var dropIndex int
	if up {
		targetIndex = 0
		dropIndex = 1
	} else {
		targetIndex = itemCount - 1
		dropIndex = itemCount - 1
	}

	targetItem := scrollableGridItems.Nth(targetIndex)
	if err := launcher.DragItemInBubbleLauncherWithScrolling(ctx, tconn, ui, dragItem, targetItem, up); err != nil {
		return errors.Wrap(err, "bubble launcher scroll failed")
	}

	// Drag item should have been moved right of the first item in the scrollable grid.
	dropItemFinder := scrollableGridItems.Nth(dropIndex)
	dropItemInfo, err := ui.Info(ctx, dropItemFinder)
	if err != nil {
		return errors.Wrap(err, "failed to get second app item info")
	}

	if dragItemName != dropItemInfo.Name {
		return errors.Wrapf(err, "Last item %s is not the drag item %s", dropItemInfo.Name, dragItemName)
	}

	if dropItemInfo.State[state.Offscreen] {
		return errors.New("drag item offscreen after drag with scroll")
	}

	return nil
}

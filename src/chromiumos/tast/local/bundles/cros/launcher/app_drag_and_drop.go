// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AppDragAndDrop,
		Desc: "Test the functionality of dragging and dropping on app icons",
		Contacts: []string{
			"cash.hsu@cienet.com",
			"cienet-development@googlegroups.com",
			"tbarzic@chromium.org",
			"chromeos-sw-engprod@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
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

// AppDragAndDrop tests the functionality of dragging and dropping on app icons.
func AppDragAndDrop(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	testCase := s.Param().(launcher.TestCase)
	tabletMode := testCase.TabletMode

	cleanup, err := launcher.SetUpLauncherTest(
		ctx, tconn, tabletMode, true /*productivityLauncher*/, true /*stabilizeAppCount*/)
	if err != nil {
		s.Fatal("Failed to set up launcher test case: ", err)
	}
	defer cleanup(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	// Each subtest requires at least 3 items on the current page - the
	// first page may have a (default) page break after several
	// default apps, and depending on the device may not have enough apps to
	// satisfy this requirement.
	// To work around this, start the test on the second launcher page.
	// startPage defines which page starts testing.
	startPage := 2
	ui := uiauto.New(tconn)
	usingBubbleLauncher := !tabletMode
	if !usingBubbleLauncher {
		if err := switchToPage(ui, startPage)(ctx); err != nil {
			s.Fatal("Failed to switch to second page for test: ", err)
		}
	}

	firstItem, err := launcher.FirstNonRecentAppItem(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to count recent apps items: ", err)
	}

	if !usingBubbleLauncher {
		firstItem, err = getFirstItemOnCurrentPage(ctx, tconn, firstItem)
		if err != nil {
			s.Fatal("Failed to get the first item on the current page: ", err)
		}
	}

	if err := dragIconToIcon(ctx, tconn, ui, tabletMode, firstItem); err != nil {
		s.Fatal("Failed to drag the first icon to the second icon: ", err)
	}

	if !usingBubbleLauncher {
		if err := dragIconToNextPage(ctx, tconn, ui, firstItem, startPage); err != nil {
			s.Fatal("Failed to drag the first icon to next page: ", err)
		}
	} else {
		dropIndex, err := dragFirstIconToScrollableContainerBottom(ctx, tconn, ui)
		if err != nil {
			s.Fatal("Failed to drag the first icon to bottom of scrollable container: ", err)
		}

		if err := dragIconToScrollableContainerTop(ctx, tconn, ui, dropIndex); err != nil {
			s.Fatal("Failed to drag the last item to top of scrollable container: ", err)
		}
	}
}

// dragIconToIcon drags an app list item at firstItem index in the current app list page to an item at index firstItem + 2, creating a folder.
// It then removes all items from the folder, and verifies the original item gets dropped into a different location.
// tabletMode is used to determine which apps grid view to search for app icons.
func dragIconToIcon(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, tabletMode bool, firstItem int) error {
	srcInfo, err := ui.Info(ctx, nodewith.HasClass(launcher.ExpandedItemsClass).Nth(firstItem))
	if err != nil {
		return errors.Wrap(err, "failed to get information of first icon")
	}
	// Because the launcher views are cached on first show, both apps grid views
	// may exist if chrome doesn't restart between two launcher tests.
	var ancestorNode *nodewith.Finder
	if tabletMode {
		ancestorNode = nodewith.ClassName(launcher.PagedAppsGridViewClass)
	} else {
		ancestorNode = nodewith.ClassName(launcher.BubbleAppsGridViewClass)
	}
	src := launcher.AppItemViewFinder(srcInfo.Name).Ancestor(ancestorNode)

	locBefore, err := ui.Location(ctx, src)
	if err != nil {
		return errors.Wrap(err, "failed to get location of icon before dragging")
	}

	// Because launcher.DragIconToIcon can't drag the icon to the middle of two adjacent icons,
	// and the srcIcon and destIcon will be merged into a folder.
	// Use launcher.DragIconToIcon and launcher.RemoveIconFromFolder to change the position of the icon while avoiding merging into a folder.
	if err := launcher.DragIconToIcon(tconn, firstItem, firstItem+2)(ctx); err != nil {
		return errors.Wrap(err, "failed to drag the first icon to the third icon")
	}

	// Folders get automatically opened after getting created by dragging - make sure the created folder gets closed.
	if err := launcher.CloseFolderView(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to close the folder")
	}

	if err := launcher.RemoveIconFromFolder(tconn, launcher.UnnamedFolderFinder)(ctx); err != nil {
		return errors.Wrap(err, "failed to drag out the icon from folder")
	}

	// Launcher supports single-item folders, so the folder should still exist after removing second to last item.
	if err := launcher.RemoveIconFromFolder(tconn, launcher.UnnamedFolderFinder)(ctx); err != nil {
		return errors.Wrap(err, "failed to drag out the icon from single-item folder")
	}

	locAfter, err := ui.Location(ctx, src)
	if err != nil {
		return errors.Wrap(err, "failed to get location of icon after dragging")
	}

	if (locBefore.CenterX() == locAfter.CenterX()) &&
		(locBefore.CenterY() == locAfter.CenterY()) {
		return errors.New("failed to verify dragged icon in the new position")
	}

	return nil
}

// dragIconToNextPage drags the first icon at index itemIndex in the app list UI from the startPage to the next page.
func dragIconToNextPage(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, itemIndex, startPage int) error {
	srcInfo, err := ui.Info(ctx, nodewith.HasClass(launcher.ExpandedItemsClass).Nth(itemIndex))
	if err != nil {
		return errors.Wrap(err, "failed to get information of first icon")
	}

	// Checks item is in current startPage.
	if _, err := isItemInPage(ctx, ui, srcInfo.Name, startPage); err != nil {
		return errors.Wrap(err, "failed to identify page before dragging")
	}

	if err := launcher.DragIconAtIndexToNextPage(tconn, itemIndex)(ctx); err != nil {
		return errors.Wrap(err, "failed to drag icon to next page")
	}

	nextPage := startPage + 1
	if _, err := isItemInPage(ctx, ui, srcInfo.Name, nextPage); err != nil {
		return errors.Wrap(err, "failed to identify page after dragging")
	}
	testing.ContextLogf(ctx, "%q has been moved to page %d", srcInfo.Name, nextPage)

	// Return to the previous page after verifying that dropped app should be in the new page.
	if err := switchToPage(ui, startPage)(ctx); err != nil {
		return errors.Wrap(err, "failed to recovery to the previous page")
	}

	return nil
}

// dragFirstIconToScrollableContainerBottom drags the first item in the scrollable apps grid view to the last available slot in the view.
// Returns the index of the drag item view in the scrollable app grid after successful drag operation.
func dragFirstIconToScrollableContainerBottom(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context) (int, error) {
	scrollableGridItems := nodewith.ClassName(launcher.ExpandedItemsClass).Ancestor(nodewith.ClassName("ScrollableAppsGridView"))

	dragItem := scrollableGridItems.Nth(0)
	dragItemInfo, err := ui.Info(ctx, dragItem)
	if err != nil {
		return -1, errors.Wrap(err, "failed to get drag item info")
	}
	dragItemName := dragItemInfo.Name

	allItemsInfo, err := ui.NodesInfo(ctx, scrollableGridItems)
	if err != nil {
		return -1, errors.Wrap(err, "failed to get list of grid items")
	}
	itemCount := len(allItemsInfo)
	targetItem := scrollableGridItems.Nth(itemCount - 1)

	if err := launcher.DragItemInBubbleLauncherWithScrolling(ctx, tconn, ui, dragItem, targetItem, false /*up*/); err != nil {
		return -1, errors.Wrap(err, "bubble launcher scroll failed")
	}

	// Drag item should have been moved right of the first item in the scrollable grid.
	lastItemInfo, err := ui.Info(ctx, scrollableGridItems.Nth(itemCount-1))
	if err != nil {
		return -1, errors.Wrap(err, "failed to get second app item info")
	}

	if dragItemName != lastItemInfo.Name {
		return -1, errors.Wrapf(err, "Last item %s is not the drag item %s", lastItemInfo.Name, dragItemInfo.Name)
	}
	return itemCount - 1, nil
}

// dragIconToScrollableContainerTop drags an app list item at itemIndex in the scrollable apps grid view to the slot after the first item.
func dragIconToScrollableContainerTop(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, itemIndex int) error {
	scrollableGridItems := nodewith.ClassName(launcher.ExpandedItemsClass).Ancestor(nodewith.ClassName("ScrollableAppsGridView"))
	dragItem := scrollableGridItems.Nth(itemIndex)
	dragItemInfo, err := ui.Info(ctx, dragItem)
	if err != nil {
		return errors.Wrap(err, "failed to get drag item info")
	}
	dragItemName := dragItemInfo.Name

	targetItem := scrollableGridItems.Nth(0)

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
	return nil
}

// getFirstItemOnCurrentPage returns the index on the first app list item that's on the current page
func getFirstItemOnCurrentPage(ctx context.Context, tconn *chrome.TestConn, minIndex int) (int, error) {
	for currentIndex := minIndex; ; currentIndex++ {
		itemOnCurrentPage, err := launcher.IsItemOnCurrentPage(ctx, tconn, nodewith.ClassName(launcher.ExpandedItemsClass).Nth(currentIndex))
		if err != nil {
			return -1, errors.Wrap(err, "checking whether item is on page failed")
		}
		if itemOnCurrentPage {
			return currentIndex, nil
		}
	}
}

// switchToPage switches to the target page by clicking the switch button.
func switchToPage(ui *uiauto.Context, targetPage int) action.Action {
	pageNodeName := regexp.MustCompile(fmt.Sprintf(`Page %d of \d+`, targetPage))
	return uiauto.Combine("switch launcher page",
		ui.LeftClick(nodewith.Role(role.Button).NameRegex(pageNodeName)),
		ui.WaitForLocation(nodewith.HasClass(launcher.ExpandedItemsClass).First()), // Wait for item to be stable.
	)
}

// isItemInPage checks if the target item is located in the expected page.
// Note that this method may change the current page while switching to other page.
func isItemInPage(ctx context.Context, ui *uiauto.Context, itemName string, targetPage int) (bool, error) {
	if err := switchToPage(ui, targetPage)(ctx); err != nil {
		return false, errors.Wrap(err, "failed to switch to page")
	}
	onscreen, err := isItemOnscreen(ctx, ui, itemName)
	if err != nil {
		return false, errors.Wrap(err, "failed to find item in certain page")
	}
	return onscreen, nil
}

// isItemOnscreen checks whether the target is on the current page.
func isItemOnscreen(ctx context.Context, ui *uiauto.Context, itemName string) (bool, error) {
	itemView := launcher.AppItemViewFinder(itemName)
	info, err := ui.Info(ctx, itemView)
	if err != nil {
		return false, err
	}

	return !info.State[state.Offscreen], nil
}

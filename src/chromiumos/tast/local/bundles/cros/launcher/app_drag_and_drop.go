// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AppDragAndDrop,
		Desc: "Test the functionality of dragging and dropping on app icons",
		Contacts: []string{
			"kyle.chen@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:    []string{"group:mainline", "informational"},
		Fixture: "chromeLoggedInWith100FakeApps",
	})
}

// AppDragAndDrop tests the functionality of dragging and dropping on app icons.
func AppDragAndDrop(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	ui := uiauto.New(tconn)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	for _, subtest := range []struct {
		modeName string
		isTablet bool
	}{
		{"drag and drop in clamshell mode", false},
		{"drag and drop in tablet mode", true},
	} {
		cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, subtest.isTablet)
		if err != nil {
			s.Fatal("Failed to set tablet mode to be tabletMode: ", err)
		}
		defer cleanup(cleanupCtx)

		f := func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, subtest.modeName+"_ui_dump")

			if err := launcher.Open(tconn)(ctx); err != nil {
				s.Fatal("Failed to open the launcher: ", err)
			}

			// Each subtest requires at least 3 items on the current page - the first page may have a (default) page break after several
			// default apps, and depending on the device may not have enough apps to satisfy this requirement.
			// To work around this, start the test on the second launcher page.
			if err := newLauncherPageSwitcher(ui).switchToPage(1)(ctx); err != nil {
				s.Fatal("Failed to switch to second page for test: ", err)
			}

			firstItem, err := getFirstItemOnCurrentPage(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to get the first item on the current page: ", err)
			}

			if err := dragIconToIcon(ctx, tconn, ui, firstItem); err != nil {
				s.Fatal("Failed to drag the first icon to the second icon: ", err)
			}

			if err := dragIconToNextPage(ctx, tconn, ui, firstItem); err != nil {
				s.Fatal("Failed to drag the first icon to next page: ", err)
			}
		}

		if !s.Run(ctx, subtest.modeName, f) {
			s.Errorf("Failed to run subtest %q", subtest.modeName)
		}
	}
}

func dragIconToIcon(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, firstItem int) error {
	srcInfo, err := ui.Info(ctx, nodewith.HasClass(launcher.ExpandedItemsClass).Nth(firstItem))
	if err != nil {
		return errors.Wrap(err, "failed to get information of first icon")
	}
	src := launcher.AppItemViewFinder(srcInfo.Name)

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

	if err := launcher.RemoveIconFromFolder(tconn)(ctx); err != nil {
		return errors.Wrap(err, "failed to drag out the icon from folder")
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

// dragIconToNextPage drags an icon to from the current to the next page.
func dragIconToNextPage(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, itemIndex int) error {
	srcInfo, err := ui.Info(ctx, nodewith.HasClass(launcher.ExpandedItemsClass).Nth(itemIndex))
	if err != nil {
		return errors.Wrap(err, "failed to get information of first icon")
	}

	pageBefore, err := identifyItemInWhichPage(ctx, ui, srcInfo.Name)
	if err != nil {
		return errors.Wrap(err, "failed to identify page before dragging")
	}

	if err := launcher.DragIconToNextPage(tconn, itemIndex)(ctx); err != nil {
		return errors.Wrap(err, "failed to drag icon to next page")
	}

	pageAfter, err := identifyItemInWhichPage(ctx, ui, srcInfo.Name)
	if err != nil {
		return errors.Wrap(err, "failed to identify page after dragging")
	}
	testing.ContextLogf(ctx, "%q has been moved to page %d", srcInfo.Name, pageAfter)

	if pageBefore == pageAfter {
		return errors.New("failed to verify dragged icon in the new page")
	}

	// Return to the previous page after verifying that dropped app should be in the new page.
	if err := newLauncherPageSwitcher(ui).switchToPage(pageBefore)(ctx); err != nil {
		return errors.Wrap(err, "failed to recovery to the previous page")
	}

	return nil
}

// getFirstItemOnCurrentPage returns the index on the first app list item that's on the current page
func getFirstItemOnCurrentPage(ctx context.Context, tconn *chrome.TestConn) (int, error) {
	currentIndex := 0

	for {
		itemOnCurrentPage, err := launcher.IsItemOnCurrentPage(ctx, tconn, nodewith.ClassName(launcher.ExpandedItemsClass).Nth(currentIndex))
		if err != nil {
			return -1, errors.Wrap(err, "checking whether item is on page failed")
		}
		if itemOnCurrentPage {
			break
		}
		currentIndex = currentIndex + 1
	}
	return currentIndex, nil
}

// identifyItemInWhichPage identifies which page the target item is on.
func identifyItemInWhichPage(ctx context.Context, ui *uiauto.Context, itemName string) (int, error) {
	switcher := newLauncherPageSwitcher(ui)

	pageCnt, err := switcher.totalPages(ctx)
	if err != nil {
		return pageCnt, nil
	}

	for i := 0; i < pageCnt; i++ {
		if err := switcher.switchToPage(i)(ctx); err != nil {
			return -1, err
		}

		if onscreen, err := switcher.isItemOnscreen(ctx, itemName); err != nil {
			return -1, err
		} else if onscreen {
			return i + 1, nil
		}
	}

	return -1, errors.New("failed to find item in any page")
}

// launcherPageSwitcher holds resources for switching pages when dragging and dropping on app icons.
type launcherPageSwitcher struct {
	ui        *uiauto.Context
	switchBtn *nodewith.Finder
}

// newLauncherPageSwitcher creates a new instance of launcherPageSwitcher.
func newLauncherPageSwitcher(ui *uiauto.Context) *launcherPageSwitcher {
	return &launcherPageSwitcher{
		ui:        ui,
		switchBtn: nodewith.HasClass("Button").Ancestor(nodewith.HasClass("PageSwitcher")),
	}
}

// totalPages counts the number of page.
func (s *launcherPageSwitcher) totalPages(ctx context.Context) (int, error) {
	if err := s.ui.WithTimeout(time.Second).WaitUntilExists(s.switchBtn.First())(ctx); err != nil {
		return 1, errors.Wrap(err, "failed to find switch button")
	}

	buttonsInfo, err := s.ui.NodesInfo(ctx, s.switchBtn)
	if err != nil {
		return 1, errors.Wrap(err, "failed to count total pages")
	}

	return len(buttonsInfo), nil
}

// switchToPage switches to page n by clicking the switch button.
func (s *launcherPageSwitcher) switchToPage(n int) action.Action {
	return uiauto.Combine("switch launcher page",
		s.ui.WithTimeout(5*time.Second).LeftClick(s.switchBtn.Nth(n)),
		s.ui.WaitForLocation(nodewith.HasClass(launcher.ExpandedItemsClass).First()), // Wait for item to be stable.
	)
}

// isItemOnscreen checks whether the target is on the current page.
func (s *launcherPageSwitcher) isItemOnscreen(ctx context.Context, itemName string) (bool, error) {
	itemView := launcher.AppItemViewFinder(itemName)
	item := nodewith.Name(itemName).HasClass("Label").Ancestor(itemView)

	info, err := s.ui.Info(ctx, item.First())
	if err != nil {
		return false, err
	}

	return !info.State[state.Offscreen], nil
}

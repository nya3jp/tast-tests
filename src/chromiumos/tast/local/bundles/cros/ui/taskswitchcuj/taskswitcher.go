// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package taskswitchcuj

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cuj/inputsimulations"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// initializeSwitchTaskByHotseat returns a function f such that f(ctx)
// will use the hotseat to switch the currently focused window to the
// "next window". The "next window" is chosen by looking at the hotseat
// from right to left and clicking on the icon to the left of the
// currently visible icon, as long as there are windows present for that
// icon. If the browser icon is clicked, we select the next window from
// the submenu that pops up.
//
// Assuming we have n windows, and we call f(ctx) n times, we loop
// back to the first window.
func initializeSwitchTaskByHotseat(ctx context.Context, tconn *chrome.TestConn, stw *input.SingleTouchEventWriter, tcc *input.TouchCoordConverter, pc pointer.Context, ac *uiauto.Context, numTotalWindows, numBrowserWindows int) (func(context.Context) error, error) {
	if err := ash.SwipeUpHotseatAndWaitForCompletion(ctx, tconn, stw, tcc); err != nil {
		return nil, errors.Wrap(err, "failed to show the hotseat")
	}
	defer ash.SwipeDownHotseatAndWaitForCompletion(ctx, tconn, stw, tcc)

	// Get the bounds of the shelf icons. The shelf icon bounds are
	// available from ScrollableShelfInfo, while the metadata for ShelfItems
	// are in another place (ShelfItem). Use ShelfItem to filter out
	// the apps with no windows, and fetch their icon bounds from
	// ScrollableShelfInfo.
	items, err := ash.ShelfItems(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to obtain the shelf items")
	}
	shelfInfo, err := ash.FetchScrollableShelfInfoForState(ctx, tconn, &ash.ShelfState{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to obtain the shelf UI info")
	}
	if len(items) != len(shelfInfo.IconsBoundsInScreen) {
		return nil, errors.Errorf("mismatch count of icons in the hotseat: %d vs %d", len(items), len(shelfInfo.IconsBoundsInScreen))
	}

	iconBounds := make([]coords.Rect, 0, len(items))
	for i, item := range items {
		if item.Status == ash.ShelfItemClosed {
			continue
		}
		iconBounds = append(iconBounds, *shelfInfo.IconsBoundsInScreen[i])
	}

	// Find the correct icon for i-th run. Assumptions:
	// - each app icon has 1 window, except for the browser icon (there are numTotalWindows)
	// - browser has the leftmost icon (currIconIdx == 0)
	// With these assumptions, we select the icons from the right, and
	// when we reach the browser icon, we select a window from the popup
	// menu from the top. In other words, there would be icons of
	// [browser] [play store] [gmail] ...
	// and we would select [gmail] -> [play store] -> [browser],
	// where selecting browser icon shows a popup.`
	i := 0
	return func(ctx context.Context) error {
		if i >= numTotalWindows {
			i = 0
		}

		if err := ash.SwipeUpHotseatAndWaitForCompletion(ctx, tconn, stw, tcc); err != nil {
			return errors.Wrap(err, "failed to show the hotseat")
		}

		currIconIdx := numTotalWindows - (numBrowserWindows - 1) - i - 1
		var isPopup bool
		var popupIdx int
		if currIconIdx <= 0 {
			isPopup = true
			// This assumes the order of menu items of window selection popup is
			// stable. Select from the top, but offset-by-one since the first
			// menu item is a non-clickable title.
			popupIdx = -currIconIdx + 1
			currIconIdx = 0
		}

		if err := pc.ClickAt(iconBounds[currIconIdx].CenterPoint())(ctx); err != nil {
			return errors.Wrapf(err, "failed to click icon at %d", currIconIdx)
		}

		if isPopup {
			menus := nodewith.HasClass("MenuItemView")
			if err := ac.WaitUntilExists(menus.First())(ctx); err != nil {
				return errors.Wrap(err, "failed to wait for the menu to appear")
			}
			if err := pc.Click(menus.Nth(popupIdx))(ctx); err != nil {
				return errors.Wrapf(err, "failed to click menu item %d", popupIdx)
			}
		}

		if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
			return ash.WaitWindowFinishAnimating(ctx, tconn, w.ID)
		}); err != nil {
			return errors.Wrap(err, "failed to wait for the window animation")
		}
		i++
		return ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfHidden)
	}, nil
}

// initializeSwitchTaskByAltTab is similar to initializeSwitchTaskByHotseat,
// except it uses Alt+Tab to switch to the least recently used window.
func initializeSwitchTaskByAltTab(ctx context.Context, kw *input.KeyboardEventWriter, numWindows int) func(context.Context) error {
	// Press Alt, hit Tab for the number of windows to choose the last used
	// window, and then release Alt. We prefer this method of Alt+Tab as opposed
	// to Alt+Shift+Tab so that the device is forced to render a preview for
	// every open window, which hopefully increases graphical load.
	return action.Combine(
		"Alt+Tab to the right most window",
		kw.AccelPressAction("Alt"),
		action.Sleep(500*time.Millisecond),
		func(ctx context.Context) error {
			return inputsimulations.RepeatKeyPress(ctx, kw, "Tab", 500*time.Millisecond, numWindows-1)
		},
		action.Sleep(time.Second),
		kw.AccelReleaseAction("Alt"),
	)
}

// initializeSwitchTaskByOverviewMode is similar to initializeSwitchTaskByHotseat,
// except it uses the overview mode to switch to the least recently used window.
// This function assumes that after calling setOverviewMode(ctx), the LRU window
// will be visible on the screen and clickable. This is not trivially the case with
// tablets, as normal overview mode might have the LRU window hidden by a horizontal
// scroll. Thus, |setOverviewMode| needs make the LRU window visible.
func initializeSwitchTaskByOverviewMode(ctx context.Context, tconn *chrome.TestConn, pc pointer.Context, setOverviewMode func(ctx context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		if err := setOverviewMode(ctx); err != nil {
			return errors.Wrap(err, "failed to enter overview mode")
		}

		// If switching task by overview mode fails, ensure to exit
		// overview mode for proper test cleanup. On a successful
		// overview task switch, clicking the window should already
		// close overview mode.
		done := false
		defer func() {
			if !done {
				if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
					testing.ContextLog(ctx, "Failed to exit overview mode: ", err)
				}
			}
		}()

		ws, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get the overview windows")
		}

		// Find the bottom-right overview item, which is the bottom of the LRU
		// list of the windows.
		var targetWindow *ash.Window
		for _, w := range ws {
			if w.OverviewInfo == nil {
				continue
			}
			if targetWindow == nil {
				targetWindow = w
			} else {
				overviewBounds := w.OverviewInfo.Bounds
				targetBounds := targetWindow.OverviewInfo.Bounds
				// Assume the windows are arranged in a grid and pick the
				// bottom right one.
				if overviewBounds.Top > targetBounds.Top || (overviewBounds.Top == targetBounds.Top && overviewBounds.Left > targetBounds.Left) {
					targetWindow = w
				}
			}
		}
		if targetWindow == nil {
			return errors.New("no windows are in overview mode")
		}
		if err := pc.ClickAt(targetWindow.OverviewInfo.Bounds.CenterPoint())(ctx); err != nil {
			return errors.Wrap(err, "failed to click")
		}
		if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
			return w.ID == targetWindow.ID && w.OverviewInfo == nil && w.IsActive
		}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to wait")
		}
		done = true
		return nil
	}
}

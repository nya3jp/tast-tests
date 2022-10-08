// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SplitViewTabletMode,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "In tablet mode, checks split view works properly",
		Contacts: []string{
			"cattalyya@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Params: []testing.Param{{
			Name: "portrait",
			Val:  true,
		}, {
			Name: "landscape",
			Val:  false,
		}},
		Timeout: chrome.GAIALoginTimeout + arc.BootTimeout + 120*time.Second,
		VarDeps: []string{"ui.gaiaPoolDefault"},
	})
}

func SplitViewTabletMode(ctx context.Context, s *testing.State) {
	const (
		newWindowText      = "New window"
		newWindowClassName = "MenuItemView"
	)

	// Reserve ten seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	info, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain internal display info: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure tablet mode: ", err)
	}
	defer cleanup(cleanupCtx)

	defer display.SetDisplayRotationSync(cleanupCtx, tconn, info.ID, display.Rotate0)

	// Rotate the screen if it is a portrait test.
	portrait := s.Param().(bool)
	portraitByDefault := info.Bounds.Height > info.Bounds.Width

	rotations := []display.RotationAngle{display.Rotate0, display.Rotate90, display.Rotate180, display.Rotate270}
	rotIndex := 0
	if portrait != portraitByDefault {
		if portrait {
			// Start with primary portrait which is |display.Rotate270| from the primary landscape display.
			rotIndex = 3
		} else {
			// Start with primary landscape which is |display.Rotate90| from the primary portrait display.
			rotIndex = 1
		}
		if err = display.SetDisplayRotationSync(ctx, tconn, info.ID, rotations[rotIndex]); err != nil {
			s.Fatal("Failed to rotate display: ", err)
		}
	}

	// Obtain the latest display info after rotating the display.
	info, err = display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain internal display info: ", err)
	}

	tew, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Failed to access to the touch screen: ", err)
	}
	defer tew.Close()

	if err := tew.SetRotation(rotIndex * 90); err != nil {
		s.Fatal("Failed to set display rotation: ", err)
	}

	// Setup for launching ARC apps.
	if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store and Close: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(cleanupCtx)

	if err := a.WaitIntentHelper(ctx); err != nil {
		s.Fatal("Failed to wait for ARC Intent Helper: ", err)
	}

	numWindows := 4
	// Launch four windows: two chrome windows and two chrome apps.
	appsList := []apps.App{apps.FilesSWA, apps.PlayStore}

	if err := ash.CreateWindows(ctx, tconn, cr, "", numWindows-len(appsList)); err != nil {
		s.Fatal("Failed to create new windows: ", err)
	}

	for _, app := range appsList {
		if err := apps.Launch(ctx, tconn, app.ID); err != nil {
			s.Fatalf("Failed to launch %s: %s", app.Name, err)
		}
		if err := ash.WaitForApp(ctx, tconn, app.ID, time.Minute); err != nil {
			s.Fatalf("%s did not appear in shelf after launch: %s", app.Name, err)
		}
	}

	// Wait for all windows to show up.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		ws, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to obtain all windows"))
		}
		if len(ws) != numWindows {
			return errors.Errorf("unexpected number of windows: got %d, want %d", len(ws), numWindows)
		}
		for _, w := range ws {
			if w.IsAnimating || w.State != ash.WindowStateMaximized {
				return errors.Errorf("fail to wait for window(id=%d) to finish animating or maximizing", w.ID)
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 20 * time.Second, Interval: 500 * time.Millisecond}); err != nil {
		s.Fatalf("Failed to wait for all %d windows to be opened and maximized", numWindows)
	}

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain all windows: ", err)
	}

	fileWindow1ID := -1
	for _, w := range ws {
		if strings.HasPrefix(w.Title, "Files") {
			fileWindow1ID = w.ID
		}
	}
	if fileWindow1ID == -1 {
		s.Fatal("Fail to open a File app")
	}

	stw, err := tew.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Failed to create a single touch writer: ", err)
	}
	defer stw.Close()

	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to enter overview: ", err)
	}
	defer ash.SetOverviewModeAndWait(cleanupCtx, tconn, false)

	// 1. Test dragging a window to snap to primary-snapped position.
	window1, err := ash.FindFirstWindowInOverview(ctx, tconn)
	workArea := info.WorkArea
	primarySnappedPoint := coords.NewPoint(workArea.Left, workArea.CenterPoint().Y)
	if portrait {
		primarySnappedPoint = coords.NewPoint(workArea.CenterPoint().X, workArea.Top)
	}

	if err := dragToSnapFirstOverviewWindow(ctx, tconn, tew, stw, primarySnappedPoint); err != nil {
		s.Fatal("Failed to drag window from overview and snap left: ", err)
	}

	if err := ash.WaitForCondition(ctx, tconn, func(window *ash.Window) bool {
		return !window.IsAnimating && window.State == ash.WindowStateLeftSnapped && window1.ID == window.ID
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed to wait for primary-snapped window: ", err)
	}

	// 2. Test that clicking a window view in overview turns the window to
	// secondary-snapped state.
	window2, err := ash.FindFirstWindowInOverview(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to find window in overview grid: ", err)
	}

	pc, err := pointer.NewTouch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to set up the touch context: ", err)
	}
	defer pc.Close()

	if err := pc.ClickAt(window2.OverviewInfo.Bounds.CenterPoint())(ctx); err != nil {
		s.Fatalf("Failed to press a window(id=%d) view to snap: %s", window2.ID, err)
	}

	if err := ash.WaitForCondition(ctx, tconn, func(window *ash.Window) bool {
		return !window.IsAnimating && window.State == ash.WindowStateRightSnapped && window.ID == window2.ID
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed to wait for secondary-snapped window: ", err)
	}

	// 3. Test that a window chosen from the shelf replaced the right snapped window.
	// Find the shelf icon buttons.
	tcc := tew.NewTouchCoordConverter(info.Bounds.Size())

	if err := ash.SwipeUpHotseatAndWaitForCompletion(ctx, tconn, stw, tcc); err != nil {
		s.Fatal("Failed to swipe up the hotseat: ", err)
	}

	ui := uiauto.New(tconn)
	filesAppShelfButton := nodewith.Name(apps.Files.Name).ClassName("ash/ShelfAppButton")
	newWindowContextMenuItem := nodewith.Name(newWindowText).ClassName(newWindowClassName)
	if err := uiauto.Combine("click new window context menu item",
		ui.WaitUntilExists(filesAppShelfButton),
		ui.RightClick(filesAppShelfButton),
		ui.WaitUntilExists(newWindowContextMenuItem),
		ui.LeftClick(newWindowContextMenuItem),
	)(ctx); err != nil {
		s.Fatal("Failed to click New Window on Files app shelf icon: ", err)
	}

	if err := ash.WaitForCondition(ctx, tconn, func(window *ash.Window) bool {
		return !window.IsAnimating && window.State == ash.WindowStateRightSnapped && window.ID == window2.ID
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed to wait for secondary-snapped window: ", err)
	}

	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return strings.HasPrefix(w.Title, "Files") && w.ID != fileWindow1ID
	}, &testing.PollOptions{Timeout: 5 * time.Second, Interval: time.Second}); err != nil {
		s.Fatal("Failed to find initial Files app window: ", err)
	}

	ws, err = ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain all windows: ", err)
	}
	fileWindow2ID := -1
	for _, w := range ws {
		if strings.HasPrefix(w.Title, "Files") && w.ID != fileWindow1ID {
			fileWindow2ID = w.ID
		}
	}

	if err := ash.WaitForCondition(ctx, tconn, func(window *ash.Window) bool {
		return !window.IsAnimating && window.State == ash.WindowStateRightSnapped && window.ID == fileWindow2ID
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed to wait for a new window to replace the secondary-snapped window: ", err)
	}

	// 4. Test that swiping up the window from shelf enters intermediate split
	// view that shows overview.

	// Make gesture duration sufficiently long for window drag not to be recognized as a gesture to the home screen.
	duration := time.Duration(info.Bounds.Height/3) * time.Millisecond
	start := coords.NewPoint(info.Bounds.CenterPoint().X/4, info.Bounds.Height)
	startX, startY := tcc.ConvertLocation(start)
	end := coords.NewPoint(info.Bounds.CenterPoint().X/4, info.Bounds.Height/2)
	endX, endY := tcc.ConvertLocation(end)

	if err := stw.Swipe(ctx, startX, startY-1, endX, endY, duration); err != nil {
		s.Fatal("Failed to swipe: ", err)
	}

	// Wait with the swipe paused so the overview mode gesture is recognized. Use 1 second because this is roughly the amount of time it takes for the 'swipe up and hold' overview gesture to trigger.
	const pauseDuration = time.Second
	if err = testing.Sleep(ctx, pauseDuration); err != nil {
		s.Fatal("Failed to sleep while waiting for overview to trigger: ", err)
	}
	if err := stw.End(); err != nil {
		s.Fatal("Failed to finish the swipe gesture: ", err)
	}

	// When the drag up ends overview is already fully shown. The only thing that remains is to wait for the windows to finish animating to their final point in the overview grid.
	for _, window := range ws {
		if err := ash.WaitWindowFinishAnimating(ctx, tconn, window.ID); err != nil {
			s.Fatal("Failed to wait for the dragged window to animate: ", err)
		}
	}

	// Now that all windows are done animating, ensure overview is still shown.
	if err := ash.WaitForOverviewState(ctx, tconn, ash.Shown, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for animation to finish: ", err)
	}

}

// dragToSnapFirstOverviewWindow finds the first window in overview, and drags
// to snap it. This function assumes that overview is already active.
func dragToSnapFirstOverviewWindow(ctx context.Context, tconn *chrome.TestConn, tew *input.TouchscreenEventWriter, stw *input.SingleTouchEventWriter, target coords.Point) error {
	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the primary display info")
	}
	tcc := tew.NewTouchCoordConverter(info.Bounds.Size())

	w, err := ash.FindFirstWindowInOverview(ctx, tconn)
	if err != nil {
		// If you see this error on the second window snap (to the right), check if
		// b/143499564 has been reintroduced.
		return errors.Wrap(err, "failed to find window in overview grid")
	}

	centerX, centerY := tcc.ConvertLocation(w.OverviewInfo.Bounds.CenterPoint())
	if err := stw.LongPressAt(ctx, centerX, centerY); err != nil {
		return errors.Wrap(err, "failed to long-press to start dragging window")
	}

	// Validity check to ensure there is one dragging item.
	if _, err := ash.DraggedWindowInOverview(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to get dragged overview item")
	}

	targetX, targetY := tcc.ConvertLocation(target)

	// Make sure that the split view drag indicator shows up while dragging a
	// window in overview.
	if dragIndicator := nodewith.ClassName("SplitViewDragIndicator").First(); dragIndicator == nil {
		return errors.Wrap(err, "failed to display drag indicator")
	}

	if err := stw.Swipe(ctx, centerX, centerY, targetX, targetY, time.Second); err != nil {
		return errors.Wrap(err, "failed to swipe for snapping window")
	}
	if err := stw.End(); err != nil {
		return errors.Wrap(err, "failed to end swipe")
	}

	return nil
}

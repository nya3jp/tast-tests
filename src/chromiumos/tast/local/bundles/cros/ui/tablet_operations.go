// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/perfutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: TabletOperations,
		Desc: "Check if the performance around user operations for tablet mode is good enough; see also go/cros-ui-perftests-cq#heading=h.fwfk0yg3teo1",
		Contacts: []string{
			"xdai@chromium.org",
			"sammiequon@chromium.org",
			"chromeos-wmp@google.com",
			"mukai@chromium.org", // Tast author
		},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          chrome.LoggedIn(),
		SoftwareDeps: []string{"chrome", "tablet_mode"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
	})
}

func TabletOperations(ctx context.Context, s *testing.State) {
	expects := perfutil.CreateExpectations(ctx,
		"Ash.DragWindowFromShelf.PresentationTime",
		"Ash.HotseatTransition.AnimationSmoothness.TransitionToHiddenHotseat",
		"Ash.HotseatTransition.Drag.PresentationTime",
		"Ash.HotseatWidgetAnimation.Widget.AnimationSmoothness.TransitionToHiddenHotseat",
		"Ash.Overview.AnimationSmoothness.Enter.SplitView",
		"Ash.Overview.AnimationSmoothness.Exit.SplitView",
		"Ash.Overview.WindowDrag.PresentationTime.TabletMode",
		"Ash.SplitViewResize.AnimationSmoothness.DividerAnimation",
		"Ash.SplitViewResize.PresentationTime.TabletMode.WithOverview",
		// Ash.TabletMode.AnimationSmoothness.{Enter,Exit} are skipped, as it is
		// known to be bad. TODO(https://crbug.com/1054489): add them.
	)
	// When custom expectation value needs to be set, modify expects here.

	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get the connection to the test API: ", err)
	}

	conns, err := ash.CreateWindows(ctx, tconn, cr, "", 2)
	if err != nil {
		s.Fatal("Failed to create new windows: ", err)
	}
	if err := conns.Close(); err != nil {
		s.Fatal("Failed to close the connections: ", err)
	}

	r := perfutil.NewRunner(cr)
	r.Runs = 3
	r.RunTracing = false

	s.Log("1. enter/exit tablet mode status")
	// Before conducting the animation, ensure into the clamshell mode.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure into clamshell mode: ", err)
	}
	defer cleanup(ctx)

	// Turn windows into normal state before entering into tablet-mode.
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get window information: ", err)
	}
	for _, w := range ws {
		if err := ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateNormal); err != nil {
			s.Fatalf("Failed to set window %d state to normal: %v", w.ID, err)
		}
	}
	r.RunMultiple(ctx, s, "tablet-mode", perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		if err := ash.SetTabletModeEnabled(ctx, tconn, true); err != nil {
			return errors.Wrap(err, "failed to enable tablet mode")
		}

		// Wait for the top window to finish animating before changing states.
		if err := ash.WaitWindowFinishAnimating(ctx, tconn, ws[0].ID); err != nil {
			return errors.Wrap(err, "failed to wait for top window animation")
		}

		if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
			return errors.Wrap(err, "failed to disable tablet mode")
		}

		if err := ash.WaitWindowFinishAnimating(ctx, tconn, ws[0].ID); err != nil {
			return errors.Wrap(err, "failed to wait for top window animation")
		}
		return nil
	},
		"Ash.TabletMode.AnimationSmoothness.Enter",
		"Ash.TabletMode.AnimationSmoothness.Exit",
	), perfutil.StoreAllWithHeuristics)

	if err := ash.SetTabletModeEnabled(ctx, tconn, true); err != nil {
		s.Fatal("Failed to enter into the tablet mode: ", err)
	}

	tsew, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Failed to get the touch screen: ", err)
	}
	defer tsew.Close()
	// Ensures in the landscape orientation; the following test scenario won't
	// succeed when the device is in the portrait mode.
	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the orientation info: ", err)
	}
	angle := -orientation.Angle
	info, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the internal display info: ", err)
	}
	if orientation.Type == display.OrientationPortraitPrimary {
		angle += 90
		if err = display.SetDisplayRotationSync(ctx, tconn, info.ID, display.Rotate90); err != nil {
			s.Fatal("Failed to rotate display: ", err)
		}
		defer display.SetDisplayRotationSync(ctx, tconn, info.ID, display.Rotate0)
		info, err = display.GetInternalInfo(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to obtain the internal display info: ", err)
		}
	}
	tsew.SetRotation(angle)

	tcc := tsew.NewTouchCoordConverter(info.Bounds.Size())
	stw, err := tsew.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Failed to get the single touch event writer: ", err)
	}
	defer stw.Close()

	// Have a browser window, and swipe from the bottom of the screen and then
	// release the finger; this will switch the screen to the app-list. Then
	// tap the Chrome icon in the hotseat to re-activate the window.
	s.Log("2. swipe up to minimize the window")
	r.RunMultiple(ctx, s, "hotseat-revealing", perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		if err := ash.DragToShowHomescreen(ctx, tsew.Width(), tsew.Height(), stw, tconn); err != nil {
			return errors.Wrap(err, "failed to show homescreen")
		}
		if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfShownHomeLauncher); err != nil {
			return errors.Wrap(err, "hotseat is in an unexpected state")
		}
		// Tap the chrome icon in the app-list to re-activate the browser window.
		findParams := chromeui.FindParams{
			ClassName: "ui/app_list/AppListItemView",
			Attributes: map[string]interface{}{
				"name": regexp.MustCompile("(Chrome|Chromium)"),
			},
		}
		button, err := chromeui.Find(ctx, tconn, findParams)
		if err != nil {
			return errors.Wrap(err, "failed to find the Chrome icon")
		}
		defer button.Release(ctx)
		if err := stw.Move(tcc.ConvertLocation(button.Location.CenterPoint())); err != nil {
			return errors.Wrap(err, "failed to tap the leftmost icon")
		}
		if err := stw.End(); err != nil {
			return errors.Wrap(err, "failed to release the touch")
		}
		for _, w := range ws {
			if err := ash.WaitWindowFinishAnimating(ctx, tconn, w.ID); err != nil {
				return errors.Wrapf(err, "failed to wait for window (%d) animating", w.ID)
			}
		}
		return nil
	},
		"Ash.DragWindowFromShelf.PresentationTime",
		"Ash.HotseatTransition.AnimationSmoothness.TransitionToHiddenHotseat",
		"Ash.HotseatTransition.Drag.PresentationTime",
		"Ash.HotseatWidgetAnimation.Widget.AnimationSmoothness.TransitionToHiddenHotseat",
	), perfutil.StoreAllWithHeuristics)

	// This part works as:
	// - enter into the overview mode
	// - drag a window to the left to cause the split-view
	// - move the split-view divider to the right edge of the screen
	// - then wait for it to end the split-view
	s.Log("3. swipe to overview, enter splitview, and resize")
	r.RunMultiple(ctx, s, "overview-splitview", perfutil.RunAndWaitAll(tconn, func(ctx context.Context) (err error) {
		if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			return errors.Wrap(err, "failed to enter into overview with gesture")
		}
		w, findErr := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool { return w.OverviewInfo != nil })
		if findErr != nil {
			return errors.Wrap(err, "failed to find a window in overview")
		}
		wx, wy := tcc.ConvertLocation(w.OverviewInfo.Bounds.CenterPoint())
		if err := stw.LongPressAt(ctx, wx, wy); err != nil {
			return errors.Wrapf(err, "failed to activate drag of window %d", w.ID)
		}
		pressed := true
		defer func() {
			if !pressed {
				return
			}
			if endErr := stw.End(); endErr != nil {
				s.Log("Failed to release the touch: ", endErr)
				if err == nil {
					err = endErr
				}
			}
		}()
		leftX, _ := tcc.ConvertLocation(info.Bounds.TopLeft())
		centerX, centerY := tcc.ConvertLocation(info.Bounds.CenterPoint())
		right := info.Bounds.TopRight()
		right.X -= info.Bounds.Width / 20
		rightX, _ := tcc.ConvertLocation(right)
		// Swipe to the left-center to enter into the split-view mode.
		if err := stw.Swipe(ctx, wx, wy, leftX, centerY, 300*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to swipe to left")
		}
		if err := stw.End(); err != nil {
			return errors.Wrap(err, "failed to end the swipe to left")
		}
		pressed = false
		if err := ash.WaitWindowFinishAnimating(ctx, tconn, w.ID); err != nil {
			return errors.Wrap(err, "failed to wait for window animating")
		}
		// Exit the overview mode on the lefthand side, and then re-enter into
		// overview mode.
		ow, err := ash.FindFirstWindowInOverview(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to find the window in overview mode")
		}
		if err := stw.Move(tcc.ConvertLocation(ow.OverviewInfo.Bounds.CenterPoint())); err != nil {
			return errors.Wrapf(err, "failed to tap on window %d", ow.ID)
		}
		pressed = true
		if err := stw.End(); err != nil {
			return errors.Wrap(err, "failed to release the tap")
		}
		pressed = false
		if err := ash.WaitWindowFinishAnimating(ctx, tconn, ow.ID); err != nil {
			return errors.Wrap(err, "failed to wait for the overview window animation")
		}

		if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			return errors.Wrap(err, "failed to enter into the overview mode")
		}

		// Swipe on the splitview divider to exit splitview.
		if err := stw.Swipe(ctx, centerX, centerY, rightX, centerY, 300*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to swipe to right")
		}
		if err := stw.End(); err != nil {
			return errors.Wrap(err, "failed to end the swipe to right")
		}
		return ash.WaitForCondition(ctx, tconn, func(window *ash.Window) bool {
			return window.ID == w.ID && !window.IsAnimating && window.State != ash.WindowStateLeftSnapped
		}, &testing.PollOptions{Timeout: 2 * time.Second})
	},
		"Ash.Overview.AnimationSmoothness.Enter.SplitView",
		"Ash.Overview.AnimationSmoothness.Exit.SplitView",
		"Ash.Overview.WindowDrag.PresentationTime.TabletMode",
		"Ash.SplitViewResize.AnimationSmoothness.DividerAnimation",
		"Ash.SplitViewResize.PresentationTime.TabletMode.WithOverview",
	), perfutil.StoreAllWithHeuristics)

	// Check the validity of histogram data.
	for _, err := range r.Values().Verify(ctx, expects) {
		s.Error("Performance expectation missed: ", err)
	}
	// Storing the results for the future analyses.
	if err := r.Values().Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed to save the values: ", err)
	}
}

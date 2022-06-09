// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/wm"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SplitView,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests split view works properly with ARC apps",
		Contacts:     []string{"toshikikikuchi@chromium.org", "amusbach@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Params: []testing.Param{
			{
				Name:              "clamshell_mode",
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "arcBootedInClamshellMode",
				Val:               false,
			},
			{
				Name:              "clamshell_mode_vm",
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "arcBootedInClamshellMode",
				Val:               false,
			},
			{
				Name:              "tablet_mode",
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "arcBootedInTabletMode",
				Val:               false,
			},
			{
				Name:              "tablet_mode_vm",
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "arcBootedInTabletMode",
				Val:               false,
			},
			{
				Name:              "tablet_home_launcher",
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "arcBootedInTabletMode",
				Val:               true,
			},
			{
				Name:              "tablet_home_launcher_vm",
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "arcBootedInTabletMode",
				Val:               true,
			},
		},
	})
}

func waitForWindowBoundsCondition(ctx context.Context, tconn *chrome.TestConn, packageName string, cond func(coords.Rect) error) error {
	checkIfSatisfied := func(ctx context.Context) error {
		window, err := ash.GetARCAppWindowInfo(ctx, tconn, packageName)
		if err != nil {
			return testing.PollBreak(err)
		}
		return cond(window.BoundsInRoot)
	}
	if err := testing.Poll(ctx, checkIfSatisfied, &testing.PollOptions{Interval: 2 * time.Second, Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for the bounds condition to be satisfied")
	}

	return nil
}
func testDragCaptionToSnapLeftRight(ctx context.Context, tconn *chrome.TestConn, d *ui.Device, pc pointer.Context, displayInfo *display.Info, leftAct, rightAct *arc.Activity) error {
	if err := wm.DragCaptionToSnap(ctx, tconn, pc, displayInfo, leftAct, true /* primary */); err != nil {
		return errors.Wrap(err, "failed to drag window's caption and snap to left")
	}
	if err := wm.WaitForArcAndAshWindowState(ctx, tconn, d, leftAct, arc.WindowStatePrimarySnapped); err != nil {
		return errors.Wrap(err, "failed to wait until window state changes to primary snapped")
	}

	if err := wm.DragCaptionToSnap(ctx, tconn, pc, displayInfo, rightAct, false /* primary */); err != nil {
		return errors.Wrap(err, "failed to drag window's caption and snap to right")
	}
	if err := wm.WaitForArcAndAshWindowState(ctx, tconn, d, rightAct, arc.WindowStateSecondarySnapped); err != nil {
		return errors.Wrap(err, "failed to wait until window state changes to secondary snapped")
	}

	return nil
}

func testDragCaptionToUnsnapLeftRight(ctx context.Context, tconn *chrome.TestConn, d *ui.Device, pc pointer.Context, displayInfo *display.Info, leftAct, rightAct *arc.Activity) error {
	if err := wm.DragCaptionToUnsnap(ctx, tconn, pc, displayInfo, leftAct); err != nil {
		return errors.Wrap(err, "failed to drag window's caption and unsnap from left")
	}
	if err := wm.WaitForArcAndAshWindowState(ctx, tconn, d, leftAct, arc.WindowStateNormal); err != nil {
		return errors.Wrap(err, "failed to wait until window state changes to unsnapped")
	}

	if err := wm.DragCaptionToUnsnap(ctx, tconn, pc, displayInfo, rightAct); err != nil {
		return errors.Wrap(err, "failed to drag window's caption and unsnap from right")
	}
	if err := wm.WaitForArcAndAshWindowState(ctx, tconn, d, rightAct, arc.WindowStateNormal); err != nil {
		return errors.Wrap(err, "failed to wait until window state changes to unsnapped")
	}

	return nil
}

func testSnapLeftRightViaKeyboardShortcut(ctx context.Context, tconn *chrome.TestConn, d *ui.Device, leftAct, rightAct *arc.Activity) error {
	if err := wm.ToggleSnapViaKeyboardShortcut(ctx, tconn, leftAct, true /* primary */); err != nil {
		return errors.Wrap(err, "failed to snap window to left via keyboard shortcut")
	}
	if err := wm.WaitForArcAndAshWindowState(ctx, tconn, d, leftAct, arc.WindowStatePrimarySnapped); err != nil {
		return errors.Wrap(err, "failed to wait until window state changes to primary snapped")
	}

	if err := wm.ToggleSnapViaKeyboardShortcut(ctx, tconn, rightAct, false /* primary */); err != nil {
		return errors.Wrap(err, "failed to snap window to right via keyboard shortcut")
	}
	if err := wm.WaitForArcAndAshWindowState(ctx, tconn, d, rightAct, arc.WindowStateSecondarySnapped); err != nil {
		return errors.Wrap(err, "failed to wait until window state changes to secondary snapped")
	}

	return nil
}

func testUnsnapLeftRightViaKeyboardShortcut(ctx context.Context, tconn *chrome.TestConn, d *ui.Device, leftAct, rightAct *arc.Activity) error {
	if err := wm.ToggleSnapViaKeyboardShortcut(ctx, tconn, leftAct, true /* primary */); err != nil {
		return errors.Wrap(err, "failed to unsnap window from left via keyboard shortcut")
	}
	if err := wm.WaitForArcAndAshWindowState(ctx, tconn, d, leftAct, arc.WindowStateNormal); err != nil {
		return errors.Wrap(err, "failed to wait until window state changes to unsnapped state")
	}

	if err := wm.ToggleSnapViaKeyboardShortcut(ctx, tconn, rightAct, false /* primary */); err != nil {
		return errors.Wrap(err, "failed to unsnap window from right via keyboard shortcut")
	}
	if err := wm.WaitForArcAndAshWindowState(ctx, tconn, d, rightAct, arc.WindowStateNormal); err != nil {
		return errors.Wrap(err, "failed to wait until window state changes to unsnapped state")
	}

	return nil
}

func testSnapLeftRightFromOverview(ctx, cleanupCtx context.Context, tconn *chrome.TestConn, d *ui.Device, pc pointer.Context, leftAct, rightAct *arc.Activity) error {
	if err := leftAct.Focus(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to focus the activity")
	}

	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		return errors.Wrap(err, "failed to enter overview")
	}
	defer ash.SetOverviewModeAndWait(cleanupCtx, tconn, false)

	if err := wm.DragToSnapFirstOverviewWindow(ctx, tconn, pc, true /* primary */); err != nil {
		return errors.Wrap(err, "failed to drag window from overview and snap to left")
	}
	if err := wm.WaitForArcAndAshWindowState(ctx, tconn, d, leftAct, arc.WindowStatePrimarySnapped); err != nil {
		return errors.Wrap(err, "failed to wait until window state change")
	}

	if err := wm.DragToSnapFirstOverviewWindow(ctx, tconn, pc, false /* primary */); err != nil {
		return errors.Wrap(err, "failed to drag window from overview and snap to right")
	}
	if err := wm.WaitForArcAndAshWindowState(ctx, tconn, d, rightAct, arc.WindowStateSecondarySnapped); err != nil {
		return errors.Wrap(err, "failed to wait until window state change")
	}

	return nil
}

func testResize(ctx context.Context, tconn *chrome.TestConn, d *ui.Device, ui *uiauto.Context, pc pointer.Context, tabletMode bool, leftActPackageName, rightActPackageName string) error {
	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the primary display info")
	}

	center := info.Bounds.CenterPoint()
	target := coords.NewPoint(info.Bounds.Width/4, center.Y)
	if tabletMode {
		if err := pc.Drag(center, pc.DragTo(target, time.Second))(ctx); err != nil {
			return errors.Wrap(err, "failed to drag to resize")
		}
	} else {
		// In clamshell mode, we need to use "multi-window resizer" to resize two windows simultaneously.
		if err := uiauto.Combine(
			"hover mouse where windows meet",
			mouse.Move(tconn, center.Sub(coords.NewPoint(10, 10)), 0),
			mouse.Move(tconn, center, time.Second),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to move mouse to summon multi-window resizer")
		}

		resizerBounds, err := ui.Location(ctx, nodewith.Role("window").ClassName("MultiWindowResizeController"))
		if err != nil {
			return errors.Wrap(err, "failed to get the multi-window resizer location")
		}

		if err := pc.Drag(resizerBounds.CenterPoint(), pc.DragTo(target, time.Second))(ctx); err != nil {
			return errors.Wrap(err, "failed to drag to resize")
		}
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		leftWindow, err := ash.GetARCAppWindowInfo(ctx, tconn, leftActPackageName)
		if err != nil {
			return testing.PollBreak(err)
		}
		if leftWindow.BoundsInRoot.Left != 0 {
			return errors.New("split resizing is not completed")
		}
		return nil
	}, &testing.PollOptions{Interval: 500 * time.Millisecond, Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait until split resizing is completed")
	}

	// On low-end devices, the window bounds change isn't always smooth so here
	// we keep polling the bounds until the condition gets satisfied.
	widthMargin := 50

	if err := waitForWindowBoundsCondition(ctx, tconn, leftActPackageName, func(bounds coords.Rect) error {
		if bounds.Top != 0 || bounds.Width >= info.Bounds.Width/2-widthMargin {
			return errors.Errorf("failed to verify the resized left snapped window bounds (got %v)", bounds)
		}
		return nil
	}); err != nil {
		return err
	}
	if err := waitForWindowBoundsCondition(ctx, tconn, rightActPackageName, func(bounds coords.Rect) error {
		if bounds.Top != 0 || bounds.Right() != info.Bounds.Right() || bounds.Width <= info.Bounds.Width/2+widthMargin {
			return errors.Errorf("failed to verify the resized right snapped window bounds (got %v)", bounds)
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func SplitView(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Second*10)
	defer cancel()

	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC
	d := s.FixtValue().(*arc.PreData).UIDevice

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	ui := uiauto.New(tconn).WithTimeout(5 * time.Second)

	// Ensure landscape orientation so this test can assume that windows snap on
	// the left and right. Windows snap on the top and bottom in portrait-oriented
	// tablet mode. They snap on the left and right in portrait-oriented clamshell
	// mode, but there are active (although contentious) proposals to change that.
	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the orientation info: ", err)
	}
	displayInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain primary display info: ", err)
	}
	if orientation.Type == display.OrientationPortraitPrimary {
		if err = display.SetDisplayRotationSync(ctx, tconn, displayInfo.ID, display.Rotate90); err != nil {
			s.Fatal("Failed to rotate display: ", err)
		}
		defer display.SetDisplayRotationSync(cleanupCtx, tconn, displayInfo.ID, display.Rotate0)
	}

	// Show two activities. As the content of the activities doesn't matter,
	// use two activities available by default.
	var rightAct, leftAct *arc.Activity
	for _, app := range []struct {
		act          **arc.Activity
		apkName      string
		pkgName      string
		activityName string
	}{
		{&rightAct, wm.APKNameArcWMTestApp24, wm.Pkg24, wm.ResizableUnspecifiedActivity},
		{&leftAct, wm.APKNameArcWMTestApp24PhoneSize, wm.Pkg24InPhoneSizeList, wm.ResizableUnspecifiedActivity},
	} {
		if err := a.Install(ctx, arc.APKPath(app.apkName)); err != nil {
			s.Fatal("Failed to install app: ", err)
		}
		defer a.Uninstall(cleanupCtx, app.pkgName)

		act, err := arc.NewActivity(a, app.pkgName, app.activityName)
		if err != nil {
			s.Fatalf("Failed to create a new activity (%s): %v", app.pkgName, err)
		}
		defer act.Close()
		if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
			s.Fatalf("Failed to start the activity (%s): %v", app.pkgName, err)
		}
		defer act.Stop(cleanupCtx, tconn)
		if err := ash.WaitForVisible(ctx, tconn, app.pkgName); err != nil {
			s.Fatalf("Failed to wait for visible app (%s): %v", app.pkgName, err)
		}

		*app.act = act
		if err := d.WaitForIdle(ctx, 10*time.Second); err != nil {
			s.Fatal("Failed to wait for idle: ", err)
		}
	}

	// We put this "defer" statement after setting the defer statements for cleaning up the apps
	// so that we can capture the state *before* closing the apps when it fails.
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	tabletMode, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to check whether tablet mode is active: ", err)
	}

	var pc pointer.Context
	if tabletMode {
		pc, err = pointer.NewTouch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to set up the touch context: ", err)
		}
	} else {
		pc = pointer.NewMouse(tconn)
	}
	defer pc.Close()

	if s.Param().(bool) { // arc.SplitView.tablet_home_launcher or arc.SplitView.tablet_home_launcher_vm
		tew, err := touch.NewTouchscreen(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to access to the touchscreen: ", err)
		}
		defer tew.Close()

		stw, err := tew.NewSingleTouchWriter()
		if err != nil {
			s.Fatal("Failed to create the single touch writer: ", err)
		}

		if err := ash.DragToShowHomescreen(ctx, tew.Width(), tew.Height(), stw, tconn); err != nil {
			s.Fatal("Failed to drag to show home launcher: ", err)
		}
		if err := wm.WaitForArcAndAshWindowState(ctx, tconn, d, leftAct, arc.WindowStateMinimized); err != nil {
			// If you see this error, check if https://crbug.com/1109250 has been reintroduced.
			s.Fatal("Failed to wait until window state change: ", err)
		}
		if err := wm.WaitForArcAndAshWindowState(ctx, tconn, d, rightAct, arc.WindowStateMinimized); err != nil {
			// If you see this error, check if https://crbug.com/1109250 has been reintroduced.
			s.Fatal("Failed to wait until window state change: ", err)
		}
	}

	if !tabletMode {
		// On small displays with R and devices with P, the app gets launched in a maximized state
		// although the drag-to-snap assumes the app is in a freeform mode.
		if err := wm.RestoreARCWindowIfMaximized(ctx, tconn, leftAct.PackageName()); err != nil {
			s.Fatal("Failed to restore left window if maximized: ", err)
		}
		if err := wm.RestoreARCWindowIfMaximized(ctx, tconn, rightAct.PackageName()); err != nil {
			s.Fatal("Failed to restore right window if maximized: ", err)
		}

		if err := testDragCaptionToSnapLeftRight(ctx, tconn, d, pc, displayInfo, leftAct, rightAct); err != nil {
			s.Fatal("Failed to drag windows' caption to snap: ", err)
		}
		if err := testDragCaptionToUnsnapLeftRight(ctx, tconn, d, pc, displayInfo, leftAct, rightAct); err != nil {
			s.Fatal("Failed to drag windows' caption to unsnap: ", err)
		}

		if err := testSnapLeftRightViaKeyboardShortcut(ctx, tconn, d, leftAct, rightAct); err != nil {
			s.Fatal("Failed to snap windows via keyboard shortcut: ", err)
		}
		if err := testUnsnapLeftRightViaKeyboardShortcut(ctx, tconn, d, leftAct, rightAct); err != nil {
			s.Fatal("Failed to Unsnap windows via keyboard shortcut: ", err)
		}
	}

	if err := testSnapLeftRightFromOverview(ctx, cleanupCtx, tconn, d, pc, leftAct, rightAct); err != nil {
		s.Fatal("Failed to snap windows from overview: ", err)
	}

	// Resize snapped windows.
	if err := testResize(ctx, tconn, d, ui, pc, tabletMode, leftAct.PackageName(), rightAct.PackageName()); err != nil {
		s.Fatal("Failed to resize the snapped windows: ", err)
	}

	if tabletMode {
		// Swap the left activity and the right activity.
		if err := ash.SwapWindowsInSplitView(ctx, tconn); err != nil {
			s.Fatal("Failed to swap windows in split view: ", err)
		}
		leftAct, rightAct = rightAct, leftAct
		if err := wm.WaitForArcAndAshWindowState(ctx, tconn, d, leftAct, arc.WindowStatePrimarySnapped); err != nil {
			s.Fatal("Failed to wait until window state change: ", err)
		}
		if err := wm.WaitForArcAndAshWindowState(ctx, tconn, d, rightAct, arc.WindowStateSecondarySnapped); err != nil {
			s.Fatal("Failed to wait until window state change: ", err)
		}
	}
}

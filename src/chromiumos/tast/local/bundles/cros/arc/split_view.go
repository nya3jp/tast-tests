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

	widthMargin := 50

	leftWindow, err := ash.GetARCAppWindowInfo(ctx, tconn, leftActPackageName)
	if err != nil {
		return errors.Wrap(err, "failed to get left window info")
	}
	if leftWindow.BoundsInRoot.Top != 0 || leftWindow.BoundsInRoot.Width >= info.Bounds.Width/2-widthMargin {
		return errors.Wrapf(err, "failed to verify the resized left snapped window bounds (got %v)", leftWindow.BoundsInRoot)
	}

	rightWindow, err := ash.GetARCAppWindowInfo(ctx, tconn, rightActPackageName)
	if err != nil {
		return errors.Wrap(err, "failed to get right window info")
	}
	if rightWindow.BoundsInRoot.Top != 0 || rightWindow.BoundsInRoot.Right() != info.Bounds.Right() || rightWindow.BoundsInRoot.Width <= info.Bounds.Width/2+widthMargin {
		return errors.Wrapf(err, "failed to verify the resized right snapped window bounds (got %v)", rightWindow.BoundsInRoot)
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
	if orientation.Type == display.OrientationPortraitPrimary {
		info, err := display.GetPrimaryInfo(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to obtain primary display info: ", err)
		}
		if err = display.SetDisplayRotationSync(ctx, tconn, info.ID, display.Rotate90); err != nil {
			s.Fatal("Failed to rotate display: ", err)
		}
		defer display.SetDisplayRotationSync(cleanupCtx, tconn, info.ID, display.Rotate0)
	}

	// Show two activities. As the content of the activities doesn't matter,
	// use two activities available by default.
	var rightAct, leftAct *arc.Activity
	for _, app := range []struct {
		act          **arc.Activity
		pkgName      string
		activityName string
	}{
		{&rightAct, "com.android.storagemanager", ".deletionhelper.DeletionHelperActivity"},
		{&leftAct, "com.android.settings", ".Settings"},
	} {
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
		tew, _, err := touch.NewTouchscreenAndConverter(ctx, tconn)
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

	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to enter overview: ", err)
	}
	defer ash.SetOverviewModeAndWait(cleanupCtx, tconn, false)

	// Snap activity to left.
	if err := wm.DragToSnapFirstOverviewWindow(ctx, tconn, pc, true /* primary */); err != nil {
		s.Fatal("Failed to drag window from overview and snap left: ", err)
	}
	if err := wm.WaitForArcAndAshWindowState(ctx, tconn, d, leftAct, arc.WindowStatePrimarySnapped); err != nil {
		s.Fatal("Failed to wait until window state change: ", err)
	}

	// Snap activity to right.
	if err := wm.DragToSnapFirstOverviewWindow(ctx, tconn, pc, false /* primary */); err != nil {
		s.Fatal("Failed to drag window from overview and snap right: ", err)
	}
	if err := wm.WaitForArcAndAshWindowState(ctx, tconn, d, rightAct, arc.WindowStateSecondarySnapped); err != nil {
		s.Fatal("Failed to wait until window state change: ", err)
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

// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/wm"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/touch"
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
		// TODO(b/188754062): Add support for mouse input and remove the internal display deps
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
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
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "arcBootedInTabletMode",
				Val:               false,
			},
			{
				Name:              "tablet_mode_vm",
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "arcBootedInTabletMode",
				Val:               false,
			},
			{
				Name:              "tablet_home_launcher",
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "arcBootedInTabletMode",
				Val:               true,
			},
			{
				Name:              "tablet_home_launcher_vm",
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "arcBootedInTabletMode",
				Val:               true,
			},
		},
	})
}

// showActivityForSplitViewTest starts an activity and waits for it to be idle.
func showActivityForSplitViewTest(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, pkgName, activityName string) (*arc.Activity, error) {
	act, err := arc.NewActivity(a, pkgName, activityName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a new activity")
	}
	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		act.Close()
		return nil, errors.Wrap(err, "failed to start the activity")
	}
	return act, nil
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
	rightAct, err := showActivityForSplitViewTest(
		ctx, tconn, a, "com.android.storagemanager", ".deletionhelper.DeletionHelperActivity")
	if err != nil {
		s.Fatal("Failed to show an activity: ", err)
	}
	defer rightAct.Close()
	defer rightAct.Stop(cleanupCtx, tconn)
	if err := d.WaitForIdle(ctx, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for idle: ", err)
	}

	leftAct, err := showActivityForSplitViewTest(ctx, tconn, a, "com.android.settings", ".Settings")
	if err != nil {
		s.Fatal("Failed to show an activity: ", err)
	}
	defer leftAct.Close()
	defer leftAct.Stop(cleanupCtx, tconn)
	if err := d.WaitForIdle(ctx, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for idle: ", err)
	}

	tabletMode, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to check whether tablet mode is active: ", err)
	}

	pc, err := pointer.NewTouch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to set up the touch context: ", err)
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

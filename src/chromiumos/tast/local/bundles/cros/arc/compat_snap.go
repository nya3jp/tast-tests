// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/wm"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CompatSnap,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests compatible snapping works properly for resize-locked ARC apps",
		Contacts:     []string{"toshikikikuchi@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Timeout:      4 * time.Minute,
		Fixture:      "arcBootedInClamshellMode",
	})
}

func checkCompatSnappedWindowState(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, cr *chrome.Chrome, d *ui.Device, act *arc.Activity, primary bool, stableWidth int) error {
	snappedArcWindowState := arc.WindowStateSecondarySnapped
	if primary {
		snappedArcWindowState = arc.WindowStatePrimarySnapped
	}

	if err := wm.WaitForArcAndAshWindowState(ctx, tconn, d, act, snappedArcWindowState); err != nil {
		return errors.Wrap(err, "failed to wait until window state changes to snapped")
	}

	window, err := ash.GetARCAppWindowInfo(ctx, tconn, act.PackageName())
	if err != nil {
		return errors.Wrap(err, "failed to get window info")
	}

	snappedWidth := window.BoundsInRoot.Width
	if stableWidth != snappedWidth {
		return errors.Wrapf(err, "incorrect compat-snapped window width: got %v; want %v", snappedWidth, stableWidth)
	}

	if err := wm.CheckResizeLockState(ctx, tconn, a, d, cr, act, wm.PhoneResizeLockMode, false /* isSplashVisible */); err != nil {
		return errors.Wrap(err, "failed to verify the resize lock state after snapping")
	}

	return nil
}

func testUnsnapByDragging(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, cr *chrome.Chrome, pc pointer.Context, displayInfo *display.Info, d *ui.Device, act *arc.Activity, stableWidth int) error {
	if err := wm.DragCaptionToUnsnap(ctx, tconn, pc, displayInfo, act); err != nil {
		return errors.Wrap(err, "failed to drag to unsnap")
	}

	if err := wm.WaitForArcAndAshWindowState(ctx, tconn, d, act, arc.WindowStateNormal); err != nil {
		return errors.Wrap(err, "failed to wait until window state changes to normal")
	}

	window, err := ash.GetARCAppWindowInfo(ctx, tconn, act.PackageName())
	if err != nil {
		return errors.Wrap(err, "failed to get window info")
	}
	snappedWidth := window.BoundsInRoot.Width
	if stableWidth != snappedWidth {
		return errors.Wrapf(err, "incorrect compat-snapped window width: got %v; want %v", snappedWidth, stableWidth)
	}

	if err := wm.CheckResizeLockState(ctx, tconn, a, d, cr, act, wm.PhoneResizeLockMode, false /* isSplashVisible */); err != nil {
		return errors.Wrap(err, "failed to verify the resize lock state of compat-snapped window")
	}
	return nil
}

func testSnapFromOverview(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, cr *chrome.Chrome, pc pointer.Context, displayInfo *display.Info, d *ui.Device, act *arc.Activity, primary bool, stableWidth int) error {
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		return errors.Wrap(err, "failed to enter overview")
	}

	if err := wm.DragToSnapFirstOverviewWindow(ctx, tconn, pc, primary); err != nil {
		return errors.Wrap(err, "failed to drag to snap from overview")
	}

	if err := checkCompatSnappedWindowState(ctx, tconn, a, cr, d, act, primary, stableWidth); err != nil {
		return errors.Wrap(err, "failed to wait until window state change")
	}

	return nil
}

func testSnapByDragToSnap(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, cr *chrome.Chrome, pc pointer.Context, displayInfo *display.Info, d *ui.Device, act *arc.Activity, primary bool, stableWidth int) error {
	if err := wm.DragCaptionToSnap(ctx, tconn, pc, displayInfo, act, primary); err != nil {
		return errors.Wrap(err, "failed to drag to snap")
	}

	if err := checkCompatSnappedWindowState(ctx, tconn, a, cr, d, act, primary, stableWidth); err != nil {
		return errors.Wrap(err, "failed to wait until window state change")
	}

	return nil
}

func testSnapViaKeyboardShortcut(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, cr *chrome.Chrome, d *ui.Device, act *arc.Activity, primary bool, stableWidth int) error {
	if err := wm.ToggleSnapViaKeyboardShortcut(ctx, tconn, act, primary); err != nil {
		return errors.Wrap(err, "failed to snap window via keyboard shortcut")
	}

	if err := checkCompatSnappedWindowState(ctx, tconn, a, cr, d, act, primary, stableWidth); err != nil {
		return errors.Wrap(err, "failed to wait until window state change")
	}

	return nil
}

func CompatSnap(ctx context.Context, s *testing.State) {
	// Reserve ten seconds for various cleanup.
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
		s.Fatal("Creating test API connection Failed: ", err)
	}

	pc := pointer.NewMouse(tconn)
	defer pc.Close()

	displayInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}

	// Ensures landscape orientation.
	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the orientation info: ", err)
	}
	if orientation.Type == display.OrientationPortraitPrimary {
		if err = display.SetDisplayRotationSync(ctx, tconn, displayInfo.ID, display.Rotate90); err != nil {
			s.Fatal("Failed to rotate display: ", err)
		}
		defer display.SetDisplayRotationSync(cleanupCtx, tconn, displayInfo.ID, display.Rotate0)
	}

	// Set a pure white wallpaper to reduce the noises on a screenshot because currently wm.CheckResizeLockState checks the visibility of the translucent window border based on a screenshot.
	// The wallpaper will exist continuous if the Chrome session gets reused.
	ui := uiauto.New(tconn)
	if err := wm.SetSolidWhiteWallpaper(ctx, ui); err != nil {
		s.Fatal("Failed to set the white wallpaper: ", err)
	}

	// Install the test app.
	if err := a.Install(ctx, arc.APKPath(wm.ResizeLockApkName), adb.InstallOptionFromPlayStore); err != nil {
		s.Fatal("Failed to install app from PlayStore: ", err)
	}
	defer a.Uninstall(cleanupCtx, wm.ResizeLockTestPkgName)

	// Launch the test app.
	act, err := arc.NewActivity(a, wm.ResizeLockTestPkgName, wm.ResizeLockMainActivityName)
	if err != nil {
		s.Fatal("Failed to create a new activity: ", err)
	}
	defer act.Close()
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start the activity: ", err)
	}
	defer act.Stop(cleanupCtx, tconn)
	if err := ash.WaitForVisible(ctx, tconn, act.PackageName()); err != nil {
		s.Fatal("Failed to wait until the activity gets visible: ", err)
	}
	if err := d.WaitForIdle(ctx, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for Android to be idle: ", err)
	}

	// Close the compat mode splash dialog.
	if err := wm.CheckVisibility(ctx, tconn, wm.BubbleDialogClassName, true); err != nil {
		s.Fatal("Failed to wait for splash: ", err)
	}
	if err := wm.CloseSplash(ctx, tconn, wm.InputMethodClick, nil); err != nil {
		s.Fatal("Failed to close splash: ", err)
	}

	// Measure the window width of the stable state.
	window, err := ash.GetARCAppWindowInfo(ctx, tconn, act.PackageName())
	if err != nil {
		s.Fatal("Failed to get window info: ", err)
	}
	stableWidth := window.BoundsInRoot.Width

	defer ash.SetOverviewModeAndWait(cleanupCtx, tconn, false)
	for _, primary := range []bool{false, true} {
		// Case A. Snap the resize-locked window from overview mode.
		if err := testSnapFromOverview(ctx, tconn, a, cr, pc, displayInfo, d, act, primary, stableWidth); err != nil {
			s.Fatalf("Failed to snap window from overview (primary=%t): %v", primary, err)
		}
		if err := testUnsnapByDragging(ctx, tconn, a, cr, pc, displayInfo, d, act, stableWidth); err != nil {
			s.Fatalf("Failed to unsnap window (primary=%t): %v", primary, err)
		}

		// Case B. Snap the resize-locked window by dragging the caption bar to the edge of the screen.
		if err := testSnapByDragToSnap(ctx, tconn, a, cr, pc, displayInfo, d, act, primary, stableWidth); err != nil {
			s.Fatalf("Failed to snap window by drag-to-snap (primary=%t): %v", primary, err)
		}
		if err := testUnsnapByDragging(ctx, tconn, a, cr, pc, displayInfo, d, act, stableWidth); err != nil {
			s.Fatalf("Failed to unsnap window (primary=%t): %v", primary, err)
		}

		// Case C. Snap the resize-locked window via keyboard shortcut.
		if err := testSnapViaKeyboardShortcut(ctx, tconn, a, cr, d, act, primary, stableWidth); err != nil {
			s.Fatalf("Failed to snap window via keyboard shortcut (primary=%t): %v", primary, err)
		}
		if err := testUnsnapByDragging(ctx, tconn, a, cr, pc, displayInfo, d, act, stableWidth); err != nil {
			s.Fatalf("Failed to unsnap window (primary=%t): %v", primary, err)
		}
	}
}

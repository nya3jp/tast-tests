// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arc/wm"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	crui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	pkgMaximized  = "org.chromium.arc.testapp.windowmanager24.inmaximizedlist"
	pkgPhoneSize  = "org.chromium.arc.testapp.windowmanager24.inphonesizelist"
	pkgTabletSize = "org.chromium.arc.testapp.windowmanager24.intabletsizelist"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WMResizableClamshell,
		Desc:         "Verifies that Window Manager resizable clamshell use-cases behave as described in go/arc-wm-r",
		Contacts:     []string{"xutan@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_vm", "chrome"},
		Pre:          arc.Booted(),
		Timeout:      8 * time.Minute,
	})
}

func WMResizableClamshell(ctx context.Context, s *testing.State) {
	wm.SetupAndRunTestCases(ctx, s, false, []wm.TestCase{
		wm.TestCase{
			// resizable/clamshell: default launch behavior
			Name: "RC01_launch",
			Func: wmRC01,
		},
		wm.TestCase{
			// resizable/clamshell: resizable/clamshell: maximize portrait app (pillarbox)
			Name: "RC02_maximize_portrait",
			Func: wmRC02,
		},
	})
}

// wmRC01 covers resizable/clamshell default launch behavior.
// Expected behavior is defined in: go/arc-wm-r RC01: resizable/clamshell: default launch behavior.
func wmRC01(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	// launchBoundsThreshold stores the launch bounds of the last activity launch. This test is
	// written in the order from small launch bounds to big launch bounds so this variable
	// serves as the lower bound of launch bounds.
	var launchBoundsThreshold coords.Rect

	// Start with ordinary case where we are launching apps not in the whitelist.
	for activityName, desiredOrientation := range map[string]string{
		wm.ResizablePortraitActivity:  wm.Portrait,
		wm.ResizableLandscapeActivity: wm.Landscape,
	} {
		if err := func() error {
			act, err := arc.NewActivity(a, wm.Pkg24, activityName)
			if err != nil {
				return err
			}
			defer act.Close()

			if err := act.Start(ctx, tconn); err != nil {
				return err
			}
			defer act.Stop(ctx, tconn)

			if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
				return err
			}

			if err := wm.CheckRestoreResizable(ctx, tconn, act, d); err != nil {
				return err
			}

			orientation, err := wm.UIOrientation(ctx, act, d)
			if err != nil {
				return err
			}
			if orientation != desiredOrientation {
				return errors.Errorf("invalid orientation: got %v; want %v", orientation, desiredOrientation)
			}

			window, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
			if err != nil {
				return err
			}
			orientationFromBounds := wm.OrientationFromBounds(window.BoundsInRoot)
			if orientationFromBounds != desiredOrientation {
				return errors.Errorf("invalid bounds orientation: got %v; want %v",
					orientationFromBounds, desiredOrientation)
			}

			if desiredOrientation == wm.Portrait {
				launchBoundsThreshold = window.BoundsInRoot
			}

			return nil
		}(); err != nil {
			return errors.Wrapf(err, "%q test failed", activityName)
		}
	}

	// Then we verify the launch logic for whitelisted apps is correct.
	apkPath := map[string]string{
		pkgMaximized:  "ArcWMTestApp_24_InMaximizedList.apk",
		pkgPhoneSize:  "ArcWMTestApp_24_InPhoneSizeList.apk",
		pkgTabletSize: "ArcWMTestApp_24_InTabletSizeList.apk",
	}
	verifyFuncMap := map[string]func(*arc.Activity, *ash.Window) error{
		pkgPhoneSize: func(act *arc.Activity, window *ash.Window) error {
			if err := wm.CheckRestoreResizable(ctx, tconn, act, d); err != nil {
				return err
			}

			orientation, err := wm.UIOrientation(ctx, act, d)
			if err != nil {
				return err
			}
			if orientation != wm.Portrait {
				return errors.Errorf("invalid orientation: got %v; want portrait", orientation)
			}
			orientationFromBounds := wm.OrientationFromBounds(window.BoundsInRoot)
			if orientationFromBounds != wm.Portrait {
				return errors.Errorf("invalid bounds orientation: got %v; want portrait", orientationFromBounds)
			}

			if launchBoundsThreshold.Width > window.BoundsInRoot.Width {
				return errors.Errorf("phone size width shouldn't be smaller than %v, but it's %v",
					launchBoundsThreshold.Width, window.BoundsInRoot.Width)
			}
			if launchBoundsThreshold.Height > window.BoundsInRoot.Height {
				return errors.Errorf("phone size height shouldn't be smaller than %v, but it's %v",
					launchBoundsThreshold.Height, window.BoundsInRoot.Height)
			}
			return nil
		},
		pkgTabletSize: func(act *arc.Activity, window *ash.Window) error {
			if window.State == ash.WindowStateMaximized {
				// We might be running on a small device that can't hold a freeform window of tablet size.
				// However we don't have concrete display value to verify it, so we won't check display size.
				return wm.CheckMaximizeResizable(ctx, tconn, act, d)
			}

			// The majority of our devices is large enough to hold a freeform window of tablet size so it should
			// come here.
			if err := wm.CheckRestoreResizable(ctx, tconn, act, d); err != nil {
				return err
			}

			orientation, err := wm.UIOrientation(ctx, act, d)
			if err != nil {
				return err
			}
			if orientation != wm.Landscape {
				return errors.Errorf("invalid orientation: got %v; want landscape", orientation)
			}
			orientationFromBounds := wm.OrientationFromBounds(window.BoundsInRoot)
			if orientationFromBounds != wm.Landscape {
				return errors.Errorf("invalid bounds orientation: got %v; want landscape", orientationFromBounds)
			}

			if launchBoundsThreshold.Width > window.BoundsInRoot.Width {
				return errors.Errorf("tablet size width shouldn't be smaller than %v, but it's %v",
					launchBoundsThreshold.Width, window.BoundsInRoot.Width)
			}
			if launchBoundsThreshold.Height > window.BoundsInRoot.Height {
				return errors.Errorf("tablet size height shouldn't be smaller than %v, but it's %v",
					launchBoundsThreshold.Height, window.BoundsInRoot.Height)
			}
			return nil
		},
		pkgMaximized: func(act *arc.Activity, window *ash.Window) error {
			return wm.CheckMaximizeResizable(ctx, tconn, act, d)
		},
	}

	for _, pkgName := range []string{pkgPhoneSize, pkgTabletSize, pkgMaximized} {
		verifyFunc := verifyFuncMap[pkgName]
		if err := func() error {
			if err := a.Install(ctx, arc.APKPath(apkPath[pkgName])); err != nil {
				return err
			}
			defer a.Uninstall(ctx, pkgName)

			act, err := arc.NewActivity(a, pkgName, wm.ResizableUnspecifiedActivity)
			if err != nil {
				return err
			}
			defer act.Close()

			if err := act.Start(ctx, tconn); err != nil {
				return err
			}
			defer act.Stop(ctx, tconn)

			if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
				return err
			}

			if err := ash.WaitForVisible(ctx, tconn, pkgName); err != nil {
				return err
			}
			window, err := ash.GetARCAppWindowInfo(ctx, tconn, pkgName)
			if err != nil {
				return err
			}

			if err := verifyFunc(act, window); err != nil {
				return err
			}

			launchBoundsThreshold = window.BoundsInRoot
			return nil
		}(); err != nil {
			return err
		}
	}
	return nil
}

// wmRC02 covers resizable/clamshell: maximize portrait app (pillarbox).
// Expected behavior is defined in: go/arc-wm-r RC02: resizable/clamshell: maximize portrait app (pillarbox).
func wmRC02(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	for _, eTC := range []struct {
		Name string
		Func func(context.Context, *chrome.TestConn, string) error
	}{
		{"touchCaptionButton", touchCaptionButton},
		{"leftClickCaptionButton", leftClickCaptionButton},
	} {
		if err := func() error {
			if err := runRC02ByEventTypeFunc(ctx, tconn, a, d, eTC.Func); err != nil {
				return err
			}
			return nil
		}(); err != nil {
			return errors.Wrapf(err, "%q event type test case failed", eTC.Name)
		}
	}

	return nil
}

// touchCaptionButton function will simulate touch event on a caption button by button's name.
func touchCaptionButton(ctx context.Context, tconn *chrome.TestConn, btnName string) error {
	captionBtn, err := crui.Find(ctx, tconn, crui.FindParams{ClassName: "FrameCaptionButton", Name: btnName})
	if err != nil {
		return errors.Errorf("failed to find \"%q\" caption button", btnName)
	}
	defer captionBtn.Release(ctx)

	tsw, err := input.Touchscreen(ctx)
	if err != nil {
		return errors.New("failed to get TouchscreenEventWriter")
	}
	defer tsw.Close()

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		return errors.New("failed to get SingleTouchEventWriter")
	}
	defer stw.Close()

	// Get display info for touch coords calculation.
	primaryDisplayInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.New("failed to get display info")
	}
	if primaryDisplayInfo == nil {
		return errors.New("no primary display info found")
	}

	cBCX, cBCY := tsw.NewTouchCoordConverter(primaryDisplayInfo.Bounds.Size()).ConvertLocation(captionBtn.Location.CenterPoint())

	// Touch caption button center.
	if err := stw.Move(cBCX, cBCY); err != nil {
		return errors.Errorf("failed to move touch event writer on \"%q\" button", btnName)
	}
	if err := stw.End(); err != nil {
		return errors.Errorf("failed to end touch event writer on \"%q\" button", btnName)
	}

	return nil
}

// leftClickCaptionButton function will simulate left click event on a caption button by button's name.
func leftClickCaptionButton(ctx context.Context, tconn *chrome.TestConn, btnName string) error {
	captionBtn, err := crui.Find(ctx, tconn, crui.FindParams{ClassName: "FrameCaptionButton", Name: btnName})
	if err != nil {
		return errors.Errorf("failed to find \"%q\" caption button", btnName)
	}
	defer captionBtn.Release(ctx)

	if err := captionBtn.LeftClick(ctx); err != nil {
		return errors.Errorf("failed to perform left click on \"%q\" button", btnName)
	}

	return nil
}

// runRC02ByEventTypeFunc performs RC02 test either by left clicking or touching the caption button.
func runRC02ByEventTypeFunc(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, etFunc func(context.Context, *chrome.TestConn, string) error) error {
	act, err := arc.NewActivity(a, wm.Pkg24, wm.ResizablePortraitActivity)
	if err != nil {
		return err
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		return err
	}
	defer act.Stop(ctx, tconn)

	if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
		return err
	}

	if err := wm.CheckRestoreResizable(ctx, tconn, act, d); err != nil {
		return err
	}

	// Store windows info before maximizing the activity to compare it with after restoring it.
	winInfoBeforeMax, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}

	// Store windowID to wait for animating finishes.
	windowID := winInfoBeforeMax.ID

	// Touch/Click maximize button on caption bar.
	if err := etFunc(ctx, tconn, "Maximize"); err != nil {
		return err
	}

	if err := ash.WaitForARCAppWindowState(ctx, tconn, wm.Pkg24, ash.WindowStateMaximized); err != nil {
		return err
	}
	if err := ash.WaitWindowFinishAnimating(ctx, tconn, windowID); err != nil {
		return err
	}

	if err := wm.CheckMaximizeResizable(ctx, tconn, act, d); err != nil {
		return err
	}

	// Touch/Click restore button on caption bar.
	if err := etFunc(ctx, tconn, "Restore"); err != nil {
		return err
	}

	if err := ash.WaitForARCAppWindowState(ctx, tconn, wm.Pkg24, ash.WindowStateNormal); err != nil {
		return err
	}
	if err := ash.WaitWindowFinishAnimating(ctx, tconn, windowID); err != nil {
		return err
	}

	if err := wm.CheckRestoreResizable(ctx, tconn, act, d); err != nil {
		return err
	}

	// Get window info after restoring, this should be equal to winInfoBeforeMax.
	winInfoAfterMax, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}

	// Compare BoundsInRoot of the activity before and after switching to maximize and restore button on caption bar.
	if winInfoBeforeMax.BoundsInRoot != winInfoAfterMax.BoundsInRoot {
		return errors.Errorf("failed to validate window bounds after restoring from maximize state, got: %q, want: %q", winInfoAfterMax.BoundsInRoot, winInfoBeforeMax.BoundsInRoot)
	}

	return nil
}

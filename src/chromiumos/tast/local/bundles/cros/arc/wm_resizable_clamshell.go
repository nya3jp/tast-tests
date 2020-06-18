// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arc/wm"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/screenshot"
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

	// testFunc represents a function that tests if the window is in a certain state.
	type testFunc func(context.Context, *chrome.TestConn, *arc.ARC, *ui.Device) error

	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC

	if err := a.Install(ctx, arc.APKPath(wm.APKNameArcWMTestApp24)); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed to initialize UI Automator: ", err)
	}
	defer d.Close()

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure if tablet mode is disabled: ", err)
	}
	defer cleanup(ctx)

	for _, test := range []struct {
		name string
		fn   testFunc
	}{
		{"RC01_launch", wmRC01}, // resizable/clamshell: default launch behavior
	} {
		s.Logf("Running test %q", test.name)

		if err := test.fn(ctx, tconn, a, d); err != nil {
			path := fmt.Sprintf("%s/screenshot-cuj-failed-test-%s.png", s.OutDir(), test.name)
			if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
				s.Log("Failed to capture screenshot: ", err)
			}
			s.Errorf("%s test failed: %v", test.name, err)
		}
	}
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

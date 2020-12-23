// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/wm"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

const (
	pkgMaximized  = "org.chromium.arc.testapp.windowmanager24.inmaximizedlist"
	pkgPhoneSize  = "org.chromium.arc.testapp.windowmanager24.inphonesizelist"
	pkgTabletSize = "org.chromium.arc.testapp.windowmanager24.intabletsizelist"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowDefaultBoundsAllowlist,
		Desc:         "Verifies that allowlists for overriding launch window bounds work",
		Contacts:     []string{"xutan@google.com", "takise@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Timeout:      8 * time.Minute,
		Params: []testing.Param{{
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}, {
			Name:              "pi",
			ExtraSoftwareDeps: []string{"android_p"},
		}},
	})
}

// WindowDefaultBoundsAllowlist checks whether top N apps have an "allowlisted default size".
// This test is part of the specification of WM R.
func WindowDefaultBoundsAllowlist(ctx context.Context, s *testing.State) {
	wm.SetupAndRunTestCases(ctx, s, false, []wm.TestCase{
		wm.TestCase{
			Name: "wmAllowlistResizableUnspecified",
			Func: wmAllowlistResizableUnspecified,
		},
	})
}

// wmAllowlistResizableUnspecified covers resizable/unspecified allowlisted launch behavior.
func wmAllowlistResizableUnspecified(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Turn the device into clamshell.
	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get tablet mode")
	}

	if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
		return errors.Wrap(err, "failed to set tablet mode to false")
	}
	defer ash.SetTabletModeEnabled(cleanupCtx, tconn, tabletModeEnabled)

	// launchBoundsThreshold stores the launch bounds of the last activity launch. This test is
	// written in the order from small launch bounds to big launch bounds so this variable
	// serves as the lower bound of launch bounds.
	launchBoundsThreshold, err := func() (coords.Rect, error) {
		// Launch a resizable portrait app first to use the bounds as the lower bound of phone size.
		act, err := arc.NewActivity(a, wm.Pkg24, wm.ResizablePortraitActivity)
		if err != nil {
			return coords.Rect{}, errors.Wrap(err, "failed to create the non-allowlisted activity")
		}
		defer act.Close()

		if err := act.Start(ctx, tconn); err != nil {
			return coords.Rect{}, errors.Wrap(err, "failed to start the non-allowlisted activity")
		}
		defer act.Stop(ctx, tconn)

		if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
			return coords.Rect{}, errors.Wrap(err, "failed to wait for the non-allowlisted activity to be ready")
		}

		// The default window state in ARC P is maximized, so ensure that the app is restored first to calculate the default freeform bounds.
		windowState, err := act.GetWindowState(ctx)
		if err != nil {
			return coords.Rect{}, errors.Wrap(err, "failed to get the default window state of the non-allowlisted activity")
		}
		if windowState != arc.WindowStateNormal {
			winInfoBeforeRestore, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
			if err != nil {
				return coords.Rect{}, err
			}
			if err := act.SetWindowState(ctx, tconn, arc.WindowStateNormal); err != nil {
				return coords.Rect{}, errors.Wrap(err, "failed to restore the window")
			}
			if err := ash.WaitForARCAppWindowState(ctx, tconn, wm.Pkg24, ash.WindowStateMaximized); err != nil {
				return coords.Rect{}, errors.Wrap(err, "failed to wait for the non-allowlisted activity to be restored")
			}
			if err := ash.WaitWindowFinishAnimating(ctx, tconn, winInfoBeforeRestore.ID); err != nil {
				return coords.Rect{}, errors.Wrap(err, "failed to wait for the animation of the non-allowlisted activity to be finished")
			}
		}

		window, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
		if err != nil {
			return coords.Rect{}, errors.Wrap(err, "failed to get the window info of the non-allowlisted activity")
		}
		return window.BoundsInRoot, nil
	}()
	if err != nil {
		return err
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

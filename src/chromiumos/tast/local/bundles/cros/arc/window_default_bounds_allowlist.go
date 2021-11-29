// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowDefaultBoundsAllowlist,
		LacrosStatus: testing.LacrosVariantUnneeded,
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
		{
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

	// Then we verify the launch logic for allow listed apps is correct.
	apkPath := map[string]string{
		wm.Pkg24InMaximizedList:  wm.APKNameArcWMTestApp24Maximized,
		wm.Pkg24InPhoneSizeList:  wm.APKNameArcWMTestApp24PhoneSize,
		wm.Pkg24InTabletSizeList: wm.APKNameArcWMTestApp24TabletSize,
	}
	verifyFuncMap := map[string]func(*arc.Activity, *ash.Window) error{
		wm.Pkg24InPhoneSizeList: func(act *arc.Activity, window *ash.Window) error {
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
			return nil
		},
		wm.Pkg24InTabletSizeList: func(act *arc.Activity, window *ash.Window) error {
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
			return nil
		},
		wm.Pkg24InMaximizedList: func(act *arc.Activity, window *ash.Window) error {
			return wm.CheckMaximizeResizable(ctx, tconn, act, d)
		},
	}

	for _, pkgName := range []string{wm.Pkg24InPhoneSizeList, wm.Pkg24InTabletSizeList, wm.Pkg24InMaximizedList} {
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

			if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
				return err
			}
			defer act.Stop(ctx, tconn)

			if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
				return err
			}

			if err := ash.WaitForVisible(ctx, tconn, pkgName); err != nil {
				return err
			}

			if err := testing.Poll(ctx, func(ctx context.Context) error {
				window, err := ash.GetARCAppWindowInfo(ctx, tconn, pkgName)
				if err != nil {
					return testing.PollBreak(err)
				}
				if err := verifyFunc(act, window); err != nil {
					return err
				}
				return nil
			}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
				return err
			}
			return nil
		}(); err != nil {
			return err
		}
	}
	return nil
}

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
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WMNonresizableTablet,
		Desc:         "Verifies that Window Manager non-resizable tablet use-cases behave as described in go/arc-wm-r",
		Contacts:     []string{"armenk@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_vm", "chrome"},
		Pre:          arc.VMBooted(),
		Timeout:      8 * time.Minute,
	})
}

func WMNonresizableTablet(ctx context.Context, s *testing.State) {
	wm.SetupAndRunTestCases(ctx, s, []wm.TestCase{
		wm.TestCase{
			// non-resizable/tablet: default launch behavior
			Name: "NT_default_launch_behavior",
			Func: wmNT01,
		},
	})
}

// wmNT01 covers non-resizable/tablet: default launch behavior.
// Expected behavior is defined in: go/arc-wm-r NT01: non-resizable/tablet: default launch behavior.
func wmNT01(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	for _, tc := range []struct {
		activityName         string
		displayRotationAngle display.RotationAngle
	}{
		{
			activityName:         wm.NonResizableUnspecifiedActivity,
			displayRotationAngle: display.Rotate0,
		},
		{
			activityName:         wm.NonResizablePortraitActivity,
			displayRotationAngle: display.Rotate270,
		},
	} {
		if err := func() error {
			if tc.displayRotationAngle != display.Rotate0 {
				resetDisplay, err := rotateDisplay(ctx, tconn, tc.displayRotationAngle)
				if err != nil {
					return err
				}
				defer resetDisplay()
			}

			act, err := arc.NewActivity(a, wm.Pkg24, tc.activityName)
			if err != nil {
				return err
			}
			defer act.Close()

			if err := act.Start(ctx, tconn); err != nil {
				return err
			}
			defer act.Stop(ctx)

			if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
				return err
			}

			windowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
			if err != nil {
				return err
			}

			return wm.CheckMaximizeWindowInTabletMode(ctx, tconn, windowInfo.TargetBounds)
		}(); err != nil {
			return errors.Wrapf(err, "%q test failed", tc.activityName)
		}
	}
	return nil
}

func rotateDisplay(ctx context.Context, tconn *chrome.TestConn, angle display.RotationAngle) (func() error, error) {
	primaryDisplayInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return nil, err
	}

	if err := display.SetDisplayRotationSync(ctx, tconn, primaryDisplayInfo.ID, angle); err != nil {
		return nil, err
	}

	return func() error {
		return resetDisplayOrientation(ctx, tconn, primaryDisplayInfo.ID)
	}, nil
}

func resetDisplayOrientation(ctx context.Context, tconn *chrome.TestConn, displayID string) error {
	if err := display.SetDisplayRotationSync(ctx, tconn, displayID, display.Rotate0); err != nil {
		return err
	}
	return nil
}

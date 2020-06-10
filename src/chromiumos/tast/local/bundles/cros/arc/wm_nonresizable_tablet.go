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
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/screenshot"
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

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure if tablet mode is enabled: ", err)
	}
	defer cleanup(ctx)

	for _, test := range []struct {
		name string
		fn   testFunc
	}{
		{"NT_default_launch_behavior", wmNT01}, // non-resizable/tablet: default launch behavior
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

// wmNT01 covers non-resizable/tablet: default launch behavior.
// Expected behavior is defined in: go/arc-wm-r NT01: non-resizable/tablet: default launch behavior.
func wmNT01(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	for _, tc := range []struct {
		activityName         string
		displayRotationAngle display.RotationAngle
	}{
		{
			activityName:         wm.ResizableUnspecifiedActivity,
			displayRotationAngle: display.Rotate0,
		},
		{
			activityName:         wm.ResizablePortraitActivity,
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

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
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

// testFunc represents a function that tests if the window is in a certain state.
type testFunc func(context.Context, *chrome.TestConn, *arc.ARC, *ui.Device) error

func init() {
	testing.AddTest(&testing.Test{
		Func:         WMNonresizableClamshell,
		Desc:         "Verifies that Window Manager NC use cases behaves as described in go/arc-wm-r",
		Contacts:     []string{"armenk@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_vm", "chrome"},
		Data:         []string{"ArcWMTestApp_24.apk"},
		Pre:          arc.VMBooted(),
		Timeout:      8 * time.Minute,
	})
}

func WMNonresizableClamshell(ctx context.Context, s *testing.State) {
	wmApkToInstall := []string{"ArcWMTestApp_24.apk"}
	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC

	tconn, d, err := wm.CommonWMSetUp(ctx, s, wmApkToInstall)
	if err != nil {
		s.Fatal("Failed to setup test: ", err)
	}
	defer d.Close()

	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}

	if tabletModeEnabled {
		// Restore tablet mode to its original state on exit.
		defer ash.SetTabletModeEnabled(ctx, tconn, true)
		if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
			s.Fatal("Failed to set tablet mode disabled: ", err)
		}

		// Wait for "tablet mode animation is finished"
		// If an activity is lauched while the table mode animation is active, the activity
		// will be launched in an undefined state, making the test flaky.
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			s.Fatal("Failed to wait until tablet-mode animation finished: ", err)
		}
	}

	for _, test := range []struct {
		name string
		fn   testFunc
	}{
		{"NC01_launch", wmNC01}, // non-resizable/clamshell: default launch behavior
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

func wmNC01(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	for _, activity := range []struct {
		name string
	}{
		{wm.NonResizeablePortraitActivity},
		{wm.NonResizeableLandscapeActivity},
	} {
		if err := func() error {
			act, err := arc.NewActivity(a, wm.Pkg24, activity.name)
			if err != nil {
				return err
			}

			if err := act.Start(ctx, tconn); err != nil {
				return err
			}
			defer act.Stop(ctx)

			if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
				return err
			}
			defer act.Close()

			return wm.CheckMaximizeNonResizeable(ctx, tconn, act, d)
		}(); err != nil {
			return errors.Wrapf(err, "%q test failed", activity.name)
		}
	}
	return nil
}

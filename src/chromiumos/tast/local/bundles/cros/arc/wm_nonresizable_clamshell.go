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

func init() {
	testing.AddTest(&testing.Test{
		Func:         WMNonresizableClamshell,
		Desc:         "Verifies that Window Manager non-resizable clamshell use-cases behave as described in go/arc-wm-r",
		Contacts:     []string{"armenk@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_vm", "chrome"},
		Data:         []string{wm.APKNameArcWMTestApp24},
		Pre:          arc.Booted(),
		Timeout:      8 * time.Minute,
	})
}

func WMNonresizableClamshell(ctx context.Context, s *testing.State) {

	// testFunc represents a function that tests if the window is in a certain state.
	type testFunc func(context.Context, *chrome.TestConn, *arc.ARC, *ui.Device) error

	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC

	if err := a.Install(ctx, s.DataPath(wm.APKNameArcWMTestApp24)); err != nil {
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

// wmNC01 covers non-resizable/clamshell default launch behavior.
// Expected behavior is defined in: go/arc-wm-r NC01: non-resizable/clamshell: default launch behavior.
func wmNC01(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	for _, activityName := range []string{
		wm.NonResizablePortraitActivity,
		wm.NonResizableLandscapeActivity,
	} {
		if err := func() error {
			act, err := arc.NewActivity(a, wm.Pkg24, activityName)
			if err != nil {
				return err
			}

			if err := act.Start(ctx, tconn); err != nil {
				return err
			}
			defer act.Stop(ctx, tconn)

			if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
				return err
			}
			defer act.Close()

			return wm.CheckMaximizeNonResizable(ctx, tconn, act, d)
		}(); err != nil {
			return errors.Wrapf(err, "%q test failed", activityName)
		}
	}
	return nil
}

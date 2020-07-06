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
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WMNonresizableClamshell,
		Desc:         "Verifies that Window Manager non-resizable clamshell use-cases behave as described in go/arc-wm-r",
		Contacts:     []string{"armenk@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_vm", "chrome"},
		Pre:          arc.Booted(),
		Timeout:      8 * time.Minute,
	})
}

func WMNonresizableClamshell(ctx context.Context, s *testing.State) {
	wm.SetupAndRunTestCases(ctx, s, false, []wm.TestCase{
		wm.TestCase{
			// non-resizable/clamshell: default launch behavior
			Name: "NC_default_launch_behavior",
			Func: wmNC01,
		},
		wm.TestCase{
			// non-resizable/clamshell: user immerse portrait app (pillarbox)
			Name: "NC_user_immerse_portrait",
			Func: wmNC04,
		},
		wm.TestCase{
			// non-resizable/clamshell: user immerse non-portrait app
			Name: "NC_user_immerse_non_portrait",
			Func: wmNC05,
		},
	})
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
			defer act.Close()

			if err := act.Start(ctx, tconn); err != nil {
				return err
			}
			defer act.Stop(ctx, tconn)

			if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
				return err
			}

			return wm.CheckMaximizeNonResizable(ctx, tconn, act, d)
		}(); err != nil {
			return errors.Wrapf(err, "%q test failed", activityName)
		}
	}
	return nil
}

// wmNC04 covers non-resizable/clamshell: user immerse portrait app (pillarbox) behavior.
// Expected behavior is defined in: go/arc-wm-r NC04: non-resizable/clamshell: user immerse portrait app (pillarbox).
func wmNC04(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	return checkMaxActivityToFullscreen(ctx, tconn, a, d, wm.NonResizablePortraitActivity)
}

// wmNC05 covers non-resizable/clamshell: user immerse non-portrait app behavior.
// Expected behavior is defined in: go/arc-wm-r NC05: non-resizable/clamshell: user immerse non-portrait app.
func wmNC05(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	for _, activityName := range []string{
		wm.NonResizableLandscapeActivity,
		wm.NonResizableUnspecifiedActivity,
	} {
		if err := checkMaxActivityToFullscreen(ctx, tconn, a, d, activityName); err != nil {
			return err
		}
	}
	return nil
}

// checkMaxActivityToFullscreen creates a new activity, lunches it and toggles to fullscreen and checks for validity of window info.
func checkMaxActivityToFullscreen(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, activityName string) error {
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

	if err := wm.CheckMaximizeNonResizable(ctx, tconn, act, d); err != nil {
		return err
	}

	windowInfoMaximized, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}
	if err := wm.ToggleFullscreen(ctx, tconn); err != nil {
		return err
	}

	if err := ash.WaitForARCAppWindowState(ctx, tconn, wm.Pkg24, ash.WindowStateFullscreen); err != nil {
		return err
	}

	if err := ash.WaitWindowFinishAnimating(ctx, tconn, windowInfoMaximized.ID); err != nil {
		return err
	}

	windowInfoFullscreen, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}
	return wm.CheckMaximizeToFullscreenToggle(ctx, tconn, windowInfoMaximized.TargetBounds, *windowInfoFullscreen)
}
